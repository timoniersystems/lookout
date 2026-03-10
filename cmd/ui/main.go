package main

import (
	"github.com/timoniersystems/lookout/pkg/ui/dgraph"
	"github.com/timoniersystems/lookout/pkg/ui/echo"
)

func main() {
	go dgraph.SetupAndRunDgraph()
	echo.LaunchWebServer()
}
