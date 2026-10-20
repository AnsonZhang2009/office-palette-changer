package ooxml

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"office-palette-changer/internal/theme"
)

// Schema orders (local names) for the elements ostyle touches. New elements
// are inserted at the position dictated by these sequences, because Word
// enforces child ordering in w:pPr/w:rPr.
var rPrOrder = []string{
	"rStyle", "rFonts", "b", "bCs", "i", "iCs", "caps", "smallCaps", "strike",
	"dstrike", "outline", "shadow", "emboss", "imprint", "noProof", "snapToGrid",
	"vanish", "webHidden", "color", "spacing", "w", "kern", "position", "sz",
	"szCs", "highlight", "u", "effect", "bdr", "shd", "fitText", "vertAlign",
	"rtl", "cs", "em", "lang", "eastAsianLayout", "specVanish", "oMath",
}

var pPrOrder = []string{
	"pStyle", "keepNext", "keepLines", "pageBreakBefore", "framePr", "widowControl",
	"numPr", "suppressLineNumbers", "pBdr", "shd", "tabs", "suppressAutoHyphens",
	"kinsoku", "wordWrap", "overflowPunct", "topLinePunct", "autoSpaceDE",
	"autoSpaceDN", "bidi", "adjustRightInd", "snapToGrid", "spacing", "ind",
	"contextualSpacing", "mirrorIndents", "suppressOverlap", "jc", "textDirection",
	"textAlignment", "textboxTightWrap", "outlineLvl", "divId", "cnfStyle", "rPr",
	"sectPr", "pPrChange",
}

var styleOrder = []string{
	"name", "aliases", "basedOn", "next", "link", "autoRedefine", "hidden",
	"uiPriority", "semiHidden", "unhideWhenUsed", "qFormat", "locked", "personal",
	"personalCompose", "personalReply", "rsid", "pPr", "rPr", "tblPr", "trPr",
	"tcPr", "tblStylePr",
}

var docDefaultsOrder = []string{"rPrDefault", "pPrDefault"}

func majorRFonts() string {
	return `<w:rFonts w:asciiTheme="majorHAnsi" w:eastAsiaTheme="majorEastAsia" w:hAnsiTheme="majorHAnsi" w:cstheme="minorBidi"/>`
}

func minorRFonts() string {
	return `<w:rFonts w:asciiTheme="minorHAnsi" w:eastAsiaTheme="minorEastAsia" w:hAnsiTheme="minorHAnsi" w:cstheme="minorBidi"/>`
}

// PatchWordStylesXML retargets default/Normal/heading fonts to the theme fonts
// and applies @word heading overrides (size, bold, italic, color, spacing) to
// the built-in Heading1-6 styles, creating any that are missing.
func PatchWordStylesXML(xml string, t *theme.Theme) (string, error) {
	if !strings.Contains(xml, "<w:styles") {
		return "", fmt.Errorf("not a Word styles part (missing w:styles root)")
	}
	doc := patchDocDefaultsFonts(xml)

	created := map[int]bool{}
	from := 0
	for {
		s, e, ok := findElem(doc, "w:style", from)
		if !ok {
			break
		}
		block := doc[s:e]
		id := attrValue(block, "w:styleId")
		lvl := headingStyleLevel(id)
		var newBlock string
		switch {
		case id == "Normal":
			newBlock = setStyleRFonts(block, minorRFonts())
		case lvl > 0:
			created[lvl] = true
			newBlock = patchHeadingStyle(block, lvl, t)
		default:
			from = e
			continue
		}
		doc = doc[:s] + newBlock + doc[e:]
		from = s + len(newBlock)
	}

	for lvl := 1; lvl <= 6; lvl++ {
		if t.WordHeadings[lvl] != nil && !created[lvl] {
			doc = insertBeforeClose(doc, "w:styles", newHeadingStyle(lvl, t))
		}
	}
	return doc, nil
}

func headingStyleLevel(id string) int {
	for lvl := 1; lvl <= 6; lvl++ {
		if id == "Heading"+strconv.Itoa(lvl) {
			return lvl
		}
	}
	return 0
}

