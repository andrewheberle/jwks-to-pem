package cmd

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/andrewheberle/jwks-to-pem/pkg/jwks"
	"github.com/andrewheberle/simplecommand"
	"github.com/bep/simplecobra"
)

type rootCommand struct {
	jwksUrl       string
	outputDir     string
	outputPattern string
	timeout       time.Duration
	debug         bool
	reloadUrl     string
	reloadMethod  string
	reloadPid     int
	reloadPidfile string
	reloadSignal  signal

	logger *slog.Logger

	*simplecommand.Command
}

type signal struct {
	v syscall.Signal
}

func (sig *signal) Type() string {
	return "signal"
}

func (c *rootCommand) Init(cd *simplecobra.Commandeer) error {
	if err := c.Command.Init(cd); err != nil {
		return err
	}

	// set default for reload signal
	c.reloadSignal = signal{syscall.SIGHUP}

	// command line flags
	cmd := cd.CobraCommand
	cmd.Flags().StringVarP(&c.jwksUrl, "url", "u", "", "URL for JSON Web Key Set (JWKS)")
	cmd.Flags().StringVarP(&c.outputDir, "out", "o", "", "Output directory")
	cmd.Flags().StringVarP(&c.outputPattern, "pattern", "p", "{{ .KeyID }}.pem", "Output pattern")
	cmd.Flags().DurationVar(&c.timeout, "timeout", time.Second*5, "Timeout to retrieve JWKS")
	cmd.Flags().StringVar(&c.reloadUrl, "reload.url", "", "URL to use for reloads")
	cmd.Flags().IntVar(&c.reloadPid, "reload.pid", 0, "Process ID to signal for reloads")
	cmd.Flags().StringVar(&c.reloadPidfile, "reload.pidfile", "", "File to look up process ID to signal for reloads")
	cmd.Flags().Var(&c.reloadSignal, "reload.signal", "Process ID to signal for reloads")
	cmd.Flags().StringVar(&c.reloadMethod, "reload.method", http.MethodPost, "Method to use for reload URL")
	cmd.Flags().BoolVar(&c.debug, "debug", false, "Enable debug logging")

	// require a url
	cmd.MarkFlagRequired("url")

	// dont allow both pid and pidfile togther
	cmd.MarkFlagsMutuallyExclusive("reload.pid", "reload.pidfile")

	// dont allow signal or webhook based reloading togther
	cmd.MarkFlagsMutuallyExclusive("reload.url", "reload.pid")
	cmd.MarkFlagsMutuallyExclusive("reload.url", "reload.pidfile")

	return nil
}

func (c *rootCommand) PreRun(this, runner *simplecobra.Commandeer) error {
	if err := c.Command.PreRun(this, runner); err != nil {
		return err
	}

	// set up logger
	logLevel := new(slog.LevelVar)
	c.logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
	if c.debug {
		logLevel.Set(slog.LevelDebug)
	}

	// parse provided pattern
	if _, err := template.New("pattern").Parse(c.outputPattern); err != nil {
		return fmt.Errorf("problem parsing pattern: %w", err)
	}

	return nil
}

func (c *rootCommand) Run(ctx context.Context, cd *simplecobra.Commandeer, args []string) error {
	// fetch JWKS
	j, err := jwks.GetJWKS(c.jwksUrl, c.timeout)
	if err != nil {
		return fmt.Errorf("problem fetching JWKS: %w", err)
	}

	// write keys based on pattern
	changed, err := j.WriteKeys(c.outputPattern, c.outputDir)
	if err != nil {
		return fmt.Errorf("problem processing keys: %w", err)
	}

	// check if any changes were made
	if !changed {
		c.logger.Info("no changes to keys")

		return nil
	}

	// no reload set up?
	if c.reloadPid == 0 && c.reloadPidfile == "" && c.reloadUrl == "" {
		return nil
	}

	// did we get a pidfile?
	if c.reloadPidfile != "" {
		b, err := os.ReadFile(c.reloadPidfile)
		if err != nil {
			return fmt.Errorf("could not open pid file: %w", err)
		}

		c.reloadPid, err = strconv.Atoi(strings.TrimSpace(string(b)))
		if err != nil {
			return fmt.Errorf("invalid pid from pidfile: %w", err)
		}
	}

	// pid based reload
	if c.reloadPid != 0 {
		p, err := os.FindProcess(c.reloadPid)
		if err != nil {
			return fmt.Errorf("could not find process: %w", err)
		}

		if err := p.Signal(c.reloadSignal.v); err != nil {
			return fmt.Errorf("reload error: %w", err)
		}

		c.logger.Info("reload of process completed", "pid", c.reloadPid, "signal", c.reloadSignal.String())

		return nil
	}

	// url/webhook based reload
	req, err := http.NewRequest(c.reloadMethod, c.reloadUrl, nil)
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

	c.logger.Info("reload of process completed", "url", c.reloadUrl, "method", c.reloadMethod)

	return nil
}

func Execute(args []string) error {
	// Set up command
	command := &rootCommand{
		Command: simplecommand.New(
			"jwks-to-pem",
			"Retrieve keys from a JWKS URL and save as PEM encoded files",
			simplecommand.WithViper("jwks", strings.NewReplacer("-", "_", ".", "_")),
		),
	}

	// Set up simplecobra
	x, err := simplecobra.New(command)
	if err != nil {
		return err
	}

	// run things
	if _, err := x.Execute(context.Background(), args); err != nil {
		return err
	}

	return nil
}
