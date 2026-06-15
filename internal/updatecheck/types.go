// Package updatecheck provides a small, library-extensible multi-source
// release-checker. Sources are registered at startup; the Manager fetches
// their releases on a configurable interval, caches the latest successful
// snapshot in memory, and exposes synchronous and pub-sub accessors.
//
// The package is intentionally free of meshmap-lite UI concerns: per-source
// user-facing labels live on the Manager (populated from config), not on
// the Source interface, so per-platform adapters stay pure data fetchers.
package updatecheck

import "time"

// ReleaseInfo describes a single release entry returned by a Source.
type ReleaseInfo struct {
	Version     string
	Body        string
	HTMLURL     string
	PublishedAt time.Time
}

// UpdateSnapshot is what the cache stores for a single Source. It is the
// payload shape returned to HTTP handlers and fanned out to subscribers.
type UpdateSnapshot struct {
	Source          string
	CurrentVersion  string
	Latest          ReleaseInfo
	Releases        []ReleaseInfo
	UpdateAvailable bool
	CheckedAt       time.Time
	SourceHash      string
}

// NamedSnapshot pairs a snapshot with the name of the source it came from.
// It is the element type of the SubscribeAll() channel.
type NamedSnapshot struct {
	Name     string
	Snapshot UpdateSnapshot
}

// SourceSpec is the registration-side description of a source. The Manager
// owns the SourceSpec shape so adapters stay decoupled from config types.
type SourceSpec struct {
	Name           string
	Label          string
	Source         Source
	CurrentVersion string
}
