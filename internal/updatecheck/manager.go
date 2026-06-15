package updatecheck

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

const (
	// defaultInterval is the fallback refresh interval when none is set
	// on the Manager.
	defaultInterval = 12 * time.Hour
	// defaultRequestTimeout is the fallback per-request timeout.
	defaultRequestTimeout = 15 * time.Second
	// defaultChannelBuffer is the buffer size for subscribe channels. A
	// slow consumer should not block the Manager's fetch goroutine.
	defaultChannelBuffer = 4
)

type registeredSource struct {
	spec   SourceSpec
	source Source
}

// Manager owns the registered sources, the in-memory cache, the fetch
// ticker, and the pub-sub fan-out. It is the only writer to Cache; HTTP
// handlers are readers.
type Manager struct {
	sources  map[string]*registeredSource
	cache    *Cache
	interval time.Duration
	timeout  time.Duration
	client   *http.Client
	logger   *slog.Logger
	now      func() time.Time

	mu          sync.RWMutex
	orderedKeys []string // stable iteration order for Names()

	subMu sync.RWMutex
	subs  map[string][]chan UpdateSnapshot
	all   []chan NamedSnapshot
}

// Options configures a Manager at construction time. Zero values fall back
// to sensible defaults.
type Options struct {
	Interval time.Duration
	Timeout  time.Duration
	Client   *http.Client
	Logger   *slog.Logger
	// Now is injectable for tests. Defaults to time.Now.
	Now func() time.Time
}

// NewManager constructs a Manager with the given options. Sources are
// registered later via Register.
func NewManager(opts Options) *Manager {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: defaultRequestTimeout}
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultRequestTimeout
	}
	interval := opts.Interval
	if interval <= 0 {
		interval = defaultInterval
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	return &Manager{
		sources:  make(map[string]*registeredSource),
		cache:    NewCache(),
		interval: interval,
		timeout:  timeout,
		client:   client,
		logger:   logger,
		now:      now,
		subs:     make(map[string][]chan UpdateSnapshot),
	}
}

// Register adds a source to the Manager. The source's Name() must be
// unique; attempting to register a duplicate returns an error.
func (m *Manager) Register(spec SourceSpec) error {
	if spec.Source == nil {
		return fmt.Errorf("updatecheck: source %q has nil Source", spec.Name)
	}
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return fmt.Errorf("updatecheck: source name is required")
	}
	if name != spec.Source.Name() {
		return fmt.Errorf("updatecheck: spec name %q does not match Source.Name() %q", name, spec.Source.Name())
	}

	m.mu.Lock()
	if _, exists := m.sources[name]; exists {
		m.mu.Unlock()

		return fmt.Errorf("updatecheck: source %q already registered", name)
	}
	m.sources[name] = &registeredSource{spec: spec, source: spec.Source}
	m.orderedKeys = append(m.orderedKeys, name)
	m.mu.Unlock()

	return nil
}

// Names returns the registered source names in registration order.
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]string, len(m.orderedKeys))
	copy(out, m.orderedKeys)

	return out
}

// Labels returns a map of source name to user-facing label.
func (m *Manager) Labels() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]string, len(m.sources))
	for _, rs := range m.sources {
		out[rs.spec.Name] = rs.spec.Label
	}

	return out
}

// Snapshot returns the cached snapshot for a source, if any.
func (m *Manager) Snapshot(name string) (UpdateSnapshot, bool) {
	return m.cache.Get(name)
}

// HasSource reports whether the given source is registered.
func (m *Manager) HasSource(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.sources[name]

	return ok
}

// CurrentVersion returns the registered current version for a source.
// The second return value is false when the source is not registered.
func (m *Manager) CurrentVersion(name string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rs, ok := m.sources[name]
	if !ok {
		return "", false
	}

	return rs.spec.CurrentVersion, true
}

// SeedSnapshot stores a pre-built snapshot in the cache, fans it out to
// subscribers, and is intended for tests and warm-start paths. Production
// code should let Start() populate the cache via the Source adapters.
func (m *Manager) SeedSnapshot(name string, snap UpdateSnapshot) {
	m.cache.Set(name, snap)
	m.publish(name, snap)
}

// SnapshotSource returns the underlying Source adapter for a registered
// name. It exists so HTTP handlers can surface platform-supplied URLs
// (e.g. the "View all releases" link) without leaking the adapter type
// through the Manager's public API.
func (m *Manager) SnapshotSource(name string) Source {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rs, ok := m.sources[name]
	if !ok {
		return nil
	}

	return rs.source
}

// Start launches one fetch goroutine per registered source. The first fetch
// runs immediately so the cache is warm for the first request after boot.
// Each goroutine exits when ctx is cancelled.
func (m *Manager) Start(ctx context.Context) {
	m.mu.RLock()
	keys := append([]string(nil), m.orderedKeys...)
	m.mu.RUnlock()

	for _, name := range keys {
		m.mu.RLock()
		rs, ok := m.sources[name]
		m.mu.RUnlock()
		if !ok {
			continue
		}
		go m.run(ctx, name, rs)
	}
}

// run is the per-source loop. It performs one immediate fetch and then
// ticks on the Manager interval.
func (m *Manager) run(ctx context.Context, name string, rs *registeredSource) {
	m.logger.Info("update check source started",
		"source", name,
		"label", rs.spec.Label,
		"interval", m.interval.String(),
		"current_version", rs.spec.CurrentVersion,
	)

	if err := m.fetchAndPublish(ctx, name, rs); err != nil {
		m.logger.Warn("update check failed", "source", name, "error", err)
	}

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("update check source stopped", "source", name)

			return
		case <-ticker.C:
			if err := m.fetchAndPublish(ctx, name, rs); err != nil {
				m.logger.Warn("update check failed", "source", name, "error", err)
			}
		}
	}
}

