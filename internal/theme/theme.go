// Package theme defines the ostyle theme model and converts parsed CSS
// stylesheets into it. A theme has a base palette (12 OOXML color slots plus
// major/minor fonts), optional @word and @ppt overrides, Word heading style
// tweaks, and an optional PowerPoint background.
package theme

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"office-palette-changer/internal/css"
)

// ColorKeys are the 12 theme color slots of an OOXML clrScheme, in order.
var ColorKeys = []string{
	"dk1", "lt1", "dk2", "lt2",
	"accent1", "accent2", "accent3", "accent4", "accent5", "accent6",
	"hlink", "folHlink",
}

var colorAliases = map[string]string{
	"dk1": "dk1", "text": "dk1", "tx1": "dk1",
	"lt1": "lt1", "background": "lt1", "bg": "lt1",
	"dk2": "dk2", "lt2": "lt2",
	"accent1": "accent1", "primary": "accent1",
	"accent2": "accent2", "secondary": "accent2",
	"accent3": "accent3", "accent4": "accent4",
	"accent5": "accent5", "accent6": "accent6",
	"hlink": "hlink", "hyperlink": "hlink", "link": "hlink",
	"folhlink": "folHlink", "followed-link": "folHlink",
}

var majorFontAliases = []string{"font-major", "heading-font", "font-heading"}
var minorFontAliases = []string{"font-minor", "body-font", "font-body"}

// HeadingStyle holds explicit overrides for a Word Heading1-6 style.
// Nil fields mean "leave as is".
type HeadingStyle struct {
	SizePt        *float64
	Bold          *bool
	Italic        *bool
	Color         string // "RRGGBB" or ""
	SpaceBeforePt *float64
	SpaceAfterPt  *float64
}

// Palette is a set of theme colors (hex without '#', uppercase) and fonts.
// Empty strings mean "not set".
type Palette struct {
	Colors    map[string]string
	MajorFont string
	MinorFont string
}

type Theme struct {
	Name string
	Base Palette
	Word Palette // overrides applied when patching .docx
	PPT  Palette // overrides applied when patching .pptx

	WordHeadings map[int]*HeadingStyle
	// PPTBackground is "", "scheme:<key>" or "hex:RRGGBB".
	PPTBackground string
}

var varRe = regexp.MustCompile(`^var\(\s*(--[\w-]+)\s*\)$`)

// NormalizeHex accepts #RGB/#RRGGBB (case-insensitive, '#' optional) and
// returns uppercase RRGGBB.
func NormalizeHex(s string) (string, error) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "#"))
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return "", fmt.Errorf("%q is not a hex color (#RGB or #RRGGBB)", s)
	}
	s = strings.ToUpper(s)
	for i := 0; i < 6; i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'F') {
			return "", fmt.Errorf("%q is not a hex color", s)
		}
	}
	return s, nil
}

// MergePalette returns base with every set value of over applied on top.
func MergePalette(base, over Palette) Palette {
	m := Palette{Colors: map[string]string{}, MajorFont: base.MajorFont, MinorFont: base.MinorFont}
	if over.MajorFont != "" {
		m.MajorFont = over.MajorFont
	}
	if over.MinorFont != "" {
		m.MinorFont = over.MinorFont
	}
	for k, v := range base.Colors {
		m.Colors[k] = v
	}
	for k, v := range over.Colors {
		m.Colors[k] = v
	}
	return m
}

// Filled returns a copy of p with all 12 color slots set, deriving sensible
// values for anything missing.
func (p Palette) Filled() Palette {
	q := Palette{Colors: map[string]string{}, MajorFont: p.MajorFont, MinorFont: p.MinorFont}
	for k, v := range p.Colors {
		q.Colors[k] = v
	}
	set := func(k, v string) {
		if q.Colors[k] == "" {
			q.Colors[k] = v
		}
	}
	set("dk1", "000000")
	set("lt1", "FFFFFF")
	set("dk2", q.Colors["dk1"])
	set("lt2", q.Colors["lt1"])
	for i := 1; i <= 6; i++ {
		key := fmt.Sprintf("accent%d", i)
		t := float64(i) / 7.0
		set(key, mix(q.Colors["dk1"], q.Colors["lt1"], t))
	}
	set("hlink", q.Colors["accent1"])
	set("folHlink", q.Colors["hlink"])
	return q
}

