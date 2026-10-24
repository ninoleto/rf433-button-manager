// Package webui contains the complete offline-capable management interface.
package webui

import "embed"

// FS is served from its static subdirectory by the application HTTP server.
//
//go:embed static/*
var FS embed.FS
