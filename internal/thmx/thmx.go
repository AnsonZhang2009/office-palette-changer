// Package thmx builds standalone .thmx theme files that can be applied from
// Office's own theme gallery (and shared with Windows users).
package thmx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"

	"office-palette-changer/internal/ooxml"
	"office-palette-changer/internal/theme"
)

// scriptFonts is the standard Office fallback list for non-latin scripts in a
// generated fontScheme.
var scriptFonts = []struct{ Script, Major, Minor string }{
	{"Jpan", "游ゴシック Light", "游ゴシック"},
	{"Hang", "맑은 고딕", "맑은 고딕"},
	{"Hans", "等线 Light", "等线"},
	{"Hant", "新細明體", "新細明體"},
	{"Arab", "", ""},
	{"Hebr", "", ""},
	{"Thai", "", ""},
	{"Ethi", "", ""},
	{"Benga", "", ""},
	{"Gujr", "", ""},
	{"Khmr", "", ""},
	{"Laos", "", ""},
	{"Mlym", "", ""},
	{"Taml", "", ""},
	{"Telu", "", ""},
	{"Knda", "", ""},
	{"Sinh", "", ""},
}

const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/></Types>`

const rels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/themeManager" Target="theme/theme1.xml"/></Relationships>`

// Build renders the theme as a complete .thmx package.
func Build(t *theme.Theme) ([]byte, error) {
	pal := t.Base.Filled()
	major := pal.MajorFont
	if major == "" {
		major = "Aptos Display"
	}
	minor := pal.MinorFont
	if minor == "" {
		minor = "Aptos"
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	fmt.Fprintf(&b, `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="%s">`, ooxml.EscAttr("ostyle: "+t.Name))
	b.WriteString(`<a:themeElements>`)
	b.WriteString(clrScheme(t.Name, pal.Colors))
	b.WriteString(fontScheme(major, minor))
	b.WriteString(fmtScheme())
	b.WriteString(`</a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/>`)
	b.WriteString(`</a:theme>`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := []struct{ name, body string }{
		{"[Content_Types].xml", contentTypes},
		{"_rels/.rels", rels},
		{"theme/theme1.xml", b.String()},
	}
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(f.body)); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func clrScheme(name string, colors map[string]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<a:clrScheme name="%s">`, ooxml.EscAttr(name))
	for _, k := range theme.ColorKeys {
		fmt.Fprintf(&b, `<a:%s><a:srgbClr val="%s"/></a:%s>`, k, colors[k], k)
	}
	b.WriteString(`</a:clrScheme>`)
	return b.String()
}

func fontScheme(major, minor string) string {
	var b strings.Builder
	b.WriteString(`<a:fontScheme name="ostyle">`)
	for _, pair := range []struct{ which, font string }{{"majorFont", major}, {"minorFont", minor}} {
		fmt.Fprintf(&b, `<a:%s><a:latin typeface="%s"/><a:ea typeface=""/><a:cs typeface=""/>`, pair.which, ooxml.EscAttr(pair.font))
		which := "major"
		if pair.which == "minorFont" {
			which = "minor"
		}
		for _, sf := range scriptFonts {
			face := sf.Major
			if which == "minor" {
				face = sf.Minor
			}
			fmt.Fprintf(&b, `<a:font script="%s" typeface="%s"/>`, sf.Script, ooxml.EscAttr(face))
		}
		fmt.Fprintf(&b, `</a:%s>`, pair.which)
	}
	b.WriteString(`</a:fontScheme>`)
	return b.String()
}

// fmtScheme is the standard palette-driven Office format scheme: every style
// references phClr / schemeClr, so it adapts to whatever palette it ships with.
func fmtScheme() string {
	return `<a:fmtScheme name="ostyle"><a:fillStyleLst>` +
		`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
		`<a:gradFill rotWithShape="1"><a:gsLst>` +
		`<a:gs pos="0"><a:schemeClr val="phClr"><a:lumMod val="110000"/><a:satMod val="105000"/><a:tint val="103000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="50000"><a:schemeClr val="phClr"><a:lumMod val="105000"/><a:satMod val="103000"/><a:tint val="109000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="100000"><a:schemeClr val="phClr"><a:lumMod val="105000"/><a:satMod val="109000"/><a:tint val="103000"/></a:schemeClr></a:gs>` +
		`</a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>` +
		`<a:gradFill rotWithShape="1"><a:gsLst>` +
		`<a:gs pos="0"><a:schemeClr val="phClr"><a:shade val="103000"/><a:satMod val="102000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="50000"><a:schemeClr val="phClr"><a:shade val="109000"/><a:satMod val="103000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="108000"/><a:satMod val="108000"/><a:tint val="103000"/></a:schemeClr></a:gs>` +
		`</a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>` +
		`</a:fillStyleLst><a:lnStyleLst>` +
		`<a:ln w="6350" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln>` +
		`<a:ln w="12700" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln>` +
		`<a:ln w="19050" cap="flat" cmpd="sng" algn="ctr"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:prstDash val="solid"/><a:miter lim="800000"/></a:ln>` +
		`</a:lnStyleLst><a:effectStyleLst>` +
		`<a:effectStyle><a:effectLst/></a:effectStyle>` +
		`<a:effectStyle><a:effectLst/></a:effectStyle>` +
		`<a:effectStyle><a:effectLst><a:outerShdw blurRad="57150" dist="19050" dir="5400000" algn="ctr" rotWithShape="0"><a:srgbClr val="000000"><a:alpha val="63000"/></a:srgbClr></a:outerShdw></a:effectLst></a:effectStyle>` +
		`</a:effectStyleLst><a:bgFillStyleLst>` +
		`<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>` +
		`<a:solidFill><a:schemeClr val="phClr"><a:lumMod val="95000"/><a:satMod val="105000"/><a:tint val="103000"/></a:schemeClr></a:solidFill>` +
		`<a:gradFill rotWithShape="1"><a:gsLst>` +
		`<a:gs pos="0"><a:schemeClr val="phClr"><a:lumMod val="93000"/><a:satMod val="105000"/><a:tint val="97000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="50000"><a:schemeClr val="phClr"><a:shade val="98000"/><a:satMod val="103000"/><a:tint val="101000"/></a:schemeClr></a:gs>` +
		`<a:gs pos="100000"><a:schemeClr val="phClr"><a:shade val="103000"/><a:satMod val="108000"/><a:tint val="101000"/></a:schemeClr></a:gs>` +
		`</a:gsLst><a:lin ang="5400000" scaled="0"/></a:gradFill>` +
		`</a:bgFillStyleLst></a:fmtScheme>`
}
