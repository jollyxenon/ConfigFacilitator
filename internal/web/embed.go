// Package web serves the embedded cfgfc Web UI and its local HTTP API.
package web

import "embed"

// staticFS embeds the zero-dependency frontend into the single binary.
//
//go:embed all:static
var staticFS embed.FS
