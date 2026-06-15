// Package github implements updatecheck.Source for github.com.
//
// It is a stub for future GitHub support. Application configuration rejects
// github sources until FetchReleases is implemented.
package github

import (
	"context"
	"errors"
	"strings"

	"meshmap-lite/internal/updatecheck"
)

const defaultPageURLBase = "https://github.com"

// Source is a future github.com release source.
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

// FetchReleases is not implemented.
func (s *Source) FetchReleases(_ context.Context) ([]updatecheck.ReleaseInfo, error) {
	return nil, errors.New("github source not implemented")
}
