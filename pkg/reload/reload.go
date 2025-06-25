package reload

import (
	"bytes"
	"fmt"
	"net"
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
	url     string
	method  string
	payload []byte
}

func NewHTTPReloader(url string, method string, payload []byte) (*HTTPReloader, error) {
	return &HTTPReloader{url, method, payload}, nil
}

func (r *HTTPReloader) Info() string {
	return r.url
}

func (r *HTTPReloader) Reload() error {
	var buf bytes.Buffer

	if r.payload != nil {
		// use payload as buf
		buf = *bytes.NewBuffer(r.payload)
	}

	// set up request
	req, err := http.NewRequest(r.method, r.url, &buf)
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

type UnixSocketReloader struct {
	socket  string
	payload []byte
}

func NewUnixSocketReloader(socket string, payload []byte) (*UnixSocketReloader, error) {
	return &UnixSocketReloader{socket, payload}, nil
}

func (r *UnixSocketReloader) Info() string {
	return r.socket
}

func (r *UnixSocketReloader) Reload() error {
	// connect to socket
	conn, err := net.Dial("unix", r.socket)
	if err != nil {
		return fmt.Errorf("could not connect: %w", err)
	}
	defer conn.Close()

	// send payload
	if _, err := conn.Write(r.payload); err != nil {
		return fmt.Errorf("error writing: %w", err)
	}

	return nil
}
