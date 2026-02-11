package main

import (
	"defender/cmd/cli"
	"defender/cmd/gui"
	"os"
)

func main() {
	guiMode := false
	for _, arg := range os.Args[1:] {
		if arg == "-gui" {
			guiMode = true
			break
		}
	}

	if guiMode {
		gui.RunGUI()
	} else {
		cli.RunCLI(os.Args[1:])
	}
}
