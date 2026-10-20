package ooxml

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"office-palette-changer/internal/theme"
)

type Result struct {
	Kind  string   // "word" or "powerpoint"
	Parts []string // entry names that were modified
}

// ApplyFile patches the .docx/.pptx at path with the theme, writing a .bak
// backup first when backup is true. The rewrite goes to a temp file in the
// same directory and is renamed into place, so the original is never left
// half-written.
func ApplyFile(path string, t *theme.Theme, backup bool) (*Result, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	res, patched, err := applyBytes(raw, t)
	if err != nil {
		return nil, err
	}
	if len(patched) == 0 {
		return nil, fmt.Errorf("%s: no theme or styles parts found (unsupported file?)", path)
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ostyle-tmp-*")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(patched); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Chmod(tmpName, info.Mode().Perm()); err != nil {
		return nil, err
	}
	if backup {
		if err := os.WriteFile(path+".bak", raw, info.Mode().Perm()); err != nil {
			return nil, fmt.Errorf("writing backup: %w", err)
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return nil, err
	}
	tmpName = "" // renamed; nothing to clean up
	return res, nil
}

type zipEntry struct {
	name string
	data []byte
}

func readZip(raw []byte) ([]zipEntry, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("not an OOXML package (legacy .doc/.ppt or corrupted?): %w", err)
	}
	var entries []zipEntry
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f.Name, err)
		}
		entries = append(entries, zipEntry{name: f.Name, data: data})
	}
	return entries, nil
}

func applyBytes(raw []byte, t *theme.Theme) (*Result, []byte, error) {
	entries, err := readZip(raw)
	if err != nil {
		return nil, nil, err
	}
	kind := detectKind(entries)
	if kind == "" {
		return nil, nil, errors.New("not a .docx/.pptx package (no word/ or ppt/ parts)")
	}

	res := &Result{Kind: kind}
	for i := range entries {
		en := &entries[i]
		switch {
		case kind == "word" && strings.HasPrefix(en.name, "word/theme/") && strings.HasSuffix(en.name, ".xml"):
			patched, err := PatchThemeXML(string(en.data), t, t.ForWord())
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", en.name, err)
			}
			en.data = []byte(patched)
			res.Parts = append(res.Parts, en.name)
		case kind == "word" && en.name == "word/styles.xml":
			patched, err := PatchWordStylesXML(string(en.data), t)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", en.name, err)
			}
			en.data = []byte(patched)
			res.Parts = append(res.Parts, en.name)
		case kind == "powerpoint" && strings.HasPrefix(en.name, "ppt/theme/") && strings.HasSuffix(en.name, ".xml"):
			patched, err := PatchThemeXML(string(en.data), t, t.ForPPT())
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", en.name, err)
			}
			en.data = []byte(patched)
			res.Parts = append(res.Parts, en.name)
		case kind == "powerpoint" && t.PPTBackground != "" &&
			strings.HasPrefix(en.name, "ppt/slideMasters/") && strings.HasSuffix(en.name, ".xml"):
			patched, ok := PatchMasterBGXML(string(en.data), t.PPTBackground)
			if ok {
				en.data = []byte(patched)
				res.Parts = append(res.Parts, en.name)
			}
		}
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, en := range entries {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: en.name, Method: zip.Deflate})
		if err != nil {
			return nil, nil, err
		}
		if _, err := w.Write(en.data); err != nil {
			return nil, nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, nil, err
	}
	return res, buf.Bytes(), nil
}

func detectKind(entries []zipEntry) string {
	kind := ""
	for _, en := range entries {
		switch {
		case strings.HasPrefix(en.name, "ppt/"):
			if kind == "word" {
				return ""
			}
			kind = "powerpoint"
		case strings.HasPrefix(en.name, "word/"):
			if kind == "powerpoint" {
				return ""
			}
			kind = "word"
		}
	}
	return kind
}

// ExtractTheme reads the current palette/fonts out of a .docx/.pptx.
func ExtractTheme(path string) (*theme.Theme, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	entries, err := readZip(raw)
	if err != nil {
		return nil, err
	}
	for _, en := range entries {
		if (strings.HasPrefix(en.name, "word/theme/") || strings.HasPrefix(en.name, "ppt/theme/")) &&
			strings.HasSuffix(en.name, ".xml") {
			return extractFromThemePart(string(en.data))
		}
	}
	return nil, errors.New("no theme part found in package")
}

// colorValue reads a color slot's value: srgbClr/@val, or sysClr/@lastClr
// for the dk1/lt1 window colors.
func colorValue(colorElem, prefix string) string {
	if s, e, ok := findElem(colorElem, prefix+":srgbClr", 0); ok {
		if v := attrValue(colorElem[s:e], "val"); v != "" {
			return v
		}
	}
	if s, e, ok := findElem(colorElem, prefix+":sysClr", 0); ok {
		if v := attrValue(colorElem[s:e], "lastClr"); v != "" {
			return v
		}
	}
	return attrValue(colorElem, "val")
}

func extractFromThemePart(xml string) (*theme.Theme, error) {
	prefix := detectThemePrefix(xml)
	t := &theme.Theme{Base: theme.Palette{Colors: map[string]string{}}}

	rootStart := strings.Index(xml, "<"+prefix+":theme")
	if rootStart >= 0 {
		if gt, _, ok := findTagEnd(xml, rootStart); ok {
			name := attrValue(xml[rootStart:gt+1], "name")
			name = strings.TrimSpace(strings.TrimPrefix(name, "ostyle: "))
			if name != "" {
				t.Name = name
			}
		}
	}

	if s, e, ok := findElem(xml, prefix+":clrScheme", 0); ok {
		clr := xml[s:e]
		for _, key := range theme.ColorKeys {
			if cs, ce, ok2 := findElem(clr, prefix+":"+key, 0); ok2 {
				v := colorValue(clr[cs:ce], prefix)
				if v != "" {
					if norm, err := theme.NormalizeHex(v); err == nil {
						t.Base.Colors[key] = norm
					}
				}
			}
		}
	}

	if s, e, ok := findElem(xml, prefix+":fontScheme", 0); ok {
		fs := xml[s:e]
		for which, dst := range map[string]*string{"majorFont": &t.Base.MajorFont, "minorFont": &t.Base.MinorFont} {
			if ms, me, ok2 := findElem(fs, prefix+":"+which, 0); ok2 {
				inner := fs[ms:me]
				if ls, le, ok3 := findElem(inner, prefix+":latin", 0); ok3 {
					tf := attrValue(inner[ls:le], "typeface")
					if tf != "" {
						*dst = tf
					}
				}
			}
		}
	}
	return t, nil
}
