package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type memoryStore struct{ cache Cache }

func (s *memoryStore) LoadUpdates() (Cache, error)   { return s.cache, nil }
func (s *memoryStore) SaveUpdates(cache Cache) error { s.cache = cache; return nil }

type fakeSource struct {
	calls int
	err   error
}

type recoveringSource struct{ calls int }

func (s *recoveringSource) Latest(context.Context) (Release, error) {
	s.calls++
	if s.calls == 1 {
		return Release{}, &LookupError{Status: 503}
	}
	return Release{Version: "v2.0.0", URL: "https://github.com/dunix241/lazypass/releases/tag/v2.0.0"}, nil
}

func TestTransientFailureRetriesOnce(t *testing.T) {
	source := &recoveringSource{}
	checker := Checker{Source: source, Store: &memoryStore{}}
	_, available, err := checker.Check(context.Background(), "v1.0.0", false)
	if err != nil || !available || source.calls != 2 {
		t.Fatalf("retry failed: %d %v", source.calls, err)
	}
}

func (s *fakeSource) Latest(context.Context) (Release, error) {
	s.calls++
	return Release{Version: "v1.10.0", URL: "https://github.com/dunix241/lazypass/releases/tag/v1.10.0"}, s.err
}

func TestStableVersionPolicy(t *testing.T) {
	for _, value := range []string{"dev", "v1.0.0-dirty", "v1.0.0-rc.1", "v1.0.0+local", "01.0.0"} {
		if Stable(value) {
			t.Fatalf("development/invalid version accepted: %s", value)
		}
	}
	if !Newer("v1.10.0", "v1.9.9") || Newer("v1.0.0", "v2.0.0") || Newer("v1.0.0", "1.0.0") {
		t.Fatal("incorrect semantic comparison")
	}
}

func TestCacheDismissalAndExplicitChecks(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store, source := &memoryStore{}, &fakeSource{}
	c := Checker{Source: source, Store: store, Now: func() time.Time { return now }}
	release, available, err := c.Check(context.Background(), "v1.0.0", false)
	if err != nil || !available || source.calls != 1 {
		t.Fatal("initial check failed")
	}
	if err := c.Dismiss(release); err != nil {
		t.Fatal(err)
	}
	_, available, err = c.Check(context.Background(), "v1.0.0", false)
	if err != nil || available || source.calls != 1 {
		t.Fatal("dismissal/cache ignored")
	}
	_, available, err = c.Check(context.Background(), "v1.0.0", true)
	if err != nil || !available || source.calls != 2 {
		t.Fatal("explicit check did not bypass cache/dismissal")
	}
	now = now.Add(Interval)
	_, _, _ = c.Check(context.Background(), "v1.0.0", false)
	if source.calls != 3 {
		t.Fatal("expired interval not checked")
	}
}

func TestFailuresThrottleAndDevelopmentSkips(t *testing.T) {
	source := &fakeSource{err: errors.New("offline")}
	store := &memoryStore{}
	c := Checker{Source: source, Store: store}
	if _, _, err := c.Check(context.Background(), "dev", false); err != nil || source.calls != 0 {
		t.Fatal("development build contacted source")
	}
	if _, _, err := c.Check(context.Background(), "v1.0.0", false); err == nil {
		t.Fatal("missing lookup failure")
	}
	_, _, _ = c.Check(context.Background(), "v1.0.0", false)
	if source.calls != 1 {
		t.Fatal("failure was not throttled")
	}
}

func TestGitHubRejectsInvalidReleaseData(t *testing.T) {
	for _, body := range []string{
		`{"tag_name":"v2.0.0","html_url":"https://github.com/dunix241/lazypass/releases/tag/v2.0.0"}`,
		`{"tag_name":"v2.0.0","html_url":"https://evil.invalid/release"}`,
		`{"tag_name":"v2.0.0","html_url":"https://github.com/dunix241/lazypass/releases/tag/v2.0.0","prerelease":true}`,
		`not-json`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("User-Agent") != "lazypass-update-check" {
				t.Error("missing user agent")
			}
			_, _ = w.Write([]byte(body))
		}))
		_, err := (GitHub{Client: server.Client(), Endpoint: server.URL}).Latest(context.Background())
		server.Close()
		valid := body == `{"tag_name":"v2.0.0","html_url":"https://github.com/dunix241/lazypass/releases/tag/v2.0.0"}`
		if (err == nil) != valid {
			t.Fatalf("release validation: %v", err)
		}
	}
}

func TestGitHubHonorsCancellationAndResponseLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(make([]byte, 65537)) }))
	defer server.Close()
	source := GitHub{Client: server.Client(), Endpoint: server.URL}
	if _, err := source.Latest(context.Background()); err == nil {
		t.Fatal("oversized response accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := source.Latest(ctx); err == nil {
		t.Fatal("canceled request succeeded")
	}
}
