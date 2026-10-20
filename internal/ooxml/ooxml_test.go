package ooxml

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"office-palette-changer/internal/theme"
)

const fixtureThemeXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme"><a:themeElements><a:clrScheme name="Office"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2><a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2><a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4><a:accent5><a:srgbClr val="5B9BD5"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6><a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink></a:clrScheme><a:fontScheme name="Office"><a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont><a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme><a:fmtScheme name="Office"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst><a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/></a:theme>`

const fixtureStylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri" w:cs="Calibri"/><w:sz w:val="22"/><w:szCs w:val="22"/><w:lang w:val="en-US" w:eastAsia="en-US" w:bidi="ar-SA"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="259" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:qFormat/><w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/></w:rPr></w:style><w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/><w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:uiPriority w:val="9"/><w:qFormat/><w:pPr><w:keepNext/><w:keepLines/><w:spacing w:before="240"/><w:outlineLvl w:val="0"/></w:pPr><w:rPr><w:rFonts w:ascii="Calibri Light" w:hAnsi="Calibri Light"/><w:color w:val="2F5496" w:themeColor="accent1" w:themeShade="99"/><w:sz w:val="32"/><w:szCs w:val="32"/></w:rPr></w:style></w:styles>`

func testTheme() *theme.Theme {
	src := `
:root {
  --text: #2E3440; --background: #ECEFF4;
  --dk2: #4C566A; --lt2: #E5E9F0;
  --accent1: #81A1C1; --accent2: #8FBCBB; --accent3: #A3BE8C;
  --accent4: #EBCB8B; --accent5: #B48EAD; --accent6: #88C0D0;
  --hlink: #88C0D0; --folhlink: #8FBCBB;
  --heading-font: "Avenir Next"; --body-font: "Helvetica Neue";
}
@word {
  h1 { size: 20pt; bold: true; color: var(--accent1); space-before: 18pt; }
  h3 { size: 12pt; bold: false; }
}
@ppt { background: var(--lt1); }
`
	t, _, err := theme.FromCSS(src, "nord")
	if err != nil {
		panic(err)
	}
	return t
}

