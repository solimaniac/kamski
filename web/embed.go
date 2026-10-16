// Package web contains immutable, offline site templates and assets.
package web

import "embed"

//go:embed templates/*.gohtml assets/*
var Files embed.FS
