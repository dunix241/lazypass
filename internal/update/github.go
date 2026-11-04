package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const Repository = "dunix241/lazypass"
const releasesURL = "https://github.com/" + Repository + "/releases"

type GitHub struct {
	Client   *http.Client
	Endpoint string
}

type LookupError struct{ Status int }

func (e *LookupError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("release lookup returned HTTP %d", e.Status)
	}
	return "release lookup unavailable"
}

func transient(err error) bool {
	var failure *LookupError
	return errors.As(err, &failure) && (failure.Status == 0 || failure.Status == http.StatusTooManyRequests || failure.Status >= 500)
}

func (g GitHub) Latest(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	endpoint := g.Endpoint
	if endpoint == "" {
		endpoint = "https://api.github.com/repos/" + Repository + "/releases/latest"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "lazypass-update-check")
	client := g.Client
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return Release{}, ctx.Err()
		}
		return Release{}, &LookupError{}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return Release{}, &LookupError{Status: response.StatusCode}
	}
	const limit = 64 << 10
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil || len(data) > limit {
		return Release{}, fmt.Errorf("invalid release response")
	}
	var result struct {
		Tag        string `json:"tag_name"`
		URL        string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if json.Unmarshal(data, &result) != nil || result.Draft || result.Prerelease || !Stable(result.Tag) {
		return Release{}, fmt.Errorf("invalid stable release response")
	}
	u, err := url.Parse(result.URL)
	if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(result.URL, releasesURL+"/tag/") {
		return Release{}, fmt.Errorf("invalid release URL")
	}
	return Release{Version: result.Tag, URL: result.URL}, nil
}
