package reload

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type Reloader interface {
	Reload() error
	Info() string
}

type ProcessReloader struct {
	pid    int
	signal syscall.Signal
}

func NewProcessReloaderFromPidfile(pidfile string, signal syscall.Signal) (*ProcessReloader, error) {
	b, err := os.ReadFile(pidfile)
	if err != nil {
		return nil, fmt.Errorf("could not open pid file: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return nil, fmt.Errorf("invalid pid from pidfile: %w", err)
	}

	return NewProcessReloader(pid, signal)
}

func NewProcessReloader(pid int, signal syscall.Signal) (*ProcessReloader, error) {
	return &ProcessReloader{pid, signal}, nil
}

func (r *ProcessReloader) Info() string {
	p, err := os.FindProcess(r.pid)
	if err != nil {
		return "process not found"
	}

	if err := p.Signal(syscall.Signal(0)); err != nil {
		return "process not found"
	}

	return fmt.Sprint("PID = %d", p.Pid)
}

func (r *ProcessReloader) Pid() int {
	return r.pid
}

func (r *ProcessReloader) Reload() error {
	p, err := os.FindProcess(r.pid)
	if err != nil {
		return fmt.Errorf("could not find process: %w", err)
	}

	if err := p.Signal(r.signal); err != nil {
		return fmt.Errorf("reload error: %w", err)
	}

	return nil
}

type HTTPReloader struct {
	url    string
	method string
}

func NewHTTPReloader(url string, method string) (*HTTPReloader, error) {
	return &HTTPReloader{url, method}, nil
}

func (r *HTTPReloader) Info() string {
	return r.url
}

func (r *HTTPReloader) Reload() error {
	req, err := http.NewRequest(r.method, r.url, nil)
	if err != nil {
		return fmt.Errorf("could not build request: %w", err)
	}

	// do request
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error during request: %w", err)
	}

	// check response
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("bad response code: %d", res.StatusCode)
	}

	return nil
}