// fetchAndPublish is the unit of work for a single fetch attempt.
func (m *Manager) fetchAndPublish(ctx context.Context, name string, rs *registeredSource) error {
	reqCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	releases, err := rs.source.FetchReleases(reqCtx)
	if err != nil {
		return fmt.Errorf("fetch releases: %w", err)
	}
	if len(releases) == 0 {
		return fmt.Errorf("release feed for source %q is empty", name)
	}

	snap := m.buildSnapshot(name, rs.spec.CurrentVersion, releases)

	m.cache.Set(name, snap)
	m.publish(name, snap)

	m.logger.Info("update check completed",
		"source", name,
		"latest_version", snap.Latest.Version,
		"update_available", snap.UpdateAvailable,
		"release_count", len(snap.Releases),
	)

	return nil
}

// buildSnapshot composes an UpdateSnapshot from raw release data. It is
// the single place that decides what counts as "newer" and how to hash a
// snapshot — both of which are Manager responsibilities, not Source ones.
func (m *Manager) buildSnapshot(name, currentVersion string, releases []ReleaseInfo) UpdateSnapshot {
	latest := releases[0]
	snap := UpdateSnapshot{
		Source:         name,
		CurrentVersion: currentVersion,
		Latest:         latest,
		Releases:       releases,
		CheckedAt:      m.now(),
		SourceHash:     computeSourceHash(latest),
	}
	snap.UpdateAvailable = computeUpdateAvailable(currentVersion, latest.Version)

	return snap
}

// computeSourceHash returns a short stable hash of the latest release.
// The frontend uses it to detect "something changed since you last looked".
func computeSourceHash(latest ReleaseInfo) string {
	h := sha256.New()
	h.Write([]byte(latest.Version))
	h.Write([]byte{'\n'})
	if !latest.PublishedAt.IsZero() {
		h.Write([]byte(latest.PublishedAt.UTC().Format(time.RFC3339)))
	}

	sum := h.Sum(nil)

	return hex.EncodeToString(sum[:8])
}

// computeUpdateAvailable implements the same "is latest newer than current"
// rule as meshgo/internal/app/update_checker.go: a non-semver "current"
// (e.g. "dev") is treated as older; a non-semver "latest" means we have
// no opinion.
func computeUpdateAvailable(currentVersion, latestVersion string) bool {
	current := normalizeSemver(currentVersion)
	latest := normalizeSemver(latestVersion)

	if !semver.IsValid(latest) {
		return false
	}
	if !semver.IsValid(current) {
		return true
	}

	return semver.Compare(current, latest) < 0
}

// normalizeSemver ensures the version has a leading "v" as required by
// golang.org/x/mod/semver. Empty input stays empty.
func normalizeSemver(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "v") {
		return "v" + trimmed
	}

	return trimmed
}

// Subscribe returns a buffered channel that receives every new snapshot
// for the named source. The returned function unsubscribes and closes the
// channel. A subscription for an unknown source still yields a working
// channel — it will simply never receive a value, which keeps callers
// from racing against Register at startup.
func (m *Manager) Subscribe(name string) (<-chan UpdateSnapshot, func()) {
	ch := make(chan UpdateSnapshot, defaultChannelBuffer)

	m.subMu.Lock()
	m.subs[name] = append(m.subs[name], ch)
	m.subMu.Unlock()

	unsubscribe := func() {
		m.subMu.Lock()
		defer m.subMu.Unlock()

		list := m.subs[name]
		for i, c := range list {
			if c == ch {
				m.subs[name] = append(list[:i], list[i+1:]...)

				break
			}
		}
		close(ch)
	}

	return ch, unsubscribe
}

// SubscribeAll returns a channel that receives a NamedSnapshot for every
// successful fetch from any source. Same unsubscribe contract as Subscribe.
func (m *Manager) SubscribeAll() (<-chan NamedSnapshot, func()) {
	ch := make(chan NamedSnapshot, defaultChannelBuffer)

	m.subMu.Lock()
	m.all = append(m.all, ch)
	m.subMu.Unlock()

	unsubscribe := func() {
		m.subMu.Lock()
		defer m.subMu.Unlock()

		for i, c := range m.all {
			if c == ch {
				m.all = append(m.all[:i], m.all[i+1:]...)

				break
			}
		}
		close(ch)
	}

	return ch, unsubscribe
}

// publish fans a snapshot out to subscribers. Sends are non-blocking:
// a full channel is dropped and the subscriber is pruned, mirroring the
// websocket hub's "drop failing subs" behavior.
func (m *Manager) publish(name string, snap UpdateSnapshot) {
	m.subMu.RLock()
	defer m.subMu.RUnlock()

	drop := func(ch chan UpdateSnapshot) bool {
		select {
		case ch <- snap:
			return false
		default:
			return true
		}
	}

	kept := m.subs[name][:0]
	for _, ch := range m.subs[name] {
		if drop(ch) {
			// Subscriber is slow or dead — let it close via its own unsubscribe.
			continue
		}
		kept = append(kept, ch)
	}
	m.subs[name] = kept

	named := NamedSnapshot{Name: name, Snapshot: snap}
	keptAll := m.all[:0]
	for _, ch := range m.all {
		select {
		case ch <- named:
			keptAll = append(keptAll, ch)
		default:
		}
	}
	m.all = keptAll
}
