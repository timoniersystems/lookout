package assets

import (
	"embed"
)

//go:embed templates/* static/css/* static/javascript/* static/images/*
var Assets embed.FS
