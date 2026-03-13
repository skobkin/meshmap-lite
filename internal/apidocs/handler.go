package apidocs

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed assets/*
var assets embed.FS

// Options configures API docs serving.
type Options struct {
	SpecURL string
	Title   string
}

// Handler serves the OpenAPI specification and a lightweight interactive viewer.
func Handler(opts Options) http.Handler {
	if opts.SpecURL == "" {
		opts.SpecURL = "/api/openapi.yaml"
	}
	if opts.Title == "" {
		opts.Title = "MeshMap Lite API"
	}

	assetRoot, err := fs.Sub(assets, "assets")
	if err != nil {
		panic(err)
	}
	specBody, err := fs.ReadFile(assetRoot, "openapi.yaml")
	if err != nil {
		panic(err)
	}

	files := http.StripPrefix("/api/assets/", http.FileServer(http.FS(assetRoot)))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api":
			http.Redirect(w, r, "/api/", http.StatusMovedPermanently)
		case r.URL.Path == "/api/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, indexHTML, opts.Title, opts.Title, opts.SpecURL, opts.SpecURL)
		case r.URL.Path == opts.SpecURL:
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(specBody)
		case strings.HasPrefix(r.URL.Path, "/api/assets/"):
			files.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

const indexHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>%s</title>
    <style>
      :root {
        color-scheme: light;
        --bg: #f6f4ee;
        --panel: #fbfaf7;
        --ink: #14281d;
        --accent: #285943;
        --accent-soft: #d8eadf;
        --line: #c9d8cf;
      }
      * { box-sizing: border-box; }
      body {
        margin: 0;
        font-family: "IBM Plex Sans", "Segoe UI", sans-serif;
        color: var(--ink);
        background:
          radial-gradient(circle at top left, rgba(40, 89, 67, 0.12), transparent 28rem),
          linear-gradient(180deg, #fcfbf8 0%%, var(--bg) 100%%);
      }
      header {
        padding: 1rem 1.25rem;
        border-bottom: 1px solid var(--line);
        background: rgba(251, 250, 247, 0.92);
        backdrop-filter: blur(10px);
      }
      .title {
        display: flex;
        flex-wrap: wrap;
        gap: 0.75rem;
        align-items: baseline;
      }
      .title h1 {
        margin: 0;
        font-size: 1.1rem;
        letter-spacing: 0.04em;
        text-transform: uppercase;
      }
      .title a {
        color: var(--accent);
        text-decoration-thickness: 0.08em;
      }
      main {
        min-height: calc(100vh - 70px);
      }
      rapi-doc {
        height: calc(100vh - 70px);
        width: 100%%;
        --bg: var(--bg);
        --fg: var(--ink);
        --header-bg: var(--panel);
        --header-fg: var(--ink);
        --nav-bg: #18392b;
        --nav-hover-bg: #285943;
        --nav-text-color: #f8fbf9;
        --nav-accent-color: #9bc7ae;
        --primary-color: var(--accent);
        --primary-color-invert: #f7fbf9;
        --schema-bg: var(--panel);
        --code-bg: #122019;
        --code-fg: #edf6f1;
        --border-color: var(--line);
      }
    </style>
    <script type="module" src="/api/assets/rapidoc-min.js"></script>
  </head>
  <body>
    <header>
      <div class="title">
        <h1>%s</h1>
        <a href="%s">Raw OpenAPI YAML</a>
      </div>
    </header>
    <main>
      <rapi-doc
        spec-url="%s"
        render-style="read"
        theme="light"
        allow-try="true"
        allow-authentication="true"
        allow-server-selection="false"
        show-header="false"
        show-info="true"
        use-path-in-nav-bar="true"
        nav-bg-color="#18392b"
        primary-color="#285943"
        sort-tags="true"
        regular-font="IBM Plex Sans, Segoe UI, sans-serif"
        mono-font="IBM Plex Mono, SFMono-Regular, ui-monospace, monospace"
      ></rapi-doc>
    </main>
  </body>
</html>
`
