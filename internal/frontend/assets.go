package frontend

import "io/fs"

func resolveAssets(opts Options) (fs.FS, bool) {
	if opts.AssetsFS != nil {
		return opts.AssetsFS, true
	}

	return resolveEmbeddedAssets()
}
