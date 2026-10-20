package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"office-palette-changer/internal/ooxml"
	"office-palette-changer/internal/theme"
	"office-palette-changer/internal/thmx"
	"office-palette-changer/themes"
)

// slotLabels are the human names for the 12 OOXML palette slots.
var slotLabels = map[string]string{
	"dk1": "text", "lt1": "background", "dk2": "text alt", "lt2": "background alt",
	"accent1": "accent 1", "accent2": "accent 2", "accent3": "accent 3",
	"accent4": "accent 4", "accent5": "accent 5", "accent6": "accent 6",
	"hlink": "hyperlink", "folHlink": "visited link",
}

func cmdList(args []string) int {
	list := ListThemes()
	if len(list) == 0 {
		fmt.Println("no themes found — create one with: ostyle palette <name> <colors...>")
		return 0
	}
	dir := UserThemeDir()
	if _, err := os.Stat(dir); err != nil {
		fmt.Printf("Themes (user dir %s not created yet — `ostyle palette` or `ostyle new` creates it):\n", dir)
	} else {
		fmt.Printf("Themes (user dir: %s):\n", dir)
	}
	for _, ti := range list {
		src, err := themes.FS.ReadFile(ti.Name + ".css")
		if err != nil {
			src = []byte{}
		}
		if ti.Path != "" {
			if b, err := os.ReadFile(ti.Path); err == nil {
				src = b
			}
		}
		t, _, err := theme.FromCSS(string(src), ti.Name)
		if err != nil {
			fmt.Printf("  %-12s [%s]  (parse error: %v)\n", ti.Name, ti.Source, err)
			continue
		}
		pal := t.Base.Filled()
		row := ""
		for _, k := range []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6"} {
			if colorEnabled() {
				row += swatchInline(pal.Colors[k])
			} else {
				row += fmt.Sprintf("#%s ", pal.Colors[k])
			}
		}
		fmt.Printf("  %-12s [%s]  %s\n", ti.Name, ti.Source, strings.TrimSpace(row))
	}
	return 0
}

func swatchInline(hex string) string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm  \x1b[0m", hexVal(hex[0:2]), hexVal(hex[2:4]), hexVal(hex[4:6]))
}

func cmdShow(args []string) int {
	if len(args) != 1 {
		return fail("usage: ostyle show <theme>")
	}
	t, source, warns, err := LoadTheme(args[0])
	if err != nil {
		return fail("%v", err)
	}
	fmt.Printf("Theme: %s  (%s)\n\n", t.Name, source)
	word := t.ForWord()
	ppt := t.ForPPT()

	fmt.Println("Palette (Word):")
	for _, k := range theme.ColorKeys {
		fmt.Println("  " + swatch(word.Colors[k], slotLabels[k]))
	}
	if fmt.Sprint(palMap(ppt)) != fmt.Sprint(palMap(word)) {
		fmt.Println("Palette (PowerPoint overrides):")
		for _, k := range theme.ColorKeys {
			if ppt.Colors[k] != word.Colors[k] {
				fmt.Println("  " + swatch(ppt.Colors[k], slotLabels[k]))
			}
		}
	}
	fmt.Println()
	fmt.Printf("Fonts:   headings %q · body %q\n", word.MajorFont, word.MinorFont)

	if len(t.WordHeadings) > 0 {
		fmt.Println("\nWord headings:")
		for lvl := 1; lvl <= 6; lvl++ {
			hs := t.WordHeadings[lvl]
			if hs == nil {
				continue
			}
			parts := []string{fmt.Sprintf("h%d", lvl)}
			if hs.SizePt != nil {
				parts = append(parts, fmt.Sprintf("%gpt", *hs.SizePt))
			}
			if hs.Bold != nil && *hs.Bold {
				parts = append(parts, "bold")
			}
			if hs.Italic != nil && *hs.Italic {
				parts = append(parts, "italic")
			}
			if hs.Color != "" {
				parts = append(parts, "#"+hs.Color)
			}
			if hs.SpaceBeforePt != nil {
				parts = append(parts, fmt.Sprintf("before %gpt", *hs.SpaceBeforePt))
			}
			if hs.SpaceAfterPt != nil {
				parts = append(parts, fmt.Sprintf("after %gpt", *hs.SpaceAfterPt))
			}
			fmt.Println("  " + strings.Join(parts, " · "))
		}
	}
	if t.PPTBackground != "" {
		fmt.Printf("\nPowerPoint background: %s\n", t.PPTBackground)
	}
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	return 0
}

func palMap(p theme.Palette) map[string]string { return p.Colors }

const scaffoldCSS = `/* ostyle theme: %s */
:root {
  --text: #2B2B2B;        /* dk1: main text color            */
  --background: #FFFFFF;  /* lt1: canvas / light fills       */
  --dk2: #444444;
  --lt2: #F2F2F2;
  --accent1: #2F6FED;
  --accent2: #26B47F;
  --accent3: #F2A33C;
  --accent4: #E85D75;
  --accent5: #8C6FE8;
  --accent6: #38B6D8;
  --hlink: #2F6FED;
  --folhlink: #8C6FE8;
  --heading-font: "Aptos Display";
  --body-font: "Aptos";
}
@word {
  h1 { size: 20pt; bold: true; color: var(--accent1); space-before: 18pt; }
  h2 { size: 15pt; bold: true; color: var(--accent2); space-before: 14pt; }
}
/* @ppt { background: var(--lt1); }  enables a themed deck background */
`

