package cmd

import (
	"fmt"
	"strings"
	"syscall"
)

func (sig *signal) Set(s string) error {
	switch strings.ToUpper(s) {
	case "HUP", "SIGHUP":
		sig.v = syscall.SIGHUP
	case "KILL", "SIGKILL":
		sig.v = syscall.SIGKILL
	case "USR1", "SIGUSR1":
		sig.v = syscall.SIGUSR1
	case "USR2", "SIGUSR2":
		sig.v = syscall.SIGUSR2

	default:
		return fmt.Errorf("unsupported signal: %s", s)
	}

	return nil
}

func (sig *signal) String() string {
	switch sig.v {
	case syscall.SIGHUP:
		return "SIGHUP"
	case syscall.SIGKILL:
		return "SIGKILL"
	case syscall.SIGUSR1:
		return "SIGUSR1"
	case syscall.SIGUSR2:
		return "SIGUSR2"
	}

	return "unknown signal"
}
