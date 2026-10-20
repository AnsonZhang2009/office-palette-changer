// Command ostyle is a lightweight palette/font/heading stylizer for
// Microsoft Word and PowerPoint documents on macOS.
package main

import (
	"os"

	"office-palette-changer/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
