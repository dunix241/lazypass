//go:build !linux && !darwin

package install

import "os/exec"

func configureCancellation(_ *exec.Cmd) {}
