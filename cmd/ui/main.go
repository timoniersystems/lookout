package main

import (
	"timonier.systems/lookout/pkg/ui/dgraph"
	"timonier.systems/lookout/pkg/ui/echo"
)

func main() {
	go dgraph.SetupAndRunDgraph()
	echo.LaunchWebServer()
}
