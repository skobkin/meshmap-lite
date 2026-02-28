package frontend

import "net/http"

// Handler serves compiled frontend assets with SPA fallback semantics.
func Handler(opts Options) http.Handler {
	if assets, ok := resolveAssets(opts); ok {
		return spaFileServer(assets)
	}

	return missingBuildHandler(opts.MissingBuildHint)
}
