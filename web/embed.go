package web

import "embed"

// Files contains the templates and static assets needed by the single binary.
//
//go:embed templates/*.html static/*
var Files embed.FS
