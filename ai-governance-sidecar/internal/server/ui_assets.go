package server

import "embed"

//go:embed web/dist/*
var UIAssets embed.FS
