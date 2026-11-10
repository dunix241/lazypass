package clipboard

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"
)

var ErrImageUnavailable = errors.New("clipboard image unavailable; use a local PNG/JPEG file")
var ErrImageTimeout = errors.New("clipboard image read timed out")
var ErrImageLimit = errors.New("clipboard image exceeds the 16 MiB limit")

const maxClipboardImage = 16 << 20

// ReadImage reads the local clipboard only; SSH clipboard forwarding is text-only.
func ReadImage(ctx context.Context) ([]byte, error) {
	if remoteSession(os.Getenv) || runtime.GOOS == "android" {
		return nil, ErrImageUnavailable
	}
	commands := imageCommands(runtime.GOOS, os.Getenv("WAYLAND_DISPLAY") != "")
	for _, args := range commands {
		data, err := readImageCommand(ctx, args)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if errors.Is(err, ErrImageTimeout) || errors.Is(err, ErrImageLimit) {
			return nil, err
		}
		if err == nil && len(data) > 0 {
			return data, nil
		}
		clear(data)
	}
	return nil, ErrImageUnavailable
}

func imageCommands(platform string, wayland bool) [][]string {
	switch platform {
	case "linux", "freebsd", "openbsd":
		if wayland {
			return [][]string{
				{"wl-paste", "--no-newline", "--type", "image/png"},
				{"wl-paste", "--no-newline", "--type", "image/jpeg"},
			}
		}
		return [][]string{
			{"xclip", "-selection", "clipboard", "-t", "image/png", "-o"},
			{"xclip", "-selection", "clipboard", "-t", "image/jpeg", "-o"},
		}
	case "windows":
		return [][]string{{"powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-Command", `Add-Type -AssemblyName System.Windows.Forms; $i=[System.Windows.Forms.Clipboard]::GetImage(); if($null -eq $i){exit 1}; try{$s=[Console]::OpenStandardOutput(); $i.Save($s,[System.Drawing.Imaging.ImageFormat]::Png)}finally{$i.Dispose()}`}}
	case "darwin":
		return [][]string{{"osascript", "-l", "JavaScript", "-e", `ObjC.import('AppKit'); var p=$.NSPasteboard.generalPasteboard; var d=p.dataForType($.NSPasteboardTypePNG); if(!d){var t=p.dataForType($.NSPasteboardTypeTIFF); if(t){var r=$.NSBitmapImageRep.imageRepWithData(t); d=r.representationUsingTypeProperties($.NSBitmapImageFileTypePNG,$({}));}} if(!d){throw Error('No image');} $.NSFileHandle.fileHandleWithStandardOutput.writeData(d);`}}
	}
	return nil
}

func readImageCommand(parent context.Context, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.WaitDelay = time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, ErrImageUnavailable
	}
	if err := cmd.Start(); err != nil {
		return nil, ErrImageUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(stdout, maxClipboardImage+1))
	if err != nil || len(data) > maxClipboardImage {
		cancel()
	}
	waitErr := cmd.Wait()
	if len(data) > maxClipboardImage {
		clear(data)
		return nil, ErrImageLimit
	}
	if ctx.Err() != nil {
		clear(data)
		if parent.Err() != nil {
			return nil, parent.Err()
		}
		return nil, ErrImageTimeout
	}
	if err != nil || waitErr != nil || len(data) > maxClipboardImage {
		clear(data)
		return nil, ErrImageUnavailable
	}
	return data, nil
}
