package theme

import (
	"strings"
	"testing"
)

const nordish = `
:root {
  --text: #2e3440;
  --background: #ECEFF4;
  --dk2: #4C566A;
  --accent1: #81A1C1;
  --accent2: #8FBCBB;
  --heading-font: "Avenir Next";
  --body-font: "Helvetica Neue";
}
@word {
  :root { --text: #111111; }
  h1 { size: 20pt; bold: true; color: var(--accent1); space-before: 18pt; space-after: 6pt; }
  h2 { size: 15pt; bold: false; italic: true; }
}
@ppt { background: var(--lt1); }
`

func TestFromCSS(t *testing.T) {
	th, warns, err := FromCSS(nordish, "nordish")
	if err != nil {
		t.Fatalf("FromCSS: %v", err)
	}
	if len(warns) != 0 {
		t.Fatalf("unexpected warnings: %v", warns)
	}
	if th.Base.Colors["dk1"] != "2E3440" {
		t.Errorf("alias --text failed: %q", th.Base.Colors["dk1"])
	}
	if th.Base.MajorFont != "Avenir Next" || th.Base.MinorFont != "Helvetica Neue" {
		t.Errorf("fonts: %+v", th.Base)
	}

	word := th.ForWord()
	if word.Colors["dk1"] != "111111" {
		t.Errorf("@word :root override failed: %q", word.Colors["dk1"])
	}
	if word.Colors["accent1"] != "81A1C1" {
		t.Errorf("base accent1 lost in word palette: %q", word.Colors["accent1"])
	}
	if word.MajorFont != "Avenir Next" {
		t.Errorf("word major font: %q", word.MajorFont)
	}

	h1 := th.WordHeadings[1]
	if h1 == nil || *h1.SizePt != 20 || !*h1.Bold || h1.Color != "81A1C1" ||
		*h1.SpaceBeforePt != 18 || *h1.SpaceAfterPt != 6 {
		t.Errorf("h1 = %+v", h1)
	}
	h2 := th.WordHeadings[2]
	if h2 == nil || *h2.Bold || !*h2.Italic || h2.Color != "" {
		t.Errorf("h2 = %+v", h2)
	}
	if th.PPTBackground != "scheme:lt1" {
		t.Errorf("ppt background = %q", th.PPTBackground)
	}

	// Filled() derives the missing slots deterministically.
	ppt := th.ForPPT()
	if ppt.Colors["lt1"] != "ECEFF4" {
		t.Errorf("lt1 = %q", ppt.Colors["lt1"])
	}
	for _, k := range ColorKeys {
		if ppt.Colors[k] == "" {
			t.Errorf("slot %s unfilled", k)
		}
	}
}

func TestFromCSSThreeDigitHexAndWarnings(t *testing.T) {
	src := ":root { --text: #abc; --bogus: 1; }\n@excel { a { b: c; } }\n@word { h9 { size: 3pt; } }"
	th, warns, err := FromCSS(src, "t")
	if err != nil {
		t.Fatalf("FromCSS: %v", err)
	}
	if th.Base.Colors["dk1"] != "AABBCC" {
		t.Errorf("3-digit hex: %q", th.Base.Colors["dk1"])
	}
	joined := strings.Join(warns, "\n")
	for _, want := range []string{"--bogus", "@excel", "h9"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected a warning mentioning %q, got:\n%s", want, joined)
		}
	}
}

func TestToCSSRoundTrip(t *testing.T) {
	th, _, err := FromCSS(nordish, "nordish")
	if err != nil {
		t.Fatalf("FromCSS: %v", err)
	}
	out := th.ToCSS()
	th2, warns, err := FromCSS(out, "nordish")
	if err != nil {
		t.Fatalf("re-parse ToCSS output: %v\n%s", err, out)
	}
	if len(warns) != 0 {
		t.Fatalf("round-trip warnings: %v\n%s", warns, out)
	}
	w1, w2 := th.ForWord(), th2.ForWord()
	for _, k := range ColorKeys {
		if w1.Colors[k] != w2.Colors[k] {
			t.Errorf("round-trip color %s: %s != %s", k, w1.Colors[k], w2.Colors[k])
		}
	}
	if th.PPTBackground != th2.PPTBackground {
		t.Errorf("round-trip background: %q != %q", th.PPTBackground, th2.PPTBackground)
	}
	if len(th.WordHeadings) != len(th2.WordHeadings) {
		t.Fatalf("round-trip headings: %d != %d", len(th.WordHeadings), len(th2.WordHeadings))
	}
}

func TestNormalizeHexErrors(t *testing.T) {
	for _, bad := range []string{"#GGGGGG", "#12345", "blue", "var("} {
		if _, err := NormalizeHex(bad); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
	if h, err := NormalizeHex("#a5f"); err != nil || h != "AA55FF" {
		t.Errorf("NormalizeHex(#a5f) = %q, %v", h, err)
	}
}
