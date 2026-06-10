//go:build !linux

package runner

import (
	"os"
	"syscall"
)

// procAttr is a no-op off Linux (the runner only ships in a Linux container;
// this keeps the package buildable for local tooling on other platforms).
func procAttr() *syscall.SysProcAttr { return nil }

func killGroup(p *os.Process, sig syscall.Signal) error { return p.Signal(sig) }