func patchDocDefaultsFonts(doc string) string {
	s, e, ok := findElem(doc, "w:docDefaults", 0)
	if !ok {
		return doc
	}
	inner := doc[s:e]
	inner = transformContainer(inner, "w:rPrDefault", docDefaultsOrder, func(rd string) string {
		return transformContainer(rd, "w:rPr", nil, func(rc string) string {
			return setChildDirect(rc, "w:rFonts", minorRFonts(), rPrOrder)
		})
	})
	return doc[:s] + inner + doc[e:]
}

// setStyleRFonts points the style's run fonts at the theme fonts. Heading
// styles get the major (heading) font; everything else the minor (body) font.
func setStyleRFonts(block, fonts string) string {
	return transformContainer(block, "w:rPr", styleOrder, func(rc string) string {
		return setChildDirect(rc, "w:rFonts", fonts, rPrOrder)
	})
}

func patchHeadingStyle(block string, lvl int, t *theme.Theme) string {
	hs := t.WordHeadings[lvl]
	if hs == nil {
		return setStyleRFonts(block, majorRFonts())
	}
	nb := setStyleRFonts(block, majorRFonts())
	if hs.SpaceBeforePt != nil || hs.SpaceAfterPt != nil {
		nb = transformContainer(nb, "w:pPr", styleOrder, func(pc string) string {
			return withSpacing(pc, hs)
		})
	}
	return transformContainer(nb, "w:rPr", styleOrder, func(rc string) string {
		rc = setChildDirect(rc, "w:rFonts", majorRFonts(), rPrOrder)
		if hs.Bold != nil {
			rc = setChildDirect(rc, "w:b", boolElem("w:b", *hs.Bold), rPrOrder)
		}
		if hs.Italic != nil {
			rc = setChildDirect(rc, "w:i", boolElem("w:i", *hs.Italic), rPrOrder)
		}
		if hs.Color != "" {
			rc = setChildDirect(rc, "w:color", fmt.Sprintf(`<w:color w:val="%s"/>`, hs.Color), rPrOrder)
		}
		if hs.SizePt != nil {
			hp := halfPoints(*hs.SizePt)
			rc = setChildDirect(rc, "w:sz", fmt.Sprintf(`<w:sz w:val="%s"/>`, hp), rPrOrder)
			rc = setChildDirect(rc, "w:szCs", fmt.Sprintf(`<w:szCs w:val="%s"/>`, hp), rPrOrder)
		}
		return rc
	})
}

// withSpacing merges before/after overrides into the style's w:spacing,
// preserving line/lineRule when they exist.
func withSpacing(pPrContent string, hs *theme.HeadingStyle) string {
	before, after, line, lineRule := "", "", "", ""
	if s, e, ok := findElem(pPrContent, "w:spacing", 0); ok {
		el := pPrContent[s:e]
		before = attrValue(el, "w:before")
		after = attrValue(el, "w:after")
		line = attrValue(el, "w:line")
		lineRule = attrValue(el, "w:lineRule")
	}
	if hs.SpaceBeforePt != nil {
		before = twentieths(*hs.SpaceBeforePt)
	}
	if hs.SpaceAfterPt != nil {
		after = twentieths(*hs.SpaceAfterPt)
	}
	var b strings.Builder
	b.WriteString("<w:spacing")
	if before != "" {
		fmt.Fprintf(&b, ` w:before="%s"`, before)
	}
	if after != "" {
		fmt.Fprintf(&b, ` w:after="%s"`, after)
	}
	if line != "" {
		fmt.Fprintf(&b, ` w:line="%s"`, line)
	}
	if lineRule != "" {
		fmt.Fprintf(&b, ` w:lineRule="%s"`, lineRule)
	}
	b.WriteString("/>")
	return setChildDirect(pPrContent, "w:spacing", b.String(), pPrOrder)
}

