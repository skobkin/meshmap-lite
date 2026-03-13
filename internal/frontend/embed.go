package frontend

import (
	"embed"
	"io/fs"
)

//go:embed all:assets/dist
var embeddedFiles embed.FS

func resolveEmbeddedAssets() (fs.FS, bool) {
	root, err := fs.Sub(embeddedFiles, "assets/dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return nil, false
	}

	return root, true
}
