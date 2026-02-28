package frontend

import (
	"io/fs"
	"os"
)

func resolveAssets(opts Options) (fs.FS, bool) {
	if opts.AssetsFS != nil {
		return opts.AssetsFS, true
	}
	if opts.DistPath == "" {
		return nil, false
	}

	st, err := os.Stat(opts.DistPath)
	if err != nil || !st.IsDir() {
		return nil, false
	}

	return os.DirFS(opts.DistPath), true
}
