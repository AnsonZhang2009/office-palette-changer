package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"office-palette-changer/internal/applescript"
	"office-palette-changer/internal/ooxml"
	"office-palette-changer/internal/theme"
)

func cmdApply(args []string) int {
	fs := newFlagSet("apply", "ostyle apply <theme> <file...> | ostyle apply <theme> --active --app word|powerpoint")
	app := fs.String("app", "", "word or powerpoint (required with --active)")
	active := fs.Bool("active", false, "style the document currently open in the app")
	noBackup := fs.Bool("no-backup", false, "do not write a .bak file before patching")
	noReopen := fs.Bool("no-reopen", false, "with --active: leave the document closed afterwards")
	force := fs.Bool("force", false, "patch even if the file appears to be open in Word/PowerPoint")
	if err := parseMixed(fs, args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fs.Usage()
		return 2
	}
	t, source, warns, err := LoadTheme(fs.Arg(0))
	if err != nil {
		return fail("%v", err)
	}
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	backup := !*noBackup

	if *active {
		if *app == "" {
			return fail("--active needs --app word or --app powerpoint")
		}
		a, err := applescript.ParseApp(*app)
		if err != nil {
			return fail("%v", err)
		}
		return applyActive(t, a, source, backup, !*noReopen)
	}

	files := fs.Args()[1:]
	if len(files) == 0 {
		fs.Usage()
		return 2
	}
	openPaths := collectOpenPaths()
	rc := 0
	for _, f := range files {
		if err := applyOne(f, t, backup, openPaths, *force); err != nil {
			fmt.Fprintf(os.Stderr, "ostyle: %v\n", err)
			rc = 1
		}
	}
	return rc
}

func applyActive(t *theme.Theme, a applescript.App, source string, backup, reopen bool) int {
	path, err := applescript.ActiveDocPath(a)
	if err != nil {
		return failWithHint(err)
	}
	if strings.HasPrefix(path, "/http") {
		return fail("the active %s document is a cloud file (%s) — save a local copy first, style it, then upload it back", a.Short(), path)
	}
	wantExt := ".docx"
	if a == applescript.PowerPoint {
		wantExt = ".pptx"
	}
	if !strings.HasSuffix(strings.ToLower(path), wantExt) {
		return fail("the active %s document is %q — expected a %s file (legacy formats must be re-saved first)", a.Short(), path, wantExt)
	}

	fmt.Printf("styling %s with theme %q (%s)\n", filepath.Base(path), t.Name, source)
	if err := applescript.SaveActive(a); err != nil {
		return failWithHint(err)
	}
	res, err := ooxml.ApplyFile(path, t, backup)
	if err != nil {
		return fail("%v", err)
	}
	if err := applescript.CloseActive(a); err != nil {
		fmt.Fprintf(os.Stderr, "ostyle: %v (patched on disk anyway — reopen manually)\n", err)
		printHint(applescript.PrivacyHint(err))
		return 0
	}
	if reopen {
		if err := applescript.OpenDoc(a, path); err != nil {
			fmt.Fprintf(os.Stderr, "ostyle: %v\nThe file is already styled on disk — open it manually.\n", err)
			printHint(applescript.PrivacyHint(err))
		}
	}
	reportApplied(path, res, backup)
	return 0
}

// failWithHint prints the error plus a macOS privacy explanation when the
// error looks like a permission/consent failure.
func failWithHint(err error) int {
	rc := fail("%v", err)
	printHint(applescript.PrivacyHint(err))
	return rc
}

func printHint(hint string) {
	if hint != "" {
		fmt.Fprintf(os.Stderr, "hint: %s\n", hint)
	}
}

func applyOne(path string, t *theme.Theme, backup bool, openPaths map[string]bool, force bool) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".docx", ".pptx":
	case ".doc", ".ppt":
		return fmt.Errorf("%s: legacy binary format — re-save it as .docx/.pptx first", path)
	default:
		return fmt.Errorf("%s: not a .docx or .pptx file", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if openPaths[normalizePath(abs)] && !force {
		return fmt.Errorf("%s is currently open in Word/PowerPoint — close it first, or use `ostyle apply %s --active --app ...`, or --force", path, t.Name)
	}
	res, err := ooxml.ApplyFile(path, t, backup)
	if err != nil {
		return err
	}
	reportApplied(path, res, backup)
	return nil
}

func reportApplied(path string, res *ooxml.Result, backup bool) {
	msg := fmt.Sprintf("✓ %s: theme applied (%s, %d part(s) patched", path, res.Kind, len(res.Parts))
	if backup {
		msg += ", backup at " + path + ".bak"
	}
	msg += ")"
	fmt.Println(msg)
}

// collectOpenPaths asks running apps which documents they have open, so we
// can refuse to patch a file underneath Office. Failure to ask (e.g. the
// Automation permission was never granted) is not fatal in headless mode.
func collectOpenPaths() map[string]bool {
	open := map[string]bool{}
	for _, a := range []applescript.App{applescript.Word, applescript.PowerPoint} {
		if !a.IsRunning() {
			continue
		}
		paths, err := applescript.OpenDocPaths(a)
		if err != nil {
			fmt.Fprintf(os.Stderr, "note: could not check open documents in %s (%v); proceeding anyway\n", a.Name(), err)
			continue
		}
		for _, p := range paths {
			open[normalizePath(p)] = true
		}
	}
	return open
}

// normalizePath resolves symlinks (/tmp vs /private/tmp) so paths reported by
// Office match paths computed from the command line.
func normalizePath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = p
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	return abs
}
