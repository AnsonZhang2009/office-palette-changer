package palette

import (
	"math"
	"strings"
	"testing"

	"office-palette-changer/internal/theme"
)

func mustGen(t *testing.T, inputs []string, opts Options) *Result {
	t.Helper()
	res, err := Generate(inputs, opts)
	if err != nil {
		t.Fatalf("Generate(%v): %v", inputs, err)
	}
	return res
}

func assertFullPalette(t *testing.T, res *Result) {
	t.Helper()
	for _, k := range theme.ColorKeys {
		hex := res.Palette.Colors[k]
		if len(hex) != 6 {
			t.Errorf("slot %s = %q (want RRGGBB)", k, hex)
		}
		if _, err := ParseHex(hex); err != nil {
			t.Errorf("slot %s = %q not hex: %v", k, hex, err)
		}
		if res.Slots[k].Origin == "" {
			t.Errorf("slot %s missing origin", k)
		}
	}
}

func TestColorMathRoundTrip(t *testing.T) {
	for _, hex := range []string{"000000", "FFFFFF", "FF5A5F", "0B3954", "087E8B", "808080", "123456"} {
		c, err := ParseHex(hex)
		if err != nil {
			t.Fatal(err)
		}
		back := c.HSL().RGB().Hex()
		if back != hex {
			t.Errorf("round trip %s -> %s", hex, back)
		}
	}
	w, _ := ParseHex("FFFFFF")
	b, _ := ParseHex("000000")
	if r := Contrast(w, b); math.Abs(r-21) > 0.01 {
		t.Errorf("white/black contrast = %.2f, want 21", r)
	}
	if d := HueDistance(350, 10); d != 20 {
		t.Errorf("hue distance 350..10 = %v, want 20", d)
	}
}

func TestGenerateDeterministic(t *testing.T) {
	in := []string{"#0B3954", "#087E8B", "#FF5A5F"}
	a := mustGen(t, in, Options{})
	b := mustGen(t, in, Options{})
	for _, k := range theme.ColorKeys {
		if a.Palette.Colors[k] != b.Palette.Colors[k] {
			t.Errorf("non-deterministic slot %s: %s vs %s", k, a.Palette.Colors[k], b.Palette.Colors[k])
		}
	}
}

func TestGenerateThreeSeeds(t *testing.T) {
	res := mustGen(t, []string{"#0B3954", "#087E8B", "#B0563F"}, Options{})
	assertFullPalette(t, res)
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}
	for i, want := range []string{"0B3954", "087E8B", "B0563F"} {
		k := "accent" + string(rune('1'+i))
		if res.Palette.Colors[k] != want || res.Slots[k].Origin != "yours" {
			t.Errorf("%s = %+v, want %s (yours)", k, res.Slots[k], want)
		}
	}
	for _, k := range []string{"accent4", "accent5", "accent6"} {
		if res.Slots[k].Origin != "generated" {
			t.Errorf("%s origin = %q, want generated", k, res.Slots[k].Origin)
		}
	}
	dk1, _ := ParseHex(res.Palette.Colors["dk1"])
	lt1, _ := ParseHex(res.Palette.Colors["lt1"])
	if c := Contrast(dk1, lt1); c < minTextContrast {
		t.Errorf("text/background contrast %.1f < %.1f", c, minTextContrast)
	}
	if dk1.HSL().L > 0.2 || lt1.HSL().L < 0.9 {
		t.Errorf("light-mode neutrals wrong: dk1 L=%.2f lt1 L=%.2f", dk1.HSL().L, lt1.HSL().L)
	}
}

func TestGenerateSingleSeedDistinctAccents(t *testing.T) {
	res := mustGen(t, []string{"#C05B3C"}, Options{})
	assertFullPalette(t, res)
	var accents []HSL
	for i := 1; i <= 6; i++ {
		c, _ := ParseHex(res.Palette.Colors["accent"+string(rune('0'+i))])
		accents = append(accents, c.HSL())
	}
	for i := range accents {
		for j := i + 1; j < len(accents); j++ {
			a, b := accents[i], accents[j]
			if HueDistance(a.H, b.H) < 18 && math.Abs(a.L-b.L) < 0.12 && math.Abs(a.S-b.S) < 0.25 {
				t.Errorf("accents %d and %d too similar: %+v vs %+v", i+1, j+1, a, b)
			}
		}
	}
	lt1, _ := ParseHex(res.Palette.Colors["lt1"])
	for i := 2; i <= 6; i++ { // generated ones must be legible on the background
		c, _ := ParseHex(res.Palette.Colors["accent"+string(rune('0'+i))])
		if Contrast(c, lt1) < minAccentContrast {
			t.Errorf("generated accent%d has contrast %.1f against background", i, Contrast(c, lt1))
		}
	}
}

func TestAutoDetectRoles(t *testing.T) {
	res := mustGen(t, []string{"#111111", "#FAFAFA", "#FF5A5F"}, Options{})
	if res.Palette.Colors["dk1"] != "111111" || res.Slots["dk1"].Origin != "yours" {
		t.Errorf("dark input not detected as text: %+v", res.Slots["dk1"])
	}
	if res.Palette.Colors["lt1"] != "FAFAFA" || res.Slots["lt1"].Origin != "yours" {
		t.Errorf("light input not detected as background: %+v", res.Slots["lt1"])
	}
	if res.Palette.Colors["accent1"] != "FF5A5F" {
		t.Errorf("accent1 = %s", res.Palette.Colors["accent1"])
	}
}

