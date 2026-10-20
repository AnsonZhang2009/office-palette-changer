package palette

import (
	"fmt"
	"math"

	"office-palette-changer/internal/theme"
)

// Options carry explicit role assignments; anything left empty is
// auto-detected from the positional inputs or derived.
type Options struct {
	Text       string
	Background string
	Accents    []string
	Dark       bool
}

// Slot describes one of the 12 generated colors and where it came from.
type Slot struct {
	Hex    string
	Origin string // "yours", "derived", "generated", "synthesized"
	Note   string
}

type Result struct {
	Slots   map[string]Slot
	Palette theme.Palette
	// WordOverrides is set for dark palettes: Word keeps dark-on-white body
	// text and gets darkened accents, since its page stays white.
	WordOverrides map[string]string
	Warnings      []string
	Dark          bool
}

// Lightness cut-offs for auto-detecting neutrals among positional colors.
// A text color must be dark *and* low-saturation: a deep navy or burgundy is
// a brand accent, not a neutral.
const (
	darkCut    = 0.22
	darkMaxSat = 0.35
	lightCut   = 0.90

	minTextContrast   = 7.0 // dk1 vs lt1 target
	minAccentContrast = 3.0 // accents vs lt1
	maxAccents        = 6
)

// Generate builds a full palette from positional seed colors plus explicit
// options. Explicit roles always win; positionals fill whatever is left.
func Generate(inputs []string, opts Options) (*Result, error) {
	res := &Result{Slots: map[string]Slot{}, Dark: opts.Dark}
	warn := func(format string, a ...any) {
		res.Warnings = append(res.Warnings, fmt.Sprintf(format, a...))
	}

	var text, bg *RGB
	textNote, bgNote := "", ""
	var seeds []RGB // user-supplied accents, in order, never modified
	var seedNotes []string

	addAccent := func(c RGB, raw, note string) {
		if len(seeds) >= maxAccents {
			warn("ignoring extra color %s — a theme has at most %d accents", raw, maxAccents)
			return
		}
		seeds = append(seeds, c)
		seedNotes = append(seedNotes, note)
	}

	if opts.Text != "" {
		c, err := ParseHex(opts.Text)
		if err != nil {
			return nil, fmt.Errorf("--text: %v", err)
		}
		text, textNote = &c, "from --text"
	}
	if opts.Background != "" {
		c, err := ParseHex(opts.Background)
		if err != nil {
			return nil, fmt.Errorf("--background: %v", err)
		}
		bg, bgNote = &c, "from --background"
	}
	for _, a := range opts.Accents {
		c, err := ParseHex(a)
		if err != nil {
			return nil, fmt.Errorf("--accent %s: %v", a, err)
		}
		addAccent(c, a, "from --accent")
	}

	for _, in := range inputs {
		c, err := ParseHex(in)
		if err != nil {
			return nil, fmt.Errorf("color %q: %v", in, err)
		}
		hsl := c.HSL()
		// In a dark palette the very dark input is the canvas and the very
		// light one is the text; in a light palette it is the other way round.
		darkRole, lightRole := &text, &bg
		darkNote, lightNote := &textNote, &bgNote
		if opts.Dark {
			darkRole, lightRole = &bg, &text
			darkNote, lightNote = &bgNote, &textNote
		}
		switch {
		case hsl.L <= darkCut && hsl.S <= darkMaxSat && *darkRole == nil:
			cc := c
			*darkRole, *darkNote = &cc, "auto-detected (dark input)"
		case hsl.L >= lightCut && *lightRole == nil:
			cc := c
			*lightRole, *lightNote = &cc, "auto-detected (light input)"
		default:
			addAccent(c, in, "yours")
		}
	}

	if len(seeds) == 0 {
		h, src := 215.0, "a default blue"
		if text != nil && text.HSL().S >= 0.08 {
			h, src = text.HSL().H, "the text color's hue"
		} else if bg != nil && bg.HSL().S >= 0.08 {
			h, src = bg.HSL().H, "the background's hue"
		}
		l := 0.48
		if opts.Dark {
			l = 0.62
		}
		seeds = append(seeds, HSL{h, 0.55, l}.RGB())
		seedNotes = append(seedNotes, "synthesized from "+src)
		warn("no accent color given — synthesized accent 1 from %s", src)
	}
	h0 := seeds[0].HSL().H

	// ---- neutrals -----------------------------------------------------------
	textDerived, bgDerived := text == nil, bg == nil
	var dk1, lt1 HSL
	if opts.Dark {
		dk1, lt1 = HSL{h0, 0.15, 0.90}, HSL{h0, 0.15, 0.10}
	} else {
		dk1, lt1 = HSL{h0, 0.18, 0.13}, HSL{h0, 0.25, 0.975}
	}
	if text != nil {
		dk1 = text.HSL()
	}
	if bg != nil {
		lt1 = bg.HSL()
	}
	for i := 0; i < 80 && Contrast(dk1.RGB(), lt1.RGB()) < minTextContrast; i++ {
		moved := false
		if textDerived {
			if opts.Dark {
				dk1.L = math.Min(dk1.L+0.02, 0.98)
			} else {
				dk1.L = math.Max(dk1.L-0.02, 0.02)
			}
			moved = true
		}
		if bgDerived {
			if opts.Dark {
				lt1.L = math.Max(lt1.L-0.01, 0.02)
			} else {
				lt1.L = math.Min(lt1.L+0.005, 0.995)
			}
			moved = true
		}
		if !moved {
			break
		}
	}
	if c := Contrast(dk1.RGB(), lt1.RGB()); c < 4.5 {
		warn("text/background contrast is only %.1f:1 — body text may be hard to read", c)
	}

	var dk2, lt2 HSL
	if opts.Dark {
		dk2 = HSL{dk1.H, math.Min(dk1.S, 0.25), clamp(dk1.L-0.18, 0, 1)}
		lt2 = HSL{lt1.H, math.Min(lt1.S+0.05, 0.30), clamp(lt1.L+0.07, 0, 1)}
	} else {
		dk2 = HSL{dk1.H, math.Min(dk1.S, 0.25), clamp(dk1.L+0.23, 0, 1)}
		lt2 = HSL{lt1.H, math.Min(lt1.S+0.05, 0.30), clamp(lt1.L-0.06, 0, 1)}
	}

	// ---- accents ------------------------------------------------------------
	bgRGB := lt1.RGB()
	lo, hi := 0.32, 0.62
	if opts.Dark {
		lo, hi = 0.55, 0.78
	}
	chosen := append([]RGB(nil), seeds...)
	notes := append([]string(nil), seedNotes...)
	origins := make([]string, len(seeds))
	for i := range origins {
		origins[i] = "yours"
		if len(seedNotes[i]) > 11 && seedNotes[i][:11] == "synthesized" {
			origins[i] = "synthesized"
		}
	}

	offsets := []float64{30, -30, 150, -150, 60, -60, 180, 90, -90, 120, -120, 45, -45}
	for _, off := range offsets {
		for i := 0; i < len(seeds) && len(chosen) < maxAccents; i++ {
			seed := seeds[i].HSL()
			cand := HSL{wrapHue(seed.H + off), clamp(seed.S, 0.35, 0.85), clamp(seed.L, lo, hi)}
			cand = legible(cand, bgRGB, opts.Dark)
			if distinct(cand, chosen) {
				chosen = append(chosen, cand.RGB())
				notes = append(notes, fmt.Sprintf("%s accent %d", relation(off), i+1))
				origins = append(origins, "generated")
			}
		}
		if len(chosen) >= maxAccents {
			break
		}
	}
	for step := 0.12; len(chosen) < maxAccents && step < 0.6; step += 0.12 {
		for _, sign := range []float64{-1, 1} {
			if len(chosen) >= maxAccents {
				break
			}
			seed := seeds[0].HSL()
			cand := legible(HSL{seed.H, seed.S, clamp(seed.L+sign*step, 0.15, 0.85)}, bgRGB, opts.Dark)
			if distinct(cand, chosen) {
				chosen = append(chosen, cand.RGB())
				notes = append(notes, "shade of accent 1")
				origins = append(origins, "generated")
			}
		}
	}
	for len(chosen) < maxAccents {
		chosen = append(chosen, HSL{h0, 0.10, 0.35 + 0.08*float64(len(chosen))}.RGB())
		notes = append(notes, "neutral filler")
		origins = append(origins, "generated")
	}

	for i := range seeds {
		if c := Contrast(seeds[i], bgRGB); c < minAccentContrast {
			warn("accent %d (#%s) has low contrast (%.1f:1) against the background — consider a darker shade",
				i+1, seeds[i].Hex(), c)
		}
	}

	// ---- links --------------------------------------------------------------
	hlinkIdx, best := -1, 61.0
	for i, c := range chosen {
		h := c.HSL()
		if h.S < 0.15 {
			continue
		}
		if d := HueDistance(h.H, 215); d < best {
			best, hlinkIdx = d, i
		}
	}
	var hlink HSL
	hlinkNote := ""
	if hlinkIdx >= 0 {
		hlink = chosen[hlinkIdx].HSL()
		hlinkNote = fmt.Sprintf("bluest accent (accent %d)", hlinkIdx+1)
	} else {
		hlink = chosen[0].HSL()
		if opts.Dark {
			hlink.L += 0.08
		} else {
			hlink.L -= 0.08
		}
		hlinkNote = "accent 1, shaded"
	}
	hlink = legible(hlink, bgRGB, opts.Dark)
	fol := HSL{wrapHue(hlink.H + 45), hlink.S, hlink.L}
	if opts.Dark {
		fol.L = clamp(fol.L+0.03, 0, 1)
	} else {
		fol.L = clamp(fol.L-0.05, 0, 1)
	}
	fol = legible(fol, bgRGB, opts.Dark)

	// ---- assemble -----------------------------------------------------------
	res.Palette = theme.Palette{Colors: map[string]string{}}
	set := func(key, hex, origin, note string) {
		res.Slots[key] = Slot{Hex: hex, Origin: origin, Note: note}
		res.Palette.Colors[key] = hex
	}
	originOf := func(explicit bool) string {
		if explicit {
			return "yours"
		}
		return "derived"
	}
	if textNote == "" {
		textNote = fmt.Sprintf("near-%s tinted with accent 1's hue", map[bool]string{true: "white", false: "black"}[opts.Dark])
	}
	if bgNote == "" {
		bgNote = fmt.Sprintf("near-%s tinted with accent 1's hue", map[bool]string{true: "black", false: "white"}[opts.Dark])
	}
	set("dk1", dk1.Hex(), originOf(!textDerived), textNote)
	set("lt1", lt1.Hex(), originOf(!bgDerived), bgNote)
	set("dk2", dk2.Hex(), "derived", "secondary text, from text color")
	set("lt2", lt2.Hex(), "derived", "secondary background, from background")
	for i := 0; i < maxAccents; i++ {
		set(fmt.Sprintf("accent%d", i+1), chosen[i].Hex(), origins[i], notes[i])
	}
	set("hlink", hlink.Hex(), "derived", hlinkNote)
	set("folHlink", fol.Hex(), "derived", "hyperlink shifted toward purple")

	if opts.Dark {
		white := RGB{1, 1, 1}
		// Word's page stays white, so give it light-mode neutrals too.
		res.WordOverrides = map[string]string{
			"dk1": HSL{h0, 0.18, 0.13}.Hex(),
			"lt1": "FFFFFF",
			"dk2": HSL{h0, 0.18, 0.36}.Hex(),
			"lt2": HSL{h0, 0.25, 0.92}.Hex(),
		}
		for _, k := range []string{"accent1", "accent2", "accent3", "accent4", "accent5", "accent6", "hlink", "folHlink"} {
			c, _ := ParseHex(res.Palette.Colors[k])
			if Contrast(c, white) < minAccentContrast {
				res.WordOverrides[k] = legible(c.HSL(), white, false).Hex()
			}
		}
	}
	return res, nil
}

// legible darkens (light mode) or lightens (dark mode) a color until it
// clears the minimum accent contrast against the background.
func legible(c HSL, bg RGB, dark bool) HSL {
	for i := 0; i < 30 && Contrast(c.RGB(), bg) < minAccentContrast; i++ {
		if dark {
			c.L = math.Min(c.L+0.03, 0.98)
		} else {
			c.L = math.Max(c.L-0.03, 0.02)
		}
	}
	return c
}

// distinct rejects candidates too close to an already chosen accent. Grayish
// colors have meaningless hue, so for them only lightness/saturation count.
func distinct(c HSL, chosen []RGB) bool {
	for _, ch := range chosen {
		h := ch.HSL()
		hd := HueDistance(c.H, h.H)
		if c.S < 0.15 || h.S < 0.15 {
			hd = 0
		}
		if hd < 18 && math.Abs(c.L-h.L) < 0.12 && math.Abs(c.S-h.S) < 0.25 {
			return false
		}
	}
	return true
}

func relation(off float64) string {
	a := math.Abs(off)
	switch {
	case a <= 60:
		return "analogous to"
	case a >= 120:
		return "complement of"
	}
	return "triad of"
}
