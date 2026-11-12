package web

import (
	"embed"
	"io/fs"
)

// Files contains the dashboard templates and static assets.
//
//go:embed templates/*.html static/css/*.css static/js/*.js
var Files embed.FS

// StaticFiles returns the static directory as the root of an fs.FS.
func StaticFiles() fs.FS {
	static, err := fs.Sub(Files, "static")
	if err != nil {
		panic(err)
	}
	return static
}
