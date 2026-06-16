// Package forgejo implements updatecheck.Source for Forgejo instances
// (https://forgejo.org). It is a direct port of the JSON DTO and
// request handling from meshgo/internal/app/update_checker.go, trimmed
// down to the Source interface so it slots into the Manager.
package forgejo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"meshmap-lite/internal/updatecheck"
)

// Source is a Forgejo release source.
type Source struct {
	name        string
	apiURL      string
	pageURL     string
	limit       int
	preReleases bool
	httpClient  *http.Client
}

// Options configures a Forgejo Source.
type Options struct {
	Name       string
	BaseURL    string // e.g. https://git.skobk.in
	Repository string // e.g. skobkin/meshmap-lite
	Limit      int
	// PreReleases, when true, includes pre-release (alpha/beta/rc) tags
	// alongside stable releases. Drafts are always excluded regardless.
	PreReleases bool
	HTTPClient  *http.Client
}

// New constructs a Forgejo Source. It returns an error if the required
// fields are missing or the URL cannot be parsed.
func New(opts Options) (*Source, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return nil, fmt.Errorf("forgejo source: name is required")
	}
	base := strings.TrimRight(strings.TrimSpace(opts.BaseURL), "/")
	if base == "" {
		return nil, fmt.Errorf("forgejo source %q: base_url is required", name)
	}
	repo := strings.TrimSpace(opts.Repository)
	if repo == "" || !strings.Contains(repo, "/") {
		return nil, fmt.Errorf("forgejo source %q: repository must be in owner/name form", name)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 15
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	apiURL := fmt.Sprintf("%s/api/v1/repos/%s/releases?draft=false&pre-release=%t&limit=%d",
		base, repo, opts.PreReleases, limit)
	pageURL := fmt.Sprintf("%s/%s/releases", base, repo)

	return &Source{
		name:        name,
		apiURL:      apiURL,
		pageURL:     pageURL,
		limit:       limit,
		preReleases: opts.PreReleases,
		httpClient:  client,
	}, nil
}

// Name returns the source's stable identifier.
func (s *Source) Name() string { return s.name }

// ReleasesPageURL returns the user-facing releases page on the Forgejo
// instance.
func (s *Source) ReleasesPageURL() string { return s.pageURL }

// APIURL returns the API endpoint the source queries. It is exposed for
// tests and diagnostics.
func (s *Source) APIURL() string { return s.apiURL }

// forgejoRelease mirrors the upstream JSON DTO.
type forgejoRelease struct {
	TagName     string    `json:"tag_name"`
	Body        string    `json:"body"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
}

// FetchReleases performs a single GET against the configured API endpoint
// and returns the parsed releases ordered newest-first (the API returns
// them newest-first when the limit cap is not exceeded).
func (s *Source) FetchReleases(ctx context.Context) ([]updatecheck.ReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("forgejo source %q: build request: %w", s.name, err)
	}
	req.Header.Set("Accept", "application/json")

	// #nosec G107 -- endpoint is built from operator-provided config.
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forgejo source %q: request: %w", s.name, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		trimmed := strings.TrimSpace(string(body))
		if trimmed == "" {
			return nil, fmt.Errorf("forgejo source %q: unexpected status %d", s.name, resp.StatusCode)
		}

		return nil, fmt.Errorf("forgejo source %q: unexpected status %d: %s", s.name, resp.StatusCode, trimmed)
	}

	var payload []forgejoRelease
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("forgejo source %q: decode: %w", s.name, err)
	}

	out := make([]updatecheck.ReleaseInfo, 0, len(payload))
	for _, item := range payload {
		version := strings.TrimSpace(item.TagName)
		if version == "" {
			continue
		}
		out = append(out, updatecheck.ReleaseInfo{
			Version:     version,
			Body:        strings.TrimSpace(item.Body),
			HTMLURL:     strings.TrimSpace(item.HTMLURL),
			PublishedAt: item.PublishedAt,
		})
	}

	return out, nil
}

// LimitForURL is a small helper kept here for symmetry with other
// platform adapters; it isn't used internally but may help future tests.
func LimitForURL(rawURL string) (int, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return 0, err
	}

	v := u.Query().Get("limit")
	if v == "" {
		return 0, nil
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}

	return n, nil
}
