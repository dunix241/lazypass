package pass

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/term"
	"lazypass/internal/debug"
)

func prepareGPGTerminal(ctx context.Context, env []string, terminal io.Reader) []string {
	if promptDisabled(env) || ctx.Err() != nil {
		return env
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return env
	}
	defer tty.Close()
	if terminal == nil {
		terminal = tty
	}
	if file, ok := terminal.(*os.File); ok && !term.IsTerminal(int(file.Fd())) {
		return env
	}
	return configureGPGTerminal(ctx, env, func(ctx context.Context, args []string, env []string) ([]byte, error) {
		command := exec.CommandContext(ctx, args[0], args[1:]...)
		command.Env = env
		command.Stdin = terminal
		return command.Output()
	})
}

type terminalCommand func(context.Context, []string, []string) ([]byte, error)

func configureGPGTerminal(ctx context.Context, env []string, run terminalCommand) []string {
	if promptDisabled(env) || ctx.Err() != nil {
		return env
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, err := run(probeCtx, []string{"tty"}, env)
	if err != nil {
		return env
	}
	path := strings.TrimSpace(string(output))
	if !strings.HasPrefix(path, "/dev/") || strings.ContainsAny(path, "\r\n\x00") {
		return env
	}
	updated := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, "GPG_TTY=") {
			updated = append(updated, item)
		}
	}
	updated = append(updated, "GPG_TTY="+path)
	if _, err := run(probeCtx, []string{"gpg-connect-agent", "updatestartuptty", "/bye"}, updated); err != nil {
		debug.Failure("gpg terminal update", debug.Terminal)
	}
	return updated
}

func promptDisabled(env []string) bool {
	for i := len(env) - 1; i >= 0; i-- {
		value, ok := strings.CutPrefix(env[i], "PASSWORD_STORE_GPG_OPTS=")
		if !ok {
			continue
		}
		fields := strings.Fields(value)
		for j, field := range fields {
			if field == "--pinentry-mode=error" || (field == "--pinentry-mode" && j+1 < len(fields) && fields[j+1] == "error") {
				return true
			}
		}
		return false
	}
	return false
}
