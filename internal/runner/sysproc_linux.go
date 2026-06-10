//go:build linux

package runner

import (
	"os"
	"syscall"
)

// procAttr returns nil so the PTY layer (creack/pty) can set Setsid+Setctty.
// setsid already makes the child a new session and process-group leader
// (pgid == pid), which is exactly what we need to signal the whole tree of
// ansible workers via the negative pid. Setting Setpgid here as well would make
// the child try to setpgid() after becoming a session leader, which fails with
// EPERM ("operation not permitted").
func procAttr() *syscall.SysProcAttr {
	return nil
}

// killGroup signals the entire process group (negative pid).
func killGroup(p *os.Process, sig syscall.Signal) error {
	return syscall.Kill(-p.Pid, sig)
}