func mix(a, b string, t float64) string {
	ar, ag, ab := hexRGB(a)
	br, bg, bb := hexRGB(b)
	r := int(math.Round(float64(ar) + t*float64(br-ar)))
	g := int(math.Round(float64(ag) + t*float64(bg-ag)))
	bl := int(math.Round(float64(ab) + t*float64(bb-ab)))
	return fmt.Sprintf("%02X%02X%02X", r, g, bl)
}

func hexRGB(h string) (int, int, int) {
	if len(h) != 6 {
		return 0, 0, 0
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return 0, 0, 0
	}
	return int(v >> 16 & 0xFF), int(v >> 8 & 0xFF), int(v & 0xFF)
}

func (t *Theme) ForWord() Palette { return MergePalette(t.Base, t.Word).Filled() }
func (t *Theme) ForPPT() Palette  { return MergePalette(t.Base, t.PPT).Filled() }

// FromCSS builds a Theme from a parsed stylesheet. It returns the theme,
// human-readable warnings for ignored constructs, and a fatal error only for
// malformed values in constructs that were otherwise accepted.
func FromCSS(src, name string) (*Theme, []string, error) {
	ss, err := css.Parse(src)
	if err != nil {
		return nil, nil, err
	}
	t := &Theme{Name: name, WordHeadings: map[int]*HeadingStyle{}}
	var warns []string
	warn := func(format string, a ...any) {
		warns = append(warns, fmt.Sprintf(format, a...))
	}

	for _, r := range ss.Rules {
		if r.Selector == ":root" {
			applyPaletteDecls(&t.Base, r.Decls, Palette{}, warn)
		} else {
			warn("line %d: ignoring top-level rule %q (only :root is allowed at top level)", r.Line, r.Selector)
		}
	}

	for _, ar := range ss.AtRules {
		switch ar.Name {
		case "word", "ppt":
			var pal *Palette
			if ar.Name == "word" {
				if t.Word.Colors == nil {
					t.Word = Palette{Colors: map[string]string{}}
				}
				pal = &t.Word
			} else {
				if t.PPT.Colors == nil {
					t.PPT = Palette{Colors: map[string]string{}}
				}
				pal = &t.PPT
			}
			for _, d := range ar.Decls {
				if ar.Name == "ppt" && d.Prop == "background" {
					bg, err := resolveBackground(d.Value, MergePalette(t.Base, t.PPT))
					if err != nil {
						warn("line %d: bad background value: %v", d.Line, err)
					} else {
						t.PPTBackground = bg
					}
					continue
				}
				warn("line %d: ignoring declaration %q inside @%s", d.Line, d.Prop, ar.Name)
			}
			// First pass: :root overrides so later rules can var() them.
			var elementRules []css.Rule
			for _, r := range ar.Rules {
				if r.Selector == ":root" {
					applyPaletteDecls(pal, r.Decls, t.Base, warn)
				} else {
					elementRules = append(elementRules, r)
				}
			}
			if ar.Name == "word" {
				effective := MergePalette(t.Base, t.Word)
				for _, r := range elementRules {
					lvl, ok := headingLevel(r.Selector)
					if !ok {
						warn("line %d: ignoring rule %q inside @word (use h1..h6 or :root)", r.Line, r.Selector)
						continue
					}
					t.WordHeadings[lvl] = parseHeading(r, effective, warn)
				}
			} else {
				for _, r := range elementRules {
					warn("line %d: ignoring rule %q inside @ppt (only :root is allowed)", r.Line, r.Selector)
				}
			}
		default:
			warn("line %d: ignoring unknown at-rule @%s", ar.Line, ar.Name)
		}
	}
	return t, warns, nil
}

