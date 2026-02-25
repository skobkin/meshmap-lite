package frontend

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Handler serves compiled frontend assets with SPA fallback semantics.
func Handler() http.Handler {
	dist := filepath.Join("web", "dist")
	if st, err := os.Stat(dist); err == nil && st.IsDir() {
		return spaFileServer(os.DirFS(dist))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("frontend assets are not built; run `cd web && npm run build`"))
	})
}

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

		// Return 404 for missing file-like requests to avoid serving HTML with JS/CSS URLs.
		if filepath.Ext(trimmedPath) != "" {
			http.NotFound(w, r)

			return
		}
		http.ServeFileFS(w, r, root, "index.html")
	})
}
