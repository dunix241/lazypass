package bitwarden

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"lazypass/internal/vault"
)

type serverAccessChecker interface {
	CheckServerAccess(context.Context, string) error
}

var accessClient = &http.Client{
	Timeout: 5 * time.Second,
	// A redirect to a login/challenge page is not proof of API access.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func (ExecRunner) CheckServerAccess(ctx context.Context, server string) error {
	return checkServerAccess(ctx, accessClient, server)
}

// This unauthenticated probe catches gateway denials before bw can interpret
// them as invalid credentials. Access can still change between the two requests.
func checkServerAccess(ctx context.Context, client *http.Client, server string) error {
	endpoint, err := url.Parse(strings.TrimRight(server, "/") + "/api/config")
	if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.Scheme != "https" && endpoint.Scheme != "http" {
		return &commandFailure{vault.ErrUnavailable, "Invalid Bitwarden server URL"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &commandFailure{vault.ErrUnavailable, "Cannot reach Bitwarden; check your connection or VPN, then refresh"}
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return accessDenied()
	case res.StatusCode == http.StatusTooManyRequests:
		return &commandFailure{vault.ErrUnavailable, "Bitwarden access is rate limited; wait before refreshing"}
	case res.StatusCode != http.StatusOK:
		return &commandFailure{vault.ErrUnavailable, "Bitwarden server access failed; check your server or proxy, then refresh"}
	default:
		return nil
	}
}

func accessDenied() error {
	return &commandFailure{vault.ErrUnavailable, "Server access denied. Disconnect the VPN or check the IP allowlist, then refresh"}
}
