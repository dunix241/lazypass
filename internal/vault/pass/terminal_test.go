package pass

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCachedCredentialProbeDoesNotChangeTerminalEnvironment(t *testing.T) {
	env := []string{"GPG_TTY=/dev/old", "PASSWORD_STORE_GPG_OPTS=--batch --pinentry-mode error"}
	if got := prepareGPGTerminal(context.Background(), env, nil); !reflect.DeepEqual(got, env) {
		t.Fatal("cached probe changed terminal state")
	}
}

func TestTerminalSetupReplacesStaleEnvironment(t *testing.T) {
	for _, agentFailure := range []bool{false, true} {
		env := []string{"GPG_TTY=/dev/stale", "DISPLAY=:1", "GPG_TTY=/dev/older"}
		calls := 0
		got := configureGPGTerminal(context.Background(), env, func(ctx context.Context, args, received []string) ([]byte, error) {
			calls++
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("terminal command has no timeout")
			}
			if calls == 1 {
				if !reflect.DeepEqual(args, []string{"tty"}) || !reflect.DeepEqual(received, env) {
					t.Fatal("probe environment changed")
				}
				return []byte("/dev/pts/42\n"), nil
			}
			if !reflect.DeepEqual(args, []string{"gpg-connect-agent", "updatestartuptty", "/bye"}) {
				t.Fatal("unexpected agent command")
			}
			if agentFailure {
				return nil, errors.New("agent unavailable")
			}
			return nil, nil
		})
		if calls != 2 || !reflect.DeepEqual(got, []string{"DISPLAY=:1", "GPG_TTY=/dev/pts/42"}) {
			t.Fatalf("terminal environment: %v", got)
		}
		if env[0] != "GPG_TTY=/dev/stale" {
			t.Fatal("input environment mutated")
		}
	}
}

func TestTerminalSetupSkipsUnavailableAndInvalidTerminal(t *testing.T) {
	for _, output := range []string{"not a tty\n", "/dev/pts/1\n/dev/pts/2", "", "/tmp/fake"} {
		calls := 0
		env := []string{"GPG_TTY=/dev/old"}
		got := configureGPGTerminal(context.Background(), env, func(context.Context, []string, []string) ([]byte, error) { calls++; return []byte(output), nil })
		if calls != 1 || !reflect.DeepEqual(got, env) {
			t.Fatal("invalid terminal accepted")
		}
	}
	for _, options := range []string{"--batch --pinentry-mode error", "--pinentry-mode=error"} {
		env := []string{"PASSWORD_STORE_GPG_OPTS=" + options}
		configureGPGTerminal(context.Background(), env, func(context.Context, []string, []string) ([]byte, error) {
			t.Fatal("cached probe ran terminal command")
			return nil, nil
		})
	}
	if promptDisabled([]string{"PASSWORD_STORE_GPG_OPTS=--pinentry-mode error", "PASSWORD_STORE_GPG_OPTS=--pinentry-mode ask"}) {
		t.Fatal("ignored final option override")
	}
	if promptDisabled([]string{"PASSWORD_STORE_GPG_OPTS=--comment=--pinentry-mode error"}) {
		t.Fatal("substring treated as pinentry option")
	}
}

func TestCanceledTerminalSetupRunsNoCommands(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	configureGPGTerminal(ctx, nil, func(context.Context, []string, []string) ([]byte, error) {
		t.Fatal("command ran after cancellation")
		return nil, nil
	})
}

func TestTerminalSetupFailureDoesNotExposeCommandOutput(t *testing.T) {
	env := []string{"GPG_TTY=/dev/old"}
	got := configureGPGTerminal(context.Background(), env, func(context.Context, []string, []string) ([]byte, error) {
		return []byte("secret-value"), errors.New("secret-value")
	})
	if strings.Contains(strings.Join(got, " "), "secret-value") || !reflect.DeepEqual(got, env) {
		t.Fatal("failed probe output leaked")
	}
}
