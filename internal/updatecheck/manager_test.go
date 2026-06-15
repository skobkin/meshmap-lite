package updatecheck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stubSource is a test double. It records the call count and returns
// either a canned payload or an error depending on the configured hook.
// hook is guarded by a mutex so tests can swap the function while a
// Manager goroutine is in flight (see TestFailedFetchDoesNotPoisonCache).
type stubSource struct {
	name    string
	pageURL string

	mu    sync.RWMutex
	hook  func(ctx context.Context) ([]ReleaseInfo, error)
	calls atomic.Int64
}

// SetHook atomically replaces the function used by FetchReleases.
func (s *stubSource) SetHook(hook func(ctx context.Context) ([]ReleaseInfo, error)) {
	s.mu.Lock()
	s.hook = hook
	s.mu.Unlock()
}

func (s *stubSource) Name() string            { return s.name }
func (s *stubSource) ReleasesPageURL() string { return s.pageURL }
func (s *stubSource) FetchReleases(ctx context.Context) ([]ReleaseInfo, error) {
	s.calls.Add(1)

	s.mu.RLock()
	hook := s.hook
	s.mu.RUnlock()

	return hook(ctx)
}

func newTestManager(t *testing.T, interval time.Duration) *Manager {
	t.Helper()

	return NewManager(Options{
		Interval: interval,
		Timeout:  time.Second,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:      func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
	})
}

func mustRelease(t *testing.T, version, body string, publishedAt time.Time) ReleaseInfo {
	t.Helper()

	return ReleaseInfo{
		Version:     version,
		Body:        body,
		HTMLURL:     "https://example.com/r/" + version,
		PublishedAt: publishedAt,
	}
}

func TestRegisterRejectsDuplicateNames(t *testing.T) {
	m := newTestManager(t, time.Hour)
	s1 := &stubSource{name: "x", pageURL: "https://example.com/x"}
	s2 := &stubSource{name: "x", pageURL: "https://example.com/x"}

	if err := m.Register(SourceSpec{Name: "x", Source: s1, Label: "X", CurrentVersion: "0.1.0"}); err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	if err := m.Register(SourceSpec{Name: "x", Source: s2, Label: "X", CurrentVersion: "0.1.0"}); err == nil {
		t.Fatalf("expected duplicate register to fail")
	}
}

func TestRegisterRejectsNameMismatch(t *testing.T) {
	m := newTestManager(t, time.Hour)
	s := &stubSource{name: "actual", pageURL: "https://example.com/a"}

	if err := m.Register(SourceSpec{Name: "other", Source: s, Label: "O"}); err == nil {
		t.Fatalf("expected name mismatch to fail")
	}
}

func TestNamesAndLabelsReturnRegisteredSources(t *testing.T) {
	m := newTestManager(t, time.Hour)
	if err := m.Register(SourceSpec{Name: "a", Label: "A", Source: &stubSource{name: "a"}}); err != nil {
		t.Fatalf("register a: %v", err)
	}
	if err := m.Register(SourceSpec{Name: "b", Label: "B", Source: &stubSource{name: "b"}}); err != nil {
		t.Fatalf("register b: %v", err)
	}

	names := m.Names()
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Fatalf("unexpected names: %v", names)
	}

	labels := m.Labels()
	if labels["a"] != "A" || labels["b"] != "B" {
		t.Fatalf("unexpected labels: %v", labels)
	}
}

