// Package update checks stable releases without installing software.
package update

import (
	"context"
	"errors"
	"fmt"
	"lazypass/internal/debug"
	"regexp"
	"strconv"
	"time"
)

const Interval = 24 * time.Hour

type Release struct {
	Version string `yaml:"version"`
	URL     string `yaml:"url"`
}

type Cache struct {
	CheckedAt time.Time `yaml:"checkedAt,omitempty"`
	Release   Release   `yaml:"release,omitempty"`
	Dismissed string    `yaml:"dismissed,omitempty"`
}

type Source interface {
	Latest(context.Context) (Release, error)
}
type Store interface {
	LoadUpdates() (Cache, error)
	SaveUpdates(Cache) error
}

type Checker struct {
	Source           Source
	Store            Store
	Now              func() time.Time
	IncludeDismissed bool
}

var stablePattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func version(value string) ([3]uint64, bool) {
	var result [3]uint64
	parts := stablePattern.FindStringSubmatch(value)
	if parts == nil {
		return result, false
	}
	for i := range result {
		n, err := strconv.ParseUint(parts[i+1], 10, 64)
		if err != nil {
			return result, false
		}
		result[i] = n
	}
	return result, true
}

func Stable(value string) bool { _, ok := version(value); return ok }

func Newer(latest, current string) bool {
	a, ok := version(latest)
	if !ok {
		return false
	}
	b, ok := version(current)
	if !ok {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

func (c Checker) Check(ctx context.Context, current string, force bool) (Release, bool, error) {
	if !Stable(current) {
		return Release{}, false, nil
	}
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	cache, err := c.Store.LoadUpdates()
	if err != nil {
		return Release{}, false, err
	}
	if force || cache.CheckedAt.IsZero() || now.Sub(cache.CheckedAt) >= Interval || now.Before(cache.CheckedAt) {
		debug.Event("update source lookup")
		cache.CheckedAt = now
		// Persist the attempt even on failure; explicit checks may always retry.
		if err := c.Store.SaveUpdates(cache); err != nil {
			return Release{}, false, err
		}
		release, err := c.Source.Latest(ctx)
		if err != nil && (transient(err) || errors.Is(err, context.DeadlineExceeded)) && ctx.Err() == nil {
			debug.Event("update source retry")
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return Release{}, false, ctx.Err()
			case <-timer.C:
			}
			release, err = c.Source.Latest(ctx)
		}
		if err != nil {
			debug.Event("update source failed")
			return Release{}, false, err
		}
		if !Stable(release.Version) {
			return Release{}, false, fmt.Errorf("release is not a stable version")
		}
		cache.Release = release
		debug.Event("update source succeeded")
		if err := c.Store.SaveUpdates(cache); err != nil {
			return Release{}, false, err
		}
	} else {
		debug.Event("update cached result")
	}
	available := Newer(cache.Release.Version, current) && (force || c.IncludeDismissed || cache.Dismissed != cache.Release.Version)
	return cache.Release, available, nil
}

func (c Checker) Dismiss(release Release) error {
	cache, err := c.Store.LoadUpdates()
	if err != nil {
		return err
	}
	cache.Dismissed = release.Version
	return c.Store.SaveUpdates(cache)
}
