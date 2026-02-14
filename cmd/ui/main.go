package main

import (
	"lookout/pkg/gui/dgraph"
	"lookout/pkg/gui/echo"
)

func main() {
	go dgraph.SetupAndRunDgraph()
	echo.LaunchWebServer()
}
