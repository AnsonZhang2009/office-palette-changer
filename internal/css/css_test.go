package css

import (
	"testing"
)

func TestParseFullTheme(t *testing.T) {
	src := `
/* comment with : tricky { chars } */
:root {
  --text: #2E3440;
  --background: #ECEFF4;
  --heading-font: "Avenir Next";
  hlink-should-not-be-here: ignored-rule;
}
@word {
  h1 { size: 20pt; bold: true; color: var(--accent1); space-before: 18pt; }
}
@ppt { background: var(--lt1); }
`
	ss, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ss.Rules) != 1 || ss.Rules[0].Selector != ":root" {
		t.Fatalf("expected one :root rule, got %+v", ss.Rules)
	}
	root := ss.Rules[0]
	if len(root.Decls) != 4 {
		t.Fatalf("expected 4 decls, got %d: %+v", len(root.Decls), root.Decls)
	}
	if root.Decls[0].Prop != "--text" || root.Decls[0].Value != "#2E3440" {
		t.Errorf("decl 0 = %+v", root.Decls[0])
	}
	if root.Decls[2].Value != `"Avenir Next"` {
		t.Errorf("string decl = %q", root.Decls[2].Value)
	}

	if len(ss.AtRules) != 2 {
		t.Fatalf("expected 2 at-rules, got %d", len(ss.AtRules))
	}
	word := ss.AtRules[0]
	if word.Name != "word" || len(word.Rules) != 1 || word.Rules[0].Selector != "h1" {
		t.Fatalf("word at-rule = %+v", word)
	}
	d := word.Rules[0].Decls
	if d[2].Value != "var(--accent1)" {
		t.Errorf("var() decl = %q", d[2].Value)
	}
	if d[0].Value != "20pt" {
		t.Errorf("size decl = %q", d[0].Value)
	}
	ppt := ss.AtRules[1]
	if ppt.Name != "ppt" || len(ppt.Decls) != 1 || ppt.Decls[0].Prop != "background" {
		t.Fatalf("ppt at-rule = %+v", ppt)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []string{
		":root { --a: #FFF ",  // missing }
		"@word { h1 ",         // missing }
		":root { --a }",       // missing :
		":root { --a: 'x'; }", // single-quote string unsupported
		"/* unterminated",
	}
	for _, src := range cases {
		if _, err := Parse(src); err == nil {
			t.Errorf("expected error for %q", src)
		}
	}
}

func TestParseColonRootInAtRule(t *testing.T) {
	src := "@word { :root { --text: #111111; } h2 { size: 14pt; } }"
	ss, err := Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ar := ss.AtRules[0]
	if len(ar.Rules) != 2 || ar.Rules[0].Selector != ":root" || ar.Rules[1].Selector != "h2" {
		t.Fatalf("at-rule rules = %+v", ar.Rules)
	}
}