func TestExplicitRolesWinOverAutoDetect(t *testing.T) {
	res := mustGen(t, []string{"#101010", "#FF5A5F"}, Options{Text: "#222222"})
	if res.Palette.Colors["dk1"] != "222222" || !strings.Contains(res.Slots["dk1"].Note, "--text") {
		t.Errorf("explicit --text lost: %+v", res.Slots["dk1"])
	}
	// The dark positional could not take the text role, so it is an accent.
	found := false
	for i := 1; i <= 6; i++ {
		if res.Palette.Colors["accent"+string(rune('0'+i))] == "101010" {
			found = true
		}
	}
	if !found {
		t.Error("displaced dark positional should have become an accent")
	}
	res2 := mustGen(t, []string{"#FF5A5F"}, Options{Accents: []string{"#087E8B"}})
	if res2.Palette.Colors["accent1"] != "087E8B" || res2.Palette.Colors["accent2"] != "FF5A5F" {
		t.Errorf("--accent should come first: %s, %s", res2.Palette.Colors["accent1"], res2.Palette.Colors["accent2"])
	}
}

func TestDarkMode(t *testing.T) {
	res := mustGen(t, []string{"#7AA2F7", "#9D7CD8"}, Options{Dark: true})
	assertFullPalette(t, res)
	dk1, _ := ParseHex(res.Palette.Colors["dk1"])
	lt1, _ := ParseHex(res.Palette.Colors["lt1"])
	if dk1.HSL().L < 0.8 || lt1.HSL().L > 0.2 {
		t.Errorf("dark-mode neutrals wrong: dk1 L=%.2f lt1 L=%.2f", dk1.HSL().L, lt1.HSL().L)
	}
	if Contrast(dk1, lt1) < minTextContrast {
		t.Errorf("dark-mode contrast %.1f", Contrast(dk1, lt1))
	}
	if res.WordOverrides == nil || res.WordOverrides["lt1"] != "FFFFFF" {
		t.Fatalf("dark mode must provide Word overrides, got %v", res.WordOverrides)
	}
	wt, _ := ParseHex(res.WordOverrides["dk1"])
	if wt.HSL().L > 0.2 {
		t.Errorf("Word text override should be dark, got L=%.2f", wt.HSL().L)
	}
	white := RGB{1, 1, 1}
	for k, hex := range res.WordOverrides {
		if k == "lt1" || k == "lt2" { // background slots are light by design
			continue
		}
		c, _ := ParseHex(hex)
		if Contrast(c, white) < minAccentContrast {
			t.Errorf("Word override %s=%s not legible on white (%.1f)", k, hex, Contrast(c, white))
		}
	}
	// In dark mode a very dark positional is the canvas, not the text.
	res2 := mustGen(t, []string{"#0D1117", "#58A6FF"}, Options{Dark: true})
	if res2.Palette.Colors["lt1"] != "0D1117" {
		t.Errorf("dark input should be background in dark mode, lt1=%s", res2.Palette.Colors["lt1"])
	}
}

func TestUserAccentPreservedButWarned(t *testing.T) {
	res := mustGen(t, []string{"#FFF59D", "#0B3954"}, Options{})
	if res.Palette.Colors["accent1"] != "FFF59D" {
		t.Errorf("user accent was modified: %s", res.Palette.Colors["accent1"])
	}
	joined := strings.Join(res.Warnings, "\n")
	if !strings.Contains(joined, "accent 1") || !strings.Contains(joined, "contrast") {
		t.Errorf("expected low-contrast warning for accent 1, got: %v", res.Warnings)
	}
}

func TestExtraAccentsIgnored(t *testing.T) {
	in := []string{"#C05B3C", "#7C8A5A", "#D9A05B", "#A5643B", "#5E7360", "#E2B75B", "#336699", "#993366"}
	res := mustGen(t, in, Options{})
	assertFullPalette(t, res)
	if res.Palette.Colors["accent6"] != "E2B75B" {
		t.Errorf("accent6 = %s", res.Palette.Colors["accent6"])
	}
	joined := strings.Join(res.Warnings, "\n")
	if !strings.Contains(joined, "#336699") || !strings.Contains(joined, "#993366") {
		t.Errorf("expected warnings about ignored extras, got: %v", res.Warnings)
	}
}

func TestNoAccentSynthesized(t *testing.T) {
	res := mustGen(t, []string{"#1F2328"}, Options{})
	assertFullPalette(t, res)
	if res.Slots["accent1"].Origin != "synthesized" {
		t.Errorf("accent1 origin = %q", res.Slots["accent1"].Origin)
	}
	if len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], "synthesized") {
		t.Errorf("expected synthesized warning, got %v", res.Warnings)
	}
}

func TestBadInputs(t *testing.T) {
	if _, err := Generate([]string{"blue"}, Options{}); err == nil {
		t.Error("expected error for named color")
	}
	if _, err := Generate([]string{"#FF5A5F"}, Options{Text: "#12"}); err == nil {
		t.Error("expected error for bad --text")
	}
	if _, err := Generate(nil, Options{Accents: []string{"zzz"}}); err == nil {
		t.Error("expected error for bad --accent")
	}
}