func applyPaletteDecls(pal *Palette, decls []css.Decl, base Palette, warn func(string, ...any)) {
	for _, d := range decls {
		if !strings.HasPrefix(d.Prop, "--") {
			warn("line %d: ignoring non-custom property %q (use --name)", d.Line, d.Prop)
			continue
		}
		key := strings.ToLower(strings.TrimPrefix(d.Prop, "--"))
		merged := MergePalette(base, *pal)
		if canon, ok := colorAliases[key]; ok {
			v, err := resolveColorValue(d.Value, merged)
			if err != nil {
				warn("line %d: %v", d.Line, err)
				continue
			}
			if pal.Colors == nil {
				pal.Colors = map[string]string{}
			}
			pal.Colors[canon] = v
			continue
		}
		if contains(majorFontAliases, key) {
			pal.MajorFont = css.StringValue(d)
			continue
		}
		if contains(minorFontAliases, key) {
			pal.MinorFont = css.StringValue(d)
			continue
		}
		warn("line %d: unknown custom property --%s", d.Line, key)
	}
}

func resolveColorValue(val string, pal Palette) (string, error) {
	val = strings.TrimSpace(val)
	if m := varRe.FindStringSubmatch(val); m != nil {
		key := strings.ToLower(strings.TrimPrefix(m[1], "--"))
		if canon, ok := colorAliases[key]; ok {
			if v := pal.Colors[canon]; v != "" {
				return v, nil
			}
		}
		return "", fmt.Errorf("unknown or unset color variable %s", m[1])
	}
	return NormalizeHex(val)
}

func resolveBackground(val string, pal Palette) (string, error) {
	val = strings.TrimSpace(val)
	if m := varRe.FindStringSubmatch(val); m != nil {
		key := strings.ToLower(strings.TrimPrefix(m[1], "--"))
		if canon, ok := colorAliases[key]; ok {
			return "scheme:" + canon, nil
		}
		return "", fmt.Errorf("unknown color variable %s", m[1])
	}
	h, err := NormalizeHex(val)
	if err != nil {
		return "", err
	}
	return "hex:" + h, nil
}

func headingLevel(sel string) (int, bool) {
	s := strings.ToLower(sel)
	for i := 1; i <= 6; i++ {
		if s == fmt.Sprintf("h%d", i) || s == fmt.Sprintf("heading%d", i) {
			return i, true
		}
	}
	return 0, false
}

func parseHeading(r css.Rule, pal Palette, warn func(string, ...any)) *HeadingStyle {
	hs := &HeadingStyle{}
	for _, d := range r.Decls {
		switch d.Prop {
		case "size", "font-size":
			if pt, err := parsePt(d.Value); err == nil {
				hs.SizePt = &pt
			} else {
				warn("line %d: %v", d.Line, err)
			}
		case "bold":
			if b, err := parseBool(d.Value); err == nil {
				hs.Bold = &b
			} else {
				warn("line %d: %v", d.Line, err)
			}
		case "italic":
			if b, err := parseBool(d.Value); err == nil {
				hs.Italic = &b
			} else {
				warn("line %d: %v", d.Line, err)
			}
		case "color":
			if v, err := resolveColorValue(d.Value, pal); err == nil {
				hs.Color = v
			} else {
				warn("line %d: %v", d.Line, err)
			}
		case "space-before", "spacing-before", "before":
			if pt, err := parsePt(d.Value); err == nil {
				hs.SpaceBeforePt = &pt
			} else {
				warn("line %d: %v", d.Line, err)
			}
		case "space-after", "spacing-after", "after":
			if pt, err := parsePt(d.Value); err == nil {
				hs.SpaceAfterPt = &pt
			} else {
				warn("line %d: %v", d.Line, err)
			}
		default:
			warn("line %d: unknown heading property %q", d.Line, d.Prop)
		}
	}
	return hs
}

