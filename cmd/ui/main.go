package main

import (
	"lookout/pkg/ui/dgraph"
	"lookout/pkg/ui/echo"
)

func main() {
	go dgraph.SetupAndRunDgraph()
	echo.LaunchWebServer()
}
