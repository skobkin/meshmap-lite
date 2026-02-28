package frontend

import "io/fs"

// Options configures frontend asset serving.
type Options struct {
	AssetsFS         fs.FS
	DistPath         string
	MissingBuildHint string
}
