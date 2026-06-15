package updatecheck

import "context"

// Source is the minimal interface every per-platform release source must
// implement. It is intentionally tiny and library-shaped: no UI concerns,
// no current-version comparison, no caching. Those responsibilities live
// in the Manager.
type Source interface {
	// Name returns the source's stable identifier (e.g. "meshmap-lite").
	// It is used as the cache key and as the lookup value for the
	// ?source= query parameter on the HTTP API.
	Name() string

	// FetchReleases returns releases ordered newest-first. Implementations
	// should be tolerant of upstream errors and return a wrapped error.
	FetchReleases(ctx context.Context) ([]ReleaseInfo, error)

	// ReleasesPageURL returns a user-facing "View all releases" URL,
	// pointing at the upstream platform's web UI (e.g. the Forgejo or
	// GitHub releases page). It is built from the source's own
	// configuration; the frontend does not need to know which platform
	// produced the data.
	ReleasesPageURL() string
}
