package frontend

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

func spaFileServer(root fs.FS) http.Handler {
	fsrv := http.FileServer(http.FS(root))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "" {
			http.ServeFileFS(w, r, root, "index.html")

			return
		}

		trimmedPath := strings.TrimPrefix(r.URL.Path, "/")
		if _, err := fs.Stat(root, trimmedPath); err == nil {
			fsrv.ServeHTTP(w, r)

			return
		}

		if filepath.Ext(trimmedPath) != "" {
			http.NotFound(w, r)

			return
		}

		http.ServeFileFS(w, r, root, "index.html")
	})
}
