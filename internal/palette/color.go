// Package palette derives a complete 12-slot Office color scheme from a
// handful of seed colors. All math is plain HSL plus WCAG luminance, so the
// output is deterministic and dependency-free.
package palette

import (
	"fmt"
	"math"
	"strconv"

	"office-palette-changer/internal/theme"
)

// RGB holds channels in [0,1].
type RGB struct{ R, G, B float64 }

// HSL holds hue in degrees [0,360) and saturation/lightness in [0,1].
type HSL struct{ H, S, L float64 }

// ParseHex accepts #RGB / #RRGGBB (hash optional, any case).
func ParseHex(s string) (RGB, error) {
	h, err := theme.NormalizeHex(s)
	if err != nil {
		return RGB{}, err
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return RGB{}, err
	}
	return RGB{
		R: float64(v>>16&0xFF) / 255,
		G: float64(v>>8&0xFF) / 255,
		B: float64(v&0xFF) / 255,
	}, nil
}

// Hex renders as uppercase RRGGBB without a hash.
func (c RGB) Hex() string {
	to := func(v float64) int { return int(math.Round(clamp(v, 0, 1) * 255)) }
	return fmt.Sprintf("%02X%02X%02X", to(c.R), to(c.G), to(c.B))
}

func (c RGB) HSL() HSL {
	mx := math.Max(c.R, math.Max(c.G, c.B))
	mn := math.Min(c.R, math.Min(c.G, c.B))
	l := (mx + mn) / 2
	if mx == mn {
		return HSL{0, 0, l}
	}
	d := mx - mn
	var s float64
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	var h float64
	switch mx {
	case c.R:
		h = (c.G - c.B) / d
		if c.G < c.B {
			h += 6
		}
	case c.G:
		h = (c.B-c.R)/d + 2
	default:
		h = (c.R-c.G)/d + 4
	}
	return HSL{wrapHue(h * 60), s, l}
}

func (h HSL) RGB() RGB {
	s, l := clamp(h.S, 0, 1), clamp(h.L, 0, 1)
	if s == 0 {
		return RGB{l, l, l}
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	hk := wrapHue(h.H) / 360
	return RGB{hue2rgb(p, q, hk+1.0/3), hue2rgb(p, q, hk), hue2rgb(p, q, hk-1.0/3)}
}

func hue2rgb(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	}
	return p
}

func (h HSL) Hex() string { return h.RGB().Hex() }

// Luminance is WCAG 2.x relative luminance.
func Luminance(c RGB) float64 {
	lin := func(v float64) float64 {
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// Contrast is the WCAG contrast ratio (>= 1).
func Contrast(a, b RGB) float64 {
	la, lb := Luminance(a), Luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// HueDistance is the shortest angular distance between two hues (0..180).
func HueDistance(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	if d > 180 {
		d = 360 - d
	}
	return d
}

func wrapHue(h float64) float64 {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	return h
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
