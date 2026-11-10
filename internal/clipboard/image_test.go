package clipboard

import (
	"context"
	"os"
	"testing"
)

func TestImageCommandSelection(t *testing.T) {
	for _, tc := range []struct {
		platform string
		wayland  bool
		command  string
	}{
		{"linux", true, "wl-paste"},
		{"linux", false, "xclip"},
		{"darwin", false, "osascript"},
		{"windows", false, "powershell.exe"},
	} {
		commands := imageCommands(tc.platform, tc.wayland)
		if len(commands) == 0 || commands[0][0] != tc.command {
			t.Fatal("incorrect image adapter")
		}
	}
}

func TestRemoteImageReadDoesNotUseHostClipboard(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "public-fixture")
	if _, err := ReadImage(context.Background()); err != ErrImageUnavailable {
		t.Fatal("remote clipboard was accessed")
	}
}

func TestImageCommandBoundsAndCancellation(t *testing.T) {
	if os.Getenv("LAZYPASS_IMAGE_HELPER") == "1" {
		_, _ = os.Stdout.Write(make([]byte, maxClipboardImage+1))
		os.Exit(0)
	}
	t.Setenv("LAZYPASS_IMAGE_HELPER", "1")
	data, err := readImageCommand(context.Background(), []string{os.Args[0], "-test.run=TestImageCommandBoundsAndCancellation"})
	if err != ErrImageLimit || data != nil {
		t.Fatal("oversize clipboard data accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readImageCommand(ctx, []string{os.Args[0]}); err == nil {
		t.Fatal("canceled command succeeded")
	}
}
