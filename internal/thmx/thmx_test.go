package thmx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"office-palette-changer/internal/theme"
)

func TestBuild(t *testing.T) {
	src := `
:root {
  --text: #2E3440; --background: #ECEFF4;
  --accent1: #81A1C1; --accent2: #8FBCBB; --accent3: #A3BE8C;
  --accent4: #EBCB8B; --accent5: #B48EAD; --accent6: #88C0D0;
  --heading-font: "Avenir Next"; --body-font: "Helvetica Neue";
}
`
	th, _, err := theme.FromCSS(src, "nord")
	if err != nil {
		t.Fatal(err)
	}
	data, err := Build(th)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	found := map[string]string{}
	for _, f := range zr.File {
		rc, _ := f.Open()
		buf := new(bytes.Buffer)
		buf.ReadFrom(rc)
		rc.Close()
		found[f.Name] = buf.String()
	}
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "theme/theme1.xml"} {
		if _, ok := found[name]; !ok {
			t.Errorf("missing entry %s", name)
		}
	}
	t1 := found["theme/theme1.xml"]
	for _, want := range []string{
		`<a:dk1><a:srgbClr val="2E3440"/></a:dk1>`,
		`<a:accent1><a:srgbClr val="81A1C1"/></a:accent1>`,
		`<a:latin typeface="Avenir Next"/>`,
		`<a:latin typeface="Helvetica Neue"/>`,
		`<a:font script="Jpan" typeface="游ゴシック"/>`,
		`<a:fmtScheme`, `<a:bgFillStyleLst`, `<a:effectStyleLst`,
	} {
		if !strings.Contains(t1, want) {
			t.Errorf("theme1.xml missing %s", want)
		}
	}
	if !strings.Contains(found["_rels/.rels"], "themeManager") {
		t.Error("rels missing themeManager relationship")
	}
	// Balanced tags sanity: every <a: has a matching </a: or is self-closed.
	opens := strings.Count(t1, "<a:")
	closes := strings.Count(t1, "</a:") + strings.Count(t1, "/>")
	if opens != strings.Count(t1, "</a:")+strings.Count(t1, "/>") {
		t.Errorf("unbalanced tags: %d opens vs %d closes", opens, closes)
	}
}
