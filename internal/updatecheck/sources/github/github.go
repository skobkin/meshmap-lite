// Package github implements updatecheck.Source for GitHub-compatible
// release APIs.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"meshmap-lite/internal/updatecheck"
)

const (
	defaultAPIURL = "https://api.github.com"
	defaultLimit  = 15
	maxLimit      = 100
)

// Source is a GitHub release source.
type Source struct {
	name        string
	apiURL      string
	pageURL     string
	limit       int
	preReleases bool
	httpClient  *http.Client
}

// Options configures a GitHub Source.
type Options struct {
	Name       string
	Repository string // e.g. meshtastic/firmware
	BaseURL    string // optional, defaults to https://api.github.com
	Limit      int
	// PreReleases, when true, includes pre-release (alpha/beta/rc) tags
	// alongside stable releases. Drafts are always skipped regardless.
	PreReleases bool
	HTTPClient  *http.Client
}

// New constructs a GitHub Source. It validates metadata and builds the
// API and human-facing releases URLs; it does not perform network I/O.
func New(opts Options) (*Source, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return nil, fmt.Errorf("github source: name is required")
	}
	repo := strings.TrimSpace(opts.Repository)
	owner, repoName, ok := splitRepository(repo)
	if !ok {
		return nil, fmt.Errorf("github source %q: repository must be in owner/name form", name)
	}

	base := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if base == "" {
		base = defaultAPIURL
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil, fmt.Errorf("github source %q: parse base_url: %w", name, err)
	}
	if baseURL.Scheme == "" || baseURL.Host == "" {
		return nil, fmt.Errorf("github source %q: base_url must be absolute", name)
	}

	limit := clampLimit(opts.Limit)
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	api := *baseURL
	api.Path = strings.TrimRight(api.Path, "/") + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repoName) + "/releases"
	api.RawQuery = fmt.Sprintf("per_page=%d&page=1", limit)

	pageURL := releasesPageURL(*baseURL, owner, repoName)

	return &Source{
		name:        name,
		apiURL:      api.String(),
		pageURL:     pageURL,
		limit:       limit,
		preReleases: opts.PreReleases,
		httpClient:  client,
	}, nil
}

// Name returns the source's stable identifier.
func (s *Source) Name() string { return s.name }

// ReleasesPageURL returns the user-facing releases page.
func (s *Source) ReleasesPageURL() string { return s.pageURL }

// APIURL returns the API endpoint the source queries. It is exposed for
// tests and diagnostics.
func (s *Source) APIURL() string { return s.apiURL }

type githubRelease struct {
	TagName     string    `json:"tag_name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
}

// FetchReleases performs a single GET against the configured API endpoint
// and returns non-draft releases ordered newest-first. Pre-release tags
// are included only when the source was constructed with PreReleases=true.
func (s *Source) FetchReleases(ctx context.Context) ([]updatecheck.ReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github source %q: build request: %w", s.name, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// #nosec G107 -- endpoint is built from operator-provided config.
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github source %q: request: %w", s.name, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		trimmed := strings.TrimSpace(string(body))
		if trimmed == "" {
			return nil, fmt.Errorf("github source %q: unexpected status %d", s.name, resp.StatusCode)
		}

		return nil, fmt.Errorf("github source %q: unexpected status %d: %s", s.name, resp.StatusCode, trimmed)
	}

	var payload []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("github source %q: decode: %w", s.name, err)
	}

	out := make([]updatecheck.ReleaseInfo, 0, len(payload))
	for _, item := range payload {
		version := strings.TrimSpace(item.TagName)
		if version == "" || item.Draft {
			continue
		}
		if !s.preReleases && item.Prerelease {
			continue
		}
		out = append(out, updatecheck.ReleaseInfo{
			Version:     version,
			Body:        strings.TrimSpace(item.Body),
			HTMLURL:     strings.TrimSpace(item.HTMLURL),
			PublishedAt: item.PublishedAt,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].PublishedAt.After(out[j].PublishedAt)
	})

	return out, nil
}

func splitRepository(repo string) (string, string, bool) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	owner := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])

	return owner, name, owner != "" && name != ""
}

func clampLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultLimit
	case limit > maxLimit:
		return maxLimit
	default:
		return limit
	}
}

func releasesPageURL(apiBase url.URL, owner, repo string) string {
	web := apiBase
	if strings.EqualFold(web.Host, "api.github.com") {
		web.Host = "github.com"
		web.Path = ""
	} else {
		web.Path = strings.TrimRight(web.Path, "/")
		web.Path = strings.TrimSuffix(web.Path, "/api/v3")
	}
	web.RawQuery = ""
	web.Fragment = ""
	web.Path = strings.TrimRight(web.Path, "/") + "/" + url.PathEscape(owner) + "/" + url.PathEscape(repo) + "/releases"

	return web.String()
}
