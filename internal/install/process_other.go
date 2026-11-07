//go:build !linux && !darwin && !android

package install

import "os/exec"

func configureCancellation(_ *exec.Cmd) {}
