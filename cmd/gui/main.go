package gui

import (
	"defender/pkg/gui/dgraph"
	"defender/pkg/gui/echo"
)

func RunGUI() {
	go dgraph.SetupAndRunDgraph()
	echo.LaunchWebServer()
}
