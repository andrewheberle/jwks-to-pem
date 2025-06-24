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
	}

	return "unknown signal"
}
