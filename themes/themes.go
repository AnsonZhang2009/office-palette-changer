// Package themes embeds the built-in ostyle themes shipped with the binary.
package themes

import "embed"

//go:embed *.css
var FS embed.FS
