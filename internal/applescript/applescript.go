// Package applescript drives Microsoft Word / PowerPoint just enough to
// style the document that is currently open: find it, save it, close it, and
// reopen it after ostyle has patched the file on disk.
//
// All scripts take arguments through `on run argv` rather than string
// interpolation, so file paths can never inject AppleScript code.
package applescript

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type App int

const (
	Word App = iota
	PowerPoint
)

func ParseApp(s string) (App, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "word", "microsoft word", "msword":
		return Word, nil
	case "powerpoint", "ppt", "microsoft powerpoint", "msppt":
		return PowerPoint, nil
	}
	return Word, fmt.Errorf("unknown app %q (use word or powerpoint)", s)
}

func (a App) Name() string {
	if a == PowerPoint {
		return "Microsoft PowerPoint"
	}
	return "Microsoft Word"
}

func (a App) Short() string {
	if a == PowerPoint {
		return "powerpoint"
	}
	return "word"
}

// IsRunning reports whether the app process is up (so we can avoid osascript
// launching Word just to ask a question).
func (a App) IsRunning() bool {
	err := exec.Command("pgrep", "-x", a.Name()).Run()
	return err == nil
}

func osa(script string, args ...string) (string, error) {
	cmd := exec.Command("osascript", append([]string{"-"}, args...)...)
	cmd.Stdin = strings.NewReader(script)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, oneLine(errBuf.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func oneLine(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "\n", " · ")
}

// PrivacyHint returns a user-facing explanation when err looks like a macOS
// privacy restriction. The two flavors are: Automation consent (-1743, the
// "wants to control Microsoft Word" dialog) and the per-folder "授予访问权限 /
// Grant Access" prompt that sandboxed Office apps trigger the first time they
// touch a folder. Neither can be granted programmatically; the user must
// click the dialog (or use System Settings) exactly once.
func PrivacyHint(err error) string {
	if err == nil {
		return ""
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "-1743") || strings.Contains(s, "not authorized") || strings.Contains(s, "automat"):
		return "macOS is blocking automation. Approve the “… wants to control Microsoft Word/PowerPoint” dialog once, " +
			"or System Settings → Privacy & Security → Automation → your terminal."
	case strings.Contains(s, "-5000") || strings.Contains(s, "permission") || strings.Contains(s, "权限") ||
		strings.Contains(s, "access") || strings.Contains(s, "访问"):
		return "Office needs one-time folder access. Click 授予访问权限 (Grant Access) if a dialog appeared, or " +
			"System Settings → Privacy & Security → Files and Folders → Word/PowerPoint. " +
			"This is only required the first time the folder is used."
	}
	return ""
}

// ActiveDocPath returns the POSIX path of the active document/presentation.
// It fails when nothing (or nothing saved) is open.
func ActiveDocPath(a App) (string, error) {
	docNoun := "active document"
	if a == PowerPoint {
		docNoun = "active presentation"
	}
	script := fmt.Sprintf(`
on run argv
	tell application %q
		set p to full name of %s as text
	end tell
	if p is missing value then error "active document has never been saved"
	if p starts with "/" then return p
	try
		return POSIX path of p
	on error
		return p
	end try
end run`, a.Name(), docNoun)
	out, err := osa(script)
	if err != nil {
		return "", fmt.Errorf("could not get the active document from %s (is one open and saved?): %w", a.Name(), err)
	}
	if out == "" || out == "missing value" {
		return "", fmt.Errorf("%s reports no saved active document", a.Name())
	}
	return out, nil
}

// OpenDocPaths lists the POSIX paths of all open documents, or nil.
// Word reports HFS-style paths, so each entry is converted.
func OpenDocPaths(a App) (paths []string, err error) {
	docs := "documents"
	if a == PowerPoint {
		docs = "presentations"
	}
	script := fmt.Sprintf(`
on run argv
	tell application %q
		set thePaths to full name of %s
	end tell
	set posixList to {}
	repeat with p in thePaths
		try
			if (p as text) starts with "/" then
				set end of posixList to (p as text)
			else
				set end of posixList to (POSIX path of p)
			end if
		end try
	end repeat
	set {oldDelim, AppleScript's text item delimiters} to {AppleScript's text item delimiters, linefeed}
	try
		set theText to posixList as text
		set AppleScript's text item delimiters to oldDelim
		return theText
	on error errMsg
		set AppleScript's text item delimiters to oldDelim
		error errMsg
	end try
end run`, a.Name(), docs)
	out, err := osa(script)
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" && line != "missing value" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// SaveActive saves the active document (no-op dialog when already clean).
func SaveActive(a App) error {
	doc := "active document"
	if a == PowerPoint {
		doc = "active presentation"
	}
	script := fmt.Sprintf(`
on run argv
	tell application %q to save %s
end run`, a.Name(), doc)
	_, err := osa(script)
	if err != nil {
		return fmt.Errorf("saving in %s failed: %w", a.Name(), err)
	}
	return nil
}

// CloseActive closes the active document without further prompting.
func CloseActive(a App) error {
	doc := "active document"
	suffix := " saving no"
	if a == PowerPoint {
		doc = "active presentation"
		suffix = ""
	}
	script := fmt.Sprintf(`
on run argv
	tell application %q to close %s%s
end run`, a.Name(), doc, suffix)
	_, err := osa(script)
	if err != nil {
		return fmt.Errorf("closing in %s failed: %w", a.Name(), err)
	}
	return nil
}

// OpenDoc opens a document and brings the app to front.
func OpenDoc(a App, path string) error {
	script := fmt.Sprintf(`
on run argv
	set p to item 1 of argv
	tell application %q
		open p
		activate
	end tell
end run`, a.Name())
	if _, err := osa(script, path); err != nil {
		return fmt.Errorf("reopening in %s failed: %w", a.Name(), err)
	}
	return nil
}
