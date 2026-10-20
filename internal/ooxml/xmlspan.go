package ooxml

import "strings"

// This file contains minimal, dependency-free helpers for surgical edits of
// OOXML parts. Edits replace whole well-known elements and never re-serialize
// untouched XML, so formatting, prefixes, and unknown attributes survive.

// findTagEnd returns the index of the '>' closing the tag that starts at
// doc[ltIdx] == '<', skipping quoted attribute values.
func findTagEnd(doc string, ltIdx int) (gtIdx int, selfClosing bool, ok bool) {
	inQuote := false
	last := byte(0) // last significant char seen outside quotes
	for i := ltIdx + 1; i < len(doc); i++ {
		c := doc[i]
		if inQuote {
			if c == '"' {
				inQuote = false
			}
			continue
		}
		switch c {
		case '"':
			inQuote = true
		case '>', ' ', '\t', '\n', '\r':
		default:
			last = c
		}
		if c == '>' {
			return i, last == '/', true
		}
	}
	return 0, false, false
}

func isNameBoundary(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '>' || c == '/'
}

// findElem locates the first element `name` (e.g. "a:clrScheme") at or after
// `from` and returns the full span [start, end) including both tags.
func findElem(doc, name string, from int) (int, int, bool) {
	pat := "<" + name
	search := from
	for {
		i := strings.Index(doc[search:], pat)
		if i < 0 {
			return 0, 0, false
		}
		start := search + i
		after := start + len(pat)
		if after >= len(doc) {
			return 0, 0, false
		}
		if !isNameBoundary(doc[after]) {
			search = after
			continue
		}
		gt, self, ok := findTagEnd(doc, start)
		if !ok {
			return 0, 0, false
		}
		if self {
			return start, gt + 1, true
		}
		closePat := "</" + name + ">"
		depth := 1
		j := gt + 1
		for depth > 0 {
			ci := indexClose(doc, name, j)
			if ci < 0 {
				return 0, 0, false
			}
			oi := indexOpen(doc, name, j, ci)
			if oi >= 0 {
				ogt, oself, ok2 := findTagEnd(doc, oi)
				if !ok2 {
					return 0, 0, false
				}
				if !oself {
					depth++
				}
				j = ogt + 1
				continue
			}
			depth--
			j = ci + len(closePat)
			if depth == 0 {
				return start, j, true
			}
		}
		return 0, 0, false
	}
}

func indexClose(doc, name string, from int) int {
	pat := "</" + name
	off := from
	for {
		i := strings.Index(doc[off:], pat)
		if i < 0 {
			return -1
		}
		abs := off + i
		if abs+len(pat) >= len(doc) || doc[abs+len(pat)] != '>' {
			off = abs + 1
			continue
		}
		return abs
	}
}

func indexOpen(doc, name string, from, limit int) int {
	pat := "<" + name
	off := from
	for off < limit {
		i := strings.Index(doc[off:limit], pat)
		if i < 0 {
			return -1
		}
		abs := off + i
		after := abs + len(pat)
		if after < len(doc) && !isNameBoundary(doc[after]) {
			off = after
			continue
		}
		return abs
	}
	return -1
}

// elemInner returns the inner content of the full element string elem
// (which must start at elem[0] == '<' and be exactly one element).
func elemInner(elem, name string) string {
	gt, self, _ := findTagEnd(elem, 0)
	if self {
		return ""
	}
	closeTag := "</" + name + ">"
	return elem[gt+1 : len(elem)-len(closeTag)]
}

// indexAttr finds `attr="` inside a tag string with a word boundary.
func indexAttr(tag, attr string) int {
	pat := attr + `="`
	off := 0
	for {
		i := strings.Index(tag[off:], pat)
		if i < 0 {
			return -1
		}
		abs := off + i
		if abs == 0 || tag[abs-1] == ' ' || tag[abs-1] == '/' {
			return abs
		}
		off = abs + 1
	}
}

// attrValue returns the (still XML-escaped) value of attr within elem.
func attrValue(elem, attr string) string {
	i := indexAttr(elem, attr)
	if i < 0 {
		return ""
	}
	rest := elem[i+len(attr)+2:]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// EscAttr escapes a value for use inside a double-quoted XML attribute.
func EscAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// setAttrInTag sets (or adds) attr="value" on a single tag string.
func setAttrInTag(tag, attr, val string) string {
	if i := indexAttr(tag, attr); i >= 0 {
		vStart := i + len(attr) + 2
		vEnd := vStart + strings.IndexByte(tag[vStart:], '"')
		if vEnd < vStart {
			return tag
		}
		return tag[:vStart] + EscAttr(val) + tag[vEnd:]
	}
	suffix := `="` + EscAttr(val) + `"`
	if strings.HasSuffix(tag, "/>") {
		return tag[:len(tag)-2] + " " + attr + suffix + "/>"
	}
	return tag[:len(tag)-1] + " " + attr + suffix + ">"
}

// replaceElem replaces the first `name` element at/after `from` with newElem
// and returns the updated doc plus the offset just past the replacement.
func replaceElem(doc, name, newElem string, from int) (string, int, bool) {
	s, e, ok := findElem(doc, name, from)
	if !ok {
		return doc, from, false
	}
	return doc[:s] + newElem + doc[e:], s + len(newElem), true
}