func newHeadingStyle(lvl int, t *theme.Theme) string {
	hs := t.WordHeadings[lvl]
	var pPr, rPr strings.Builder

	pPr.WriteString(`<w:keepNext/><w:keepLines/>`)
	if hs.SpaceBeforePt != nil || hs.SpaceAfterPt != nil {
		pPr.WriteString(`<w:spacing`)
		if hs.SpaceBeforePt != nil {
			fmt.Fprintf(&pPr, ` w:before="%s"`, twentieths(*hs.SpaceBeforePt))
		}
		if hs.SpaceAfterPt != nil {
			fmt.Fprintf(&pPr, ` w:after="%s"`, twentieths(*hs.SpaceAfterPt))
		}
		pPr.WriteString(`/>`)
	}
	fmt.Fprintf(&pPr, `<w:outlineLvl w:val="%d"/>`, lvl-1)

	rPr.WriteString(majorRFonts())
	if hs.Bold != nil {
		rPr.WriteString(boolElem("w:b", *hs.Bold))
	}
	if hs.Italic != nil {
		rPr.WriteString(boolElem("w:i", *hs.Italic))
	}
	if hs.Color != "" {
		fmt.Fprintf(&rPr, `<w:color w:val="%s"/>`, hs.Color)
	}
	if hs.SizePt != nil {
		hp := halfPoints(*hs.SizePt)
		fmt.Fprintf(&rPr, `<w:sz w:val="%s"/><w:szCs w:val="%s"/>`, hp, hp)
	}

	return fmt.Sprintf(
		`<w:style w:type="paragraph" w:styleId="Heading%d"><w:name w:val="heading %d"/>`+
			`<w:basedOn w:val="Normal"/><w:next w:val="Normal"/><w:uiPriority w:val="9"/><w:qFormat/>`+
			`<w:pPr>%s</w:pPr><w:rPr>%s</w:rPr></w:style>`,
		lvl, lvl, pPr.String(), rPr.String())
}

func boolElem(name string, on bool) string {
	if on {
		return "<" + name + "/>"
	}
	return fmt.Sprintf(`<%s w:val="0"/>`, name)
}

func halfPoints(pt float64) string {
	return strconv.Itoa(int(math.Round(pt * 2)))
}

func twentieths(pt float64) string {
	return strconv.Itoa(int(math.Round(pt * 20)))
}

// transformContainer applies fn to the inner content of the first `name`
// element inside content, preserving the open tag's attributes. If the
// element is missing it is created (with fn("") as content) and inserted
// among its siblings at the position given by order; a nil order appends at
// the end.
func transformContainer(content, name string, order []string, fn func(string) string) string {
	if s, e, ok := findElem(content, name, 0); ok {
		el := content[s:e]
		gt, self, ok2 := findTagEnd(el, 0)
		if !ok2 {
			return content
		}
		open := el[:gt+1]
		if self {
			open = strings.TrimSuffix(open, "/>") + ">"
		}
		newElem := open + fn(elemInner(el, name)) + "</" + name + ">"
		return content[:s] + newElem + content[e:]
	}
	newElem := "<" + name + ">" + fn("") + "</" + name + ">"
	ins := insertOffset(content, name, order)
	return content[:ins] + newElem + content[ins:]
}

// setChildDirect replaces or inserts `newElem` (a w:... element) inside the
// inner-content string `content`, positioned per order.
func setChildDirect(content, name, newElem string, order []string) string {
	if s, e, ok := findElem(content, name, 0); ok {
		return content[:s] + newElem + content[e:]
	}
	ins := insertOffset(content, name, order)
	return content[:ins] + newElem + content[ins:]
}

// insertOffset returns the byte offset in content at which an element with
// the given (prefixed) name should be inserted, relative to the sibling
// elements named in order.
func insertOffset(content, name string, order []string) int {
	if order == nil {
		return len(content)
	}
	local := name
	if i := strings.Index(name, ":"); i >= 0 {
		local = name[i+1:]
	}
	idx := -1
	for i, n := range order {
		if n == local {
			idx = i
			break
		}
	}
	if idx < 0 {
		return len(content)
	}
	prefix := ""
	if i := strings.Index(name, ":"); i >= 0 {
		prefix = name[:i+1]
	}
	lastEnd := -1
	for i := 0; i < idx; i++ {
		if _, e, ok := findElem(content, prefix+order[i], 0); ok && e > lastEnd {
			lastEnd = e
		}
	}
	if lastEnd >= 0 {
		return lastEnd
	}
	for i := idx + 1; i < len(order); i++ {
		if s, _, ok := findElem(content, prefix+order[i], 0); ok {
			return s
		}
	}
	return len(content)
}

func insertBeforeClose(doc, root, block string) string {
	i := strings.LastIndex(doc, "</"+root+">")
	if i < 0 {
		return doc
	}
	return doc[:i] + block + doc[i:]
}
