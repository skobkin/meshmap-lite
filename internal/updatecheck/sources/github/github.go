// Package github implements updatecheck.Source for github.com.
//
// It is a stub for the MVP: the package compiles and is registered with
// the Manager, but FetchReleases returns an error so the cache stays
// empty. The Meshtastic firmware checker (follow-up work) will fill in
// the JSON DTO and request handling.
package github

import (
	"context"
	"errors"
	"strings"

	"meshmap-lite/internal/updatecheck"
)

const defaultPageURLBase = "https://github.com"

// Source is a github.com release source. MVP stub.
type Source struct {
	name    string
	pageURL string
}

// Options configures a github.com Source.
type Options struct {
	Name       string
	Repository string // e.g. meshtastic/firmware
	BaseURL    string // optional, defaults to https://api.github.com
}

// New constructs a github.com Source. It only validates the metadata
// needed to compute ReleasesPageURL; it does not perform any network I/O.
func New(opts Options) (*Source, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return nil, errors.New("github source: name is required")
	}
	repo := strings.TrimSpace(opts.Repository)
	if repo == "" || !strings.Contains(repo, "/") {
		return nil, errors.New("github source: repository must be in owner/name form")
	}

	pageURL := defaultPageURLBase + "/" + repo + "/releases"

	return &Source{name: name, pageURL: pageURL}, nil
}

// Name returns the source's stable identifier.
func (s *Source) Name() string { return s.name }

// ReleasesPageURL returns the user-facing releases page on github.com.
func (s *Source) ReleasesPageURL() string { return s.pageURL }

// FetchReleases is not implemented in the MVP. It always returns an
// error so a registered github source surfaces as a failed fetch in the
// Manager, which means its cache entry stays empty and the
// "failure-doesn't-poison-cache" contract is exercised.
func (s *Source) FetchReleases(_ context.Context) ([]updatecheck.ReleaseInfo, error) {
	return nil, errors.New("github source not implemented")
}