func TestPatchThemeXML(t *testing.T) {
	th := testTheme()
	out, err := PatchThemeXML(fixtureThemeXML, th, th.ForWord())
	if err != nil {
		t.Fatalf("PatchThemeXML: %v", err)
	}
	for _, want := range []string{
		`<a:dk1><a:srgbClr val="2E3440"/></a:dk1>`,
		`<a:accent1><a:srgbClr val="81A1C1"/></a:accent1>`,
		`<a:latin typeface="Avenir Next"/>`,
		`<a:latin typeface="Helvetica Neue"/>`,
		`name="ostyle: nord"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("patched theme missing %s", want)
		}
	}
	// fmtScheme must survive untouched.
	if !strings.Contains(out, `<a:fmtScheme name="Office"`) {
		t.Error("fmtScheme was disturbed")
	}
	// The old palette must be gone.
	if strings.Contains(out, `val="4472C4"`) {
		t.Error("old accent1 still present")
	}
}

func TestPatchWordStylesXML(t *testing.T) {
	th := testTheme()
	out, err := PatchWordStylesXML(fixtureStylesXML, th)
	if err != nil {
		t.Fatalf("PatchWordStylesXML: %v", err)
	}
	// docDefaults and Normal point at the theme body font.
	if got := strings.Count(out, `w:asciiTheme="minorHAnsi"`); got != 2 {
		t.Errorf("expected 2 minorHAnsi rFonts (docDefaults + Normal), got %d", got)
	}
	// Heading1 retargeted to major font with explicit overrides.
	h1 := findStyleByID(t, out, "Heading1")
	if !strings.Contains(h1, `w:asciiTheme="majorHAnsi"`) {
		t.Error("Heading1 not retargeted to major font")
	}
	if !strings.Contains(h1, `<w:color w:val="81A1C1"/>`) {
		t.Error("Heading1 explicit color missing")
	}
	if !strings.Contains(h1, `<w:sz w:val="40"/>`) || !strings.Contains(h1, `<w:szCs w:val="40"/>`) {
		t.Error("Heading1 size (20pt = 40 half-points) missing")
	}
	if !strings.Contains(h1, `<w:b/>`) {
		t.Error("Heading1 bold missing")
	}
	if !strings.Contains(h1, `<w:spacing w:before="360"`) {
		t.Error("Heading1 space-before 18pt (=360 twips) missing")
	}
	// themeColor attrs must be dropped where we set explicit colors.
	if strings.Contains(h1, `w:themeColor=`) {
		t.Error("Heading1 still carries w:themeColor after explicit color")
	}
	// Heading3 defined only in theme -> appended with bold off.
	if !strings.Contains(out, `w:styleId="Heading3"`) || !strings.Contains(out, `<w:b w:val="0"/>`) {
		t.Error("missing appended Heading3 with bold off")
	}
}

func findStyleByID(t *testing.T, doc, id string) string {
	t.Helper()
	from := 0
	for {
		s, e, ok := findElem(doc, "w:style", from)
		if !ok {
			t.Fatalf("style %s not found", id)
		}
		if attrValue(doc[s:e], "w:styleId") == id {
			return doc[s:e]
		}
		from = e
	}
}

func TestPatchMasterBGXML(t *testing.T) {
	master := `<p:sldMaster><p:cSld><p:spTree/></p:cSld></p:sldMaster>`
	out, ok := PatchMasterBGXML(master, "scheme:lt1")
	if !ok || !strings.HasPrefix(out[strings.Index(out, "<p:cSld")+len("<p:cSld>"):], "<p:bg>") {
		t.Errorf("bg not inserted as first child of cSld: %s", out)
	}
	withBG := `<p:sldMaster><p:cSld><p:bg><p:bgRef idx="1001"><a:schemeClr val="dk1"/></p:bgRef></p:bg><p:spTree/></p:cSld></p:sldMaster>`
	out2, ok2 := PatchMasterBGXML(withBG, "hex:ABCDEF")
	if !ok2 || !strings.Contains(out2, `<a:srgbClr val="ABCDEF"/>`) {
		t.Errorf("existing bg not replaced: %s", out2)
	}
}

func TestFindElemEdgeCases(t *testing.T) {
	doc := `<r><a:ln w="1"><a:x/></a:ln><a:lnStyleLst/><a:v a="x/>' >"/></a:v></r>`
	s, e, ok := findElem(doc, "a:ln", 0)
	if !ok || doc[s:e] != `<a:ln w="1"><a:x/></a:ln>` {
		t.Errorf("findElem a:ln = %q", doc[s:e])
	}
	// a:lnStyleLst must not match a search for a:ln, and vice versa.
	s2, _, ok2 := findElem(doc, "a:lnStyleLst", 0)
	if !ok2 || s2 != s+len(`<a:ln w="1"><a:x/></a:ln>`) {
		t.Errorf("findElem boundary mismatch for prefix-sharing names: s=%d s2=%d", s, s2)
	}
}

func buildZip(files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			panic(err)
		}
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func readZipEntry(t *testing.T, raw []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, _ := f.Open()
			defer rc.Close()
			buf := new(bytes.Buffer)
			if _, err := buf.ReadFrom(rc); err != nil {
				t.Fatal(err)
			}
			return buf.String()
		}
	}
	t.Fatalf("entry %s not found", name)
	return ""
}

func docxFixture() []byte {
	return buildZip(map[string]string{
		"[Content_Types].xml":   `<Types/>`,
		"_rels/.rels":           `<Relationships/>`,
		"word/document.xml":     `<w:document/>`,
		"word/styles.xml":       fixtureStylesXML,
		"word/theme/theme1.xml": fixtureThemeXML,
	})
}

func pptxFixture() []byte {
	return buildZip(map[string]string{
		"[Content_Types].xml":               `<?xml version="1.0"?><Types/>`,
		"_rels/.rels":                       `<Relationships/>`,
		"ppt/presentation.xml":              `<?xml version="1.0"?><p:presentation/>`,
		"ppt/theme/theme1.xml":              fixtureThemeXML,
		"ppt/slideMasters/slideMaster1.xml": `<?xml version="1.0"?><p:sldMaster><p:cSld><p:spTree/></p:cSld></p:sldMaster>`,
	})
}

func TestApplyFileDocx(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.docx")
	if err := os.WriteFile(path, docxFixture(), 0o644); err != nil {
		t.Fatal(err)
	}
	th := testTheme()
	res, err := ApplyFile(path, th, true)
	if err != nil {
		t.Fatalf("ApplyFile: %v", err)
	}
	if res.Kind != "word" {
		t.Errorf("kind = %s", res.Kind)
	}
	if raw, _ := os.ReadFile(path + ".bak"); len(raw) == 0 {
		t.Error("backup not written")
	}
	patched, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	themeOut := readZipEntry(t, patched, "word/theme/theme1.xml")
	if !strings.Contains(themeOut, `val="81A1C1"`) {
		t.Error("docx theme not patched")
	}
	stylesOut := readZipEntry(t, patched, "word/styles.xml")
	if !strings.Contains(stylesOut, `w:asciiTheme="minorHAnsi"`) {
		t.Error("docx styles not patched")
	}
	// Content types must survive byte-identical.
	if got := readZipEntry(t, patched, "[Content_Types].xml"); got != `<Types/>` {
		t.Errorf("content types changed: %q", got)
	}
}

func TestApplyFilePptx(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.pptx")
	if err := os.WriteFile(path, pptxFixture(), 0o644); err != nil {
		t.Fatal(err)
	}
	th := testTheme()
	res, err := ApplyFile(path, th, false)
	if err != nil {
		t.Fatalf("ApplyFile: %v", err)
	}
	if res.Kind != "powerpoint" {
		t.Errorf("kind = %s", res.Kind)
	}
	patched, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if out := readZipEntry(t, patched, "ppt/theme/theme1.xml"); !strings.Contains(out, `val="81A1C1"`) {
		t.Error("pptx theme not patched")
	}
	if out := readZipEntry(t, patched, "ppt/slideMasters/slideMaster1.xml"); !strings.Contains(out, `<p:bg><p:bgRef idx="1001"><a:schemeClr val="lt1"/></p:bgRef></p:bg>`) {
		t.Errorf("slide master background not patched:\n%s", out)
	}
}

func TestApplyFileRejectsNonOffice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyFile(path, testTheme(), false); err == nil {
		t.Error("expected error for non-office file")
	}
}

func TestExtractTheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deck.pptx")
	if err := os.WriteFile(path, pptxFixture(), 0o644); err != nil {
		t.Fatal(err)
	}
	th, err := ExtractTheme(path)
	if err != nil {
		t.Fatalf("ExtractTheme: %v", err)
	}
	if th.Base.Colors["accent1"] != "4472C4" || th.Base.Colors["dk1"] != "000000" {
		t.Errorf("extracted colors: %+v", th.Base.Colors)
	}
	if th.Base.MajorFont != "Calibri Light" || th.Base.MinorFont != "Calibri" {
		t.Errorf("extracted fonts: %+v", th.Base)
	}
}