func cmdNew(args []string) int {
	fs := newFlagSet("new", "ostyle new <name> [--from <file.docx|file.pptx>] [--copy <theme>] [--here] [--force]")
	here := fs.Bool("here", false, "write the theme into the current directory")
	from := fs.String("from", "", "extract the palette from an existing .docx/.pptx")
	copyFrom := fs.String("copy", "", "start from a copy of an existing theme (built-in or yours)")
	force := fs.Bool("force", false, "overwrite an existing theme file")
	if err := parseMixed(fs, args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	name := fs.Arg(0)
	if err := validThemeName(name); err != nil {
		return fail("%v", err)
	}
	if *from != "" && *copyFrom != "" {
		return fail("use either --from or --copy, not both")
	}

	var content string
	switch {
	case *from != "":
		t, err := ooxml.ExtractTheme(*from)
		if err != nil {
			return fail("%v", err)
		}
		t.Name = name
		t.Source = "ostyle new " + name + " --from " + quoteArg(*from)
		content = t.ToCSS()
	case *copyFrom != "":
		src, origin, err := themeSource(*copyFrom)
		if err != nil {
			return fail("%v", err)
		}
		content = retitleTheme(src, name, origin)
	default:
		content = fmt.Sprintf(scaffoldCSS, name)
	}

	path := themeTargetPath(name, *here)
	if err := writeThemeFile(path, content, *force); err != nil {
		return fail("%v", err)
	}
	fmt.Printf("created %s\n", path)
	fmt.Println("edit it, preview with `ostyle show`, apply with `ostyle apply`.")
	return 0
}

// themeSource returns the raw CSS of a theme reference (path, user, built-in).
func themeSource(ref string) (css string, origin string, err error) {
	if strings.ContainsRune(ref, '/') || strings.HasSuffix(ref, ".css") {
		if b, err := os.ReadFile(ref); err == nil {
			return string(b), ref, nil
		}
	}
	userPath := filepath.Join(UserThemeDir(), ref+".css")
	if b, err := os.ReadFile(userPath); err == nil {
		return string(b), userPath, nil
	}
	if b, err := themes.FS.ReadFile(ref + ".css"); err == nil {
		return string(b), "built-in " + ref, nil
	}
	return "", "", fmt.Errorf("theme %q not found (looked in %s and the built-ins)", ref, UserThemeDir())
}

// retitleTheme swaps the header comment of copied CSS for one naming the new
// theme and where it came from.
func retitleTheme(src, name, origin string) string {
	lines := strings.SplitN(src, "\n", 2)
	body := src
	if strings.HasPrefix(strings.TrimSpace(lines[0]), "/* ostyle theme:") && len(lines) == 2 {
		body = lines[1]
	}
	return fmt.Sprintf("/* ostyle theme: %s (copied from %s) */\n%s", name, origin, body)
}

func cmdExport(args []string) int {
	fs := newFlagSet("export", "ostyle export <theme> [out.thmx]")
	if err := parseMixed(fs, args); err != nil {
		return 2
	}
	if fs.NArg() < 1 || fs.NArg() > 2 {
		fs.Usage()
		return 2
	}
	t, _, warns, err := LoadTheme(fs.Arg(0))
	if err != nil {
		return fail("%v", err)
	}
	out := t.Name + ".thmx"
	if fs.NArg() == 2 {
		out = fs.Arg(1)
	}
	data, err := thmx.Build(t)
	if err != nil {
		return fail("%v", err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return fail("writing %s: %v", out, err)
	}
	fmt.Printf("wrote %s — apply it from Word/PowerPoint: Design → Themes → Browse for Themes\n", out)
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	return 0
}

func cmdValidate(args []string) int {
	if len(args) != 1 {
		return fail("usage: ostyle validate <theme.css>")
	}
	t, source, warns, err := LoadTheme(args[0])
	if err != nil {
		return fail("%v", err)
	}
	fmt.Printf("%s: OK (%s)\n", args[0], source)
	for _, f := range []struct{ name, font string }{
		{"heading font", t.Base.MajorFont}, {"body font", t.Base.MinorFont},
	} {
		if f.font == "" {
			continue
		}
		if !fontInstalled(f.font) {
			fmt.Printf("note: %s %q was not found in your font folders — text will fall back\n", f.name, f.font)
		}
	}
	for _, w := range warns {
		fmt.Printf("warning: %s\n", w)
	}
	return 0
}

// fontInstalled does a lenient check of the common macOS font directories.
// PostScript file names drop spaces, so both spellings are tried.
func fontInstalled(name string) bool {
	if name == "" {
		return true
	}
	home, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(home, "Library", "Fonts"),
		"/Library/Fonts",
		"/System/Library/Fonts",
		"/System/Library/Fonts/Supplemental",
	}
	compact := strings.ToLower(strings.ReplaceAll(name, " ", ""))
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			fn := strings.ToLower(e.Name())
			fn = strings.TrimSuffix(strings.TrimSuffix(fn, ".ttc"), ".ttf")
			fn = strings.TrimSuffix(fn, ".otf")
			if fn == compact || strings.Contains(fn, compact) {
				return true
			}
		}
	}
	return false
}
