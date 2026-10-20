package ooxml

import (
	"fmt"
	"strings"
)

// PatchMasterBGXML sets the slide master background to spec ("scheme:<key>"
// or "hex:RRGGBB"), creating or replacing the p:bg element.
func PatchMasterBGXML(xml, spec string) (string, bool) {
	var color string
	switch {
	case strings.HasPrefix(spec, "scheme:"):
		color = fmt.Sprintf(`<a:schemeClr val="%s"/>`, strings.TrimPrefix(spec, "scheme:"))
	case strings.HasPrefix(spec, "hex:"):
		color = fmt.Sprintf(`<a:srgbClr val="%s"/>`, strings.TrimPrefix(spec, "hex:"))
	default:
		return xml, false
	}
	bg := "<p:bg><p:bgRef idx=\"1001\">" + color + "</p:bgRef></p:bg>"
	if doc, _, ok := replaceElem(xml, "p:bg", bg, 0); ok {
		return doc, true
	}
	i := strings.Index(xml, "<p:cSld")
	if i < 0 {
		return xml, false
	}
	gt, _, ok := findTagEnd(xml, i)
	if !ok {
		return xml, false
	}
	return xml[:gt+1] + bg + xml[gt+1:], true
}
