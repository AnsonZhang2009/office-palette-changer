package ooxml

import (
	"fmt"
	"strings"

	"office-palette-changer/internal/theme"
)

// PatchThemeXML rewrites the clrScheme of a theme part (word/theme/*.xml or
// ppt/theme/*.xml), swaps the major/minor latin typefaces when the palette
// sets fonts, and renames the theme. The fmtScheme is intentionally left
// untouched: standard Office format schemes reference phClr/schemeClr, so
// they restyle automatically from the new palette.
func PatchThemeXML(xml string, t *theme.Theme, pal theme.Palette) (string, error) {
	prefix := detectThemePrefix(xml)

	clr := clrSchemeXML(prefix, t.Name, pal.Filled().Colors)
	doc, _, ok := replaceElem(xml, prefix+":clrScheme", clr, 0)
	if !ok {
		return "", fmt.Errorf("theme part has no %s:clrScheme element", prefix)
	}

	if fs, e, found := findElem(doc, prefix+":fontScheme", 0); found {
		content := doc[fs:e]
		if pal.MajorFont != "" {
			content = patchLatin(content, prefix, "majorFont", pal.MajorFont)
		}
		if pal.MinorFont != "" {
			content = patchLatin(content, prefix, "minorFont", pal.MinorFont)
		}
		doc = doc[:fs] + content + doc[e:]
	}

	doc = setRootElementAttr(doc, prefix+":theme", "name", "ostyle: "+t.Name)
	return doc, nil
}

// detectThemePrefix reads the namespace prefix of the root element,
// skipping any <?xml ...?> declaration.
func detectThemePrefix(xml string) string {
	search := xml
	if strings.HasPrefix(search, "<?") {
		if i := strings.Index(search, "?>"); i >= 0 {
			search = search[i+2:]
		}
	}
	i := strings.Index(search, "<")
	if i < 0 {
		return "a"
	}
	rest := search[i+1:]
	j := strings.Index(rest, ":theme")
	if j < 0 {
		return "a"
	}
	prefix := rest[:j]
	if prefix == "" || strings.ContainsAny(prefix, "<> \t\r\n?\"'") {
		return "a"
	}
	return prefix
}

func clrSchemeXML(prefix, name string, colors map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<%s:clrScheme name="%s">`, prefix, EscAttr(name))
	for _, k := range theme.ColorKeys {
		fmt.Fprintf(&b, `<%s:%s><%s:srgbClr val="%s"/></%s:%s>`, prefix, k, prefix, colors[k], prefix, k)
	}
	fmt.Fprintf(&b, `</%s:clrScheme>`, prefix)
	return b.String()
}

func patchLatin(fontScheme, prefix, which, typeface string) string {
	s, e, ok := findElem(fontScheme, prefix+":"+which, 0)
	if !ok {
		return fontScheme
	}
	part := fontScheme[s:e]
	lat := fmt.Sprintf(`<%s:latin typeface="%s"/>`, prefix, EscAttr(typeface))
	newPart, _, replaced := replaceElem(part, prefix+":latin", lat, 0)
	if !replaced {
		gt, _, ok2 := findTagEnd(part, 0)
		if !ok2 {
			return fontScheme
		}
		newPart = part[:gt+1] + lat + part[gt+1:]
	}
	return fontScheme[:s] + newPart + fontScheme[e:]
}

func setRootElementAttr(doc, elem, attr, val string) string {
	i := strings.Index(doc, "<"+elem)
	if i < 0 {
		return doc
	}
	gt, _, ok := findTagEnd(doc, i)
	if !ok {
		return doc
	}
	return doc[:i] + setAttrInTag(doc[i:gt+1], attr, val) + doc[gt+1:]
}
