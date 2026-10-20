// Package cli implements the ostyle command line interface.
package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"office-palette-changer/internal/theme"
	"office-palette-changer/themes"
)

const version = "0.1.0"

const helpText = `ostyle — lightweight palette/font/heading stylizer for Word & PowerPoint (macOS)

Usage:
  ostyle list                          list available themes
  ostyle show <theme>                  inspect a theme's palette and fonts
  ostyle apply <theme> <file...>       patch closed .docx/.pptx files on disk
  ostyle apply <theme> --active --app word|powerpoint
                                       style the document that is open right now
                                       (saves it, patches the file, reopens it)
  ostyle new <name> [--from <file>]    scaffold a theme (optionally extracting
                                       the palette from an existing document)
  ostyle export <theme> [out.thmx]     export as a real .thmx for Office's
                                       theme gallery / Windows colleagues
  ostyle validate <theme.css>          lint a theme file

Themes live as .css files in ~/.config/ostyle/themes/ (or $OSTYLE_THEME_DIR),
plus the built-ins compiled into the binary. A theme ref is either a name
("nord"), a path to a .css file, or a path with a trailing / to search a dir.

Flags for apply:
  --app word|powerpoint   which app to drive (required with --active)
  --active                style the frontmost open document
  --no-backup             skip writing <file>.bak before patching
  --no-reopen             with --active: leave the document closed afterwards
  --force                 patch even if the file appears to be open

Homepage: https://github.com/ansonzhang/office-palette-changer
`

func Main(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, helpText)
		return 2
	}
	switch args[0] {
	case "list":
		return cmdList(args[1:])
	case "show":
		return cmdShow(args[1:])
	case "apply":
		return cmdApply(args[1:])
	case "new":
		return cmdNew(args[1:])
	case "export":
		return cmdExport(args[1:])
	case "validate":
		return cmdValidate(args[1:])
	case "version", "--version", "-v":
		fmt.Println("ostyle " + version)
		return 0
	case "help", "--help", "-h":
		fmt.Print(helpText)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "ostyle: unknown command %q\n\n", args[0])
		fmt.Fprint(os.Stderr, helpText)
		return 2
	}
}

func fail(format string, a ...any) int {
	fmt.Fprintf(os.Stderr, "ostyle: "+format+"\n", a...)
	return 1
}

// ---- theme resolution ------------------------------------------------------

// UserThemeDir is where user themes live: $OSTYLE_THEME_DIR, else
// $XDG_CONFIG_HOME/ostyle/themes, else ~/.config/ostyle/themes.
func UserThemeDir() string {
	if d := os.Getenv("OSTYLE_THEME_DIR"); d != "" {
		return d
	}
	base, err := os.UserHomeDir()
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		base = d
	} else if err != nil {
		return ".ostyle-themes"
	}
	return filepath.Join(base, ".config", "ostyle", "themes")
}

type ThemeInfo struct {
	Name   string
	Source string // "builtin" | "user"
	Path   string // empty for built-ins
}

func ListThemes() []ThemeInfo {
	var out []ThemeInfo
	if entries, err := themes.FS.ReadDir("."); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".css") {
				out = append(out, ThemeInfo{Name: strings.TrimSuffix(e.Name(), ".css"), Source: "builtin"})
			}
		}
	}
	if entries, err := os.ReadDir(UserThemeDir()); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".css") {
				out = append(out, ThemeInfo{
					Name:   strings.TrimSuffix(e.Name(), ".css"),
					Source: "user",
					Path:   filepath.Join(UserThemeDir(), e.Name()),
				})
			}
		}
	}
	return out
}

// LoadTheme resolves a theme reference: explicit .css path, user dir, then
// built-ins. Returns the theme, its source description, and warnings.
func LoadTheme(ref string) (*theme.Theme, string, []string, error) {
	readFile := func(path, source string) (*theme.Theme, []string, error) {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		name := strings.TrimSuffix(filepath.Base(path), ".css")
		t, warns, err := theme.FromCSS(string(src), name)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", path, err)
		}
		return t, warns, nil
	}

	if strings.ContainsRune(ref, '/') || strings.HasSuffix(ref, ".css") {
		if _, err := os.Stat(ref); err == nil {
			t, warns, err := readFile(ref, "path")
			if err != nil {
				return nil, "", nil, err
			}
			return t, ref, warns, nil
		}
		if strings.ContainsRune(ref, '/') {
			return nil, "", nil, fmt.Errorf("theme file not found: %s", ref)
		}
	}

	userPath := filepath.Join(UserThemeDir(), ref+".css")
	if _, err := os.Stat(userPath); err == nil {
		t, warns, err := readFile(userPath, "user")
		if err != nil {
			return nil, "", nil, err
		}
		return t, userPath + " (user)", warns, nil
	}

	if src, err := themes.FS.ReadFile(ref + ".css"); err == nil {
		t, warns, err := theme.FromCSS(string(src), ref)
		if err != nil {
			return nil, "", nil, fmt.Errorf("built-in theme %s: %w", ref, err)
		}
		return t, "built-in", warns, nil
	}

	return nil, "", nil, fmt.Errorf("theme %q not found (looked in %s and the built-ins)", ref, UserThemeDir())
}

// ---- output helpers --------------------------------------------------------

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	ct := os.Getenv("COLORTERM")
	if strings.Contains(ct, "truecolor") || strings.Contains(ct, "24bit") {
		return true
	}
	return strings.Contains(os.Getenv("TERM"), "color")
}

// swatch renders a colored block followed by the label, degrading to plain
// text when the terminal has no color support.
func swatch(hex, label string) string {
	r, g, b := hex[0:2], hex[2:4], hex[4:6]
	rv := hexVal(r)
	gv := hexVal(g)
	bv := hexVal(b)
	if !colorEnabled() {
		return fmt.Sprintf("%-28s #%s", label, hex)
	}
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm      \x1b[0m %-20s #%s", rv, gv, bv, label, hex)
}

func hexVal(h string) int {
	v := 0
	for i := 0; i < len(h); i++ {
		c := h[i]
		v *= 16
		switch {
		case c >= '0' && c <= '9':
			v += int(c - '0')
		case c >= 'a' && c <= 'f':
			v += int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			v += int(c-'A') + 10
		}
	}
	return v
}

func newFlagSet(name, usage string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s\n", usage)
		fs.PrintDefaults()
	}
	return fs
}

// parseMixed parses flags that may appear after positional arguments
// (Go's flag package stops at the first non-flag argument). Flags that take
// a value keep their value adjacent through the reorder.
func parseMixed(fs *flag.FlagSet, args []string) error {
	takesValue := map[string]bool{}
	fs.VisitAll(func(f *flag.Flag) {
		bf, ok := f.Value.(interface{ IsBoolFlag() bool })
		takesValue[f.Name] = !(ok && bf.IsBoolFlag())
	})
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" && a != "--" {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if !strings.Contains(name, "=") && takesValue[name] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		positional = append(positional, a)
	}
	return fs.Parse(append(flags, positional...))
}