func TestStartPerformsImmediateFetchAndPopulatesCache(t *testing.T) {
	m := newTestManager(t, 25*time.Millisecond)
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	hook := func(context.Context) ([]ReleaseInfo, error) {
		return []ReleaseInfo{
			mustRelease(t, "0.7.0", "newer", now),
			mustRelease(t, "0.6.1", "older", now.Add(-24*time.Hour)),
		}, nil
	}
	src := &stubSource{name: "meshmap-lite", pageURL: "https://example.com/m", hook: hook}

	if err := m.Register(SourceSpec{Name: "meshmap-lite", Source: src, Label: "Map", CurrentVersion: "0.6.0"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if snap, ok := m.Snapshot("meshmap-lite"); ok {
			if snap.Latest.Version != "0.7.0" {
				t.Fatalf("unexpected latest version: %q", snap.Latest.Version)
			}
			if !snap.UpdateAvailable {
				t.Fatalf("expected UpdateAvailable=true for 0.6.0 < 0.7.0")
			}
			if snap.CurrentVersion != "0.6.0" {
				t.Fatalf("unexpected current version: %q", snap.CurrentVersion)
			}
			if snap.SourceHash == "" {
				t.Fatalf("expected non-empty source hash")
			}
			// CheckedAt is set by the Manager from its injected clock.
			fixedNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			if !snap.CheckedAt.Equal(fixedNow) {
				t.Fatalf("unexpected CheckedAt: %v", snap.CheckedAt)
			}

			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected snapshot to populate within 2s")
}

func TestStartRecoversAfterFailedCheck(t *testing.T) {
	m := newTestManager(t, 25*time.Millisecond)
	var calls atomic.Int64
	hook := func(context.Context) ([]ReleaseInfo, error) {
		n := calls.Add(1)
		if n == 1 {
			return nil, errors.New("boom")
		}

		return []ReleaseInfo{mustRelease(t, "0.7.0", "ok", time.Now().UTC())}, nil
	}
	src := &stubSource{name: "meshmap-lite", hook: hook}
	if err := m.Register(SourceSpec{Name: "meshmap-lite", Source: src, Label: "Map", CurrentVersion: "0.6.0"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if snap, ok := m.Snapshot("meshmap-lite"); ok && snap.Latest.Version == "0.7.0" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected manager to recover and cache a snapshot")
}

// TestPublishIsConcurrencySafe hammers publish from many goroutines
// across many sources at once. It catches a regression to a read lock
// in publish (which mutates m.subs and m.all): the Go runtime detects
// the resulting concurrent map write as a fatal error, failing the
// test. Run without -race; the runtime detector is what we want.
func TestPublishIsConcurrencySafe(t *testing.T) {
	m := newTestManager(t, time.Hour)

	const sources = 6
	const perSource = 64

	for i := range sources {
		name := fmt.Sprintf("s%d", i)
		src := &stubSource{name: name, pageURL: "https://example.com/" + name}
		if err := m.Register(SourceSpec{Name: name, Source: src, Label: name}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}

	var wg sync.WaitGroup
	for i := range sources {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			name := fmt.Sprintf("s%d", i)
			snap := UpdateSnapshot{
				Source:         name,
				Latest:         ReleaseInfo{Version: "1.0.0"},
				CurrentVersion: "1.0.0",
			}
			for range perSource {
				m.publish(name, snap)
			}
		}(i)
	}
	wg.Wait()
}

func TestFailedFetchDoesNotPoisonCache(t *testing.T) {
	m := newTestManager(t, 25*time.Millisecond)
	hook := func(context.Context) ([]ReleaseInfo, error) {
		return []ReleaseInfo{mustRelease(t, "0.7.0", "ok", time.Now().UTC())}, nil
	}
	src := &stubSource{name: "meshmap-lite", hook: hook}
	if err := m.Register(SourceSpec{Name: "meshmap-lite", Source: src, Label: "Map", CurrentVersion: "0.6.0"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	if !waitFor(func() bool {
		_, ok := m.Snapshot("meshmap-lite")

		return ok
	}) {
		t.Fatalf("expected initial cache to populate")
	}

	// Swap the hook to one that always fails. The next tick should fail
	// but leave the previous snapshot in place.
	src.SetHook(func(context.Context) ([]ReleaseInfo, error) {
		return nil, errors.New("upstream down")
	})

	// Wait long enough for at least one failing tick.
	if !waitFor(func() bool {
		return src.calls.Load() >= 2
	}) {
		t.Fatalf("expected at least 2 fetch attempts, got %d", src.calls.Load())
	}

	if snap, ok := m.Snapshot("meshmap-lite"); !ok || snap.Latest.Version != "0.7.0" {
		t.Fatalf("expected cache to retain previous snapshot, got %+v / ok=%v", snap, ok)
	}
}

func TestTwoSourcesDoNotShareState(t *testing.T) {
	m := newTestManager(t, 25*time.Millisecond)
	a := &stubSource{
		name: "a",
		hook: func(context.Context) ([]ReleaseInfo, error) {
			return []ReleaseInfo{mustRelease(t, "1.0.0", "a", time.Now().UTC())}, nil
		},
	}
	b := &stubSource{
		name: "b",
		hook: func(context.Context) ([]ReleaseInfo, error) {
			return []ReleaseInfo{mustRelease(t, "2.0.0", "b", time.Now().UTC())}, nil
		},
	}
	if err := m.Register(SourceSpec{Name: "a", Source: a, Label: "A", CurrentVersion: "0.1.0"}); err != nil {
		t.Fatalf("register a: %v", err)
	}
	if err := m.Register(SourceSpec{Name: "b", Source: b, Label: "B", CurrentVersion: "0.1.0"}); err != nil {
		t.Fatalf("register b: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	if !waitFor(func() bool {
		sa, oka := m.Snapshot("a")
		sb, okb := m.Snapshot("b")

		return oka && okb && sa.Latest.Version == "1.0.0" && sb.Latest.Version == "2.0.0"
	}) {
		t.Fatalf("expected both sources to populate independently")
	}
}

func TestSubscribeReceivesPerSourceSnapshots(t *testing.T) {
	m := newTestManager(t, 25*time.Millisecond)
	src := &stubSource{
		name: "meshmap-lite",
		hook: func(context.Context) ([]ReleaseInfo, error) {
			return []ReleaseInfo{mustRelease(t, "0.7.0", "ok", time.Now().UTC())}, nil
		},
	}
	if err := m.Register(SourceSpec{Name: "meshmap-lite", Source: src, Label: "Map", CurrentVersion: "0.6.0"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	ch, unsub := m.Subscribe("meshmap-lite")
	t.Cleanup(unsub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	select {
	case snap := <-ch:
		if snap.Source != "meshmap-lite" {
			t.Fatalf("unexpected source name: %q", snap.Source)
		}
		if snap.Latest.Version != "0.7.0" {
			t.Fatalf("unexpected latest version: %q", snap.Latest.Version)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected subscriber to receive a snapshot")
	}
}

func TestSubscribeAllReceivesAllSources(t *testing.T) {
	m := newTestManager(t, 25*time.Millisecond)
	a := &stubSource{
		name: "a",
		hook: func(context.Context) ([]ReleaseInfo, error) {
			return []ReleaseInfo{mustRelease(t, "1.0.0", "a", time.Now().UTC())}, nil
		},
	}
	b := &stubSource{
		name: "b",
		hook: func(context.Context) ([]ReleaseInfo, error) {
			return []ReleaseInfo{mustRelease(t, "2.0.0", "b", time.Now().UTC())}, nil
		},
	}
	if err := m.Register(SourceSpec{Name: "a", Source: a, Label: "A"}); err != nil {
		t.Fatalf("register a: %v", err)
	}
	if err := m.Register(SourceSpec{Name: "b", Source: b, Label: "B"}); err != nil {
		t.Fatalf("register b: %v", err)
	}

	ch, unsub := m.SubscribeAll()
	t.Cleanup(unsub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	got := make(map[string]string)
	deadline := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case ns := <-ch:
			got[ns.Name] = ns.Snapshot.Latest.Version
		case <-deadline:
			t.Fatalf("expected both sources on SubscribeAll, got %v", got)
		}
	}
	if got["a"] != "1.0.0" || got["b"] != "2.0.0" {
		t.Fatalf("unexpected payloads: %v", got)
	}
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	m := newTestManager(t, 25*time.Millisecond)
	src := &stubSource{
		name: "a",
		hook: func(context.Context) ([]ReleaseInfo, error) {
			return []ReleaseInfo{mustRelease(t, "1.0.0", "a", time.Now().UTC())}, nil
		},
	}
	if err := m.Register(SourceSpec{Name: "a", Source: src, Label: "A"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	ch, unsub := m.Subscribe("a")

	// Drain any in-flight values first.
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed before unsubscribe called")
			}
		default:
			goto unsubscribe
		}
	}
unsubscribe:
	unsub()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatalf("expected channel to be closed after unsubscribe")
		}
	case <-time.After(time.Second):
		t.Fatalf("expected channel close after unsubscribe")
	}
}

func TestSlowSubscriberDoesNotBlockFetchLoop(t *testing.T) {
	m := newTestManager(t, 25*time.Millisecond)
	src := &stubSource{
		name: "a",
		hook: func(context.Context) ([]ReleaseInfo, error) {
			return []ReleaseInfo{mustRelease(t, "1.0.0", "a", time.Now().UTC())}, nil
		},
	}
	if err := m.Register(SourceSpec{Name: "a", Source: src, Label: "A"}); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Take a subscription and never read from it; the buffer fills, the
	// next tick should still happen, and the cache should keep updating.
	_, unsub := m.Subscribe("a")
	t.Cleanup(unsub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.Start(ctx)

	firstVersion := ""
	if !waitFor(func() bool {
		snap, ok := m.Snapshot("a")
		if !ok {
			return false
		}
		firstVersion = snap.Latest.Version

		return true
	}) {
		t.Fatalf("expected initial cache to populate")
	}

	if !waitFor(func() bool {
		return src.calls.Load() >= 2
	}) {
		t.Fatalf("expected fetch loop to keep ticking despite slow subscriber, calls=%d", src.calls.Load())
	}
	if snap, _ := m.Snapshot("a"); snap.Latest.Version != firstVersion {
		t.Fatalf("snapshot changed unexpectedly: %q -> %q", firstVersion, snap.Latest.Version)
	}
}

func TestBuildSnapshotUpdateAvailableSemantics(t *testing.T) {
	m := newTestManager(t, time.Hour)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name          string
		current       string
		wantAvailable bool
	}{
		{"newer", "0.6.0", true},
		{"equal", "0.7.0", false},
		{"older", "0.8.0", false},
		{"dev current", "dev", true},
		{"invalid latest", "0.6.0", false},
	}
	for _, c := range cases {
		var rel ReleaseInfo
		if c.name == "invalid latest" {
			rel = mustRelease(t, "not-semver", "x", now)
		} else {
			rel = mustRelease(t, "0.7.0", "x", now)
		}
		snap := m.buildSnapshot("x", c.current, []ReleaseInfo{rel})
		if snap.UpdateAvailable != c.wantAvailable {
			t.Fatalf("%s: UpdateAvailable=%v, want %v", c.name, snap.UpdateAvailable, c.wantAvailable)
		}
	}
}

func TestCacheKeyIsStableAcrossNormalizations(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rel1 := ReleaseInfo{Version: "0.7.0", PublishedAt: now}
	rel2 := ReleaseInfo{Version: "0.7.0", PublishedAt: now}
	if computeSourceHash(rel1) != computeSourceHash(rel2) {
		t.Fatalf("expected stable hash for identical releases")
	}
	rel3 := ReleaseInfo{Version: "0.7.1", PublishedAt: now}
	if computeSourceHash(rel1) == computeSourceHash(rel3) {
		t.Fatalf("expected hash to differ between versions")
	}
}

func TestNormalizeSemverAndIsReleaseNewer(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"0.7.0", "v0.7.0"},
		{"v0.7.0", "v0.7.0"},
		{"  0.7.0 ", "v0.7.0"},
	}
	for _, c := range cases {
		if got := normalizeSemver(c.in); got != c.want {
			t.Fatalf("normalizeSemver(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// waitFor polls cond until it returns true or 2s elapses.
func waitFor(cond func() bool) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}

	return cond()
}
