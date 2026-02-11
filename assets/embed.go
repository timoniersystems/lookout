package assets

import (
	"embed"
)

//go:embed templates/* static/css/* static/javascript/*
var Assets embed.FS