func parsePt(v string) (float64, error) {
	v = strings.TrimSpace(strings.ToLower(v))
	v = strings.TrimSuffix(v, "pt")
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 {
		return 0, fmt.Errorf("%q is not a point size (e.g. 18pt)", v)
	}
	return f, nil
}

func parseBool(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "yes", "on":
		return true, nil
	case "false", "no", "off":
		return false, nil
	}
	return false, fmt.Errorf("%q is not a boolean", v)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ToCSS renders a theme back to the CSS theme format (used by `ostyle new`).
func (t *Theme) ToCSS() string {
	var b strings.Builder
	fmt.Fprintf(&b, "/* ostyle theme: %s */\n", t.Name)
	b.WriteString(":root {\n")
	for _, k := range ColorKeys {
		if v := t.Base.Colors[k]; v != "" {
			fmt.Fprintf(&b, "  --%s: #%s;\n", k, v)
		}
	}
	if t.Base.MajorFont != "" {
		fmt.Fprintf(&b, "  --heading-font: %q;\n", t.Base.MajorFont)
	}
	if t.Base.MinorFont != "" {
		fmt.Fprintf(&b, "  --body-font: %q;\n", t.Base.MinorFont)
	}
	b.WriteString("}\n")
	if len(t.Word.Colors) > 0 || t.Word.MajorFont != "" || t.Word.MinorFont != "" {
		b.WriteString("@word {\n  :root {\n")
		for _, k := range ColorKeys {
			if v := t.Word.Colors[k]; v != "" {
				fmt.Fprintf(&b, "    --%s: #%s;\n", k, v)
			}
		}
		if t.Word.MajorFont != "" {
			fmt.Fprintf(&b, "    --heading-font: %q;\n", t.Word.MajorFont)
		}
		if t.Word.MinorFont != "" {
			fmt.Fprintf(&b, "    --body-font: %q;\n", t.Word.MinorFont)
		}
		b.WriteString("  }\n}\n")
	}
	if len(t.WordHeadings) > 0 {
		b.WriteString("@word {\n")
		for lvl := 1; lvl <= 6; lvl++ {
			hs := t.WordHeadings[lvl]
			if hs == nil {
				continue
			}
			fmt.Fprintf(&b, "  h%d {", lvl)
			if hs.SizePt != nil {
				fmt.Fprintf(&b, " size: %gpt;", *hs.SizePt)
			}
			if hs.Bold != nil {
				fmt.Fprintf(&b, " bold: %t;", *hs.Bold)
			}
			if hs.Italic != nil {
				fmt.Fprintf(&b, " italic: %t;", *hs.Italic)
			}
			if hs.Color != "" {
				fmt.Fprintf(&b, " color: #%s;", hs.Color)
			}
			if hs.SpaceBeforePt != nil {
				fmt.Fprintf(&b, " space-before: %gpt;", *hs.SpaceBeforePt)
			}
			if hs.SpaceAfterPt != nil {
				fmt.Fprintf(&b, " space-after: %gpt;", *hs.SpaceAfterPt)
			}
			b.WriteString(" }\n")
		}
		b.WriteString("}\n")
	}
	if t.PPTBackground != "" {
		b.WriteString("@ppt {\n")
		if strings.HasPrefix(t.PPTBackground, "scheme:") {
			fmt.Fprintf(&b, "  background: var(--%s);\n", strings.TrimPrefix(t.PPTBackground, "scheme:"))
		} else {
			fmt.Fprintf(&b, "  background: #%s;\n", strings.TrimPrefix(t.PPTBackground, "hex:"))
		}
		b.WriteString("}\n")
	}
	return b.String()
}
