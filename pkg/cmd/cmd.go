package cmd

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/andrewheberle/jwks-to-pem/pkg/jwks"
	"github.com/andrewheberle/jwks-to-pem/pkg/reload"
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
	reloadPayload string
	reloadMethod  string
	reloadPid     int
	reloadPidfile string
	reloadSignal  signal
	reloadSocket  string

	logger *slog.Logger

	reloader reload.Reloader

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
	cmd.PersistentFlags().StringVarP(&c.jwksUrl, "url", "u", "", "URL for JSON Web Key Set (JWKS)")
	cmd.PersistentFlags().StringVarP(&c.outputDir, "out", "o", "", "Output directory")
	cmd.PersistentFlags().StringVarP(&c.outputPattern, "pattern", "p", "{{ .KeyID }}.pem", "Output pattern")
	cmd.PersistentFlags().DurationVar(&c.timeout, "timeout", time.Second*5, "Timeout to retrieve JWKS")
	cmd.PersistentFlags().StringVar(&c.reloadSocket, "reload.socket", "", "Socket to use for reloads")
	cmd.PersistentFlags().StringVar(&c.reloadUrl, "reload.url", "", "URL to use for reloads")
	cmd.PersistentFlags().StringVar(&c.reloadPayload, "reload.payload", "", "Payload for URL/socket based reloads")
	cmd.PersistentFlags().IntVar(&c.reloadPid, "reload.pid", 0, "Process ID to signal for reloads")
	cmd.PersistentFlags().StringVar(&c.reloadPidfile, "reload.pidfile", "", "File to look up process ID to signal for reloads")
	cmd.PersistentFlags().Var(&c.reloadSignal, "reload.signal", "Process ID to signal for reloads")
	cmd.PersistentFlags().StringVar(&c.reloadMethod, "reload.method", http.MethodPost, "Method to use for reload URL")
	cmd.PersistentFlags().BoolVar(&c.debug, "debug", false, "Enable debug logging")

	// require a url
	cmd.MarkPersistentFlagRequired("url")

	// dont allow different reload options together
	cmd.MarkFlagsMutuallyExclusive("reload.url", "reload.pid", "reload.pidfile", "reload.socket")

	// a payload makes no sense for pid/pidfile based reloads
	cmd.MarkFlagsMutuallyExclusive("reload.payload", "reload.pid")
	cmd.MarkFlagsMutuallyExclusive("reload.payload", "reload.pidfile")

	// socket based reloads required a payload
	cmd.MarkFlagsRequiredTogether("reload.socket", "reload.payload")

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

	// set up reloader
	if c.reloadPid != 0 {
		reloader, err := reload.NewProcessReloader(c.reloadPid, c.reloadSignal.v)
		if err != nil {
			return err
		}

		if reloader.Pid() == os.Getpid() {
			c.logger.Warn("the pid selected for reload seems to be ours", "pid", reloader.Pid())
		}

		c.reloader = reloader
	} else if c.reloadPidfile != "" {
		reloader, err := reload.NewProcessReloaderFromPidfile(c.reloadPidfile, c.reloadSignal.v)
		if err != nil {
			return err
		}

		if reloader.Pid() == os.Getpid() {
			c.logger.Warn("the pid selected for reload seems to be ours", "pid", reloader.Pid())
		}

		c.reloader = reloader
	} else if c.reloadUrl != "" {
		var payload []byte

		// use payload if set
		if c.reloadPayload != "" {
			payload = []byte(c.reloadPayload)
		}

		// set up reloader
		reloader, err := reload.NewHTTPReloader(c.reloadUrl, c.reloadMethod, payload)
		if err != nil {
			return err
		}

		c.reloader = reloader
	} else if c.reloadSocket != "" {
		// set up unix socket based reloader
		reloader, err := reload.NewUnixSocketReloader(c.reloadSocket, []byte(c.reloadPayload))
		if err != nil {
			return err
		}

		c.reloader = reloader
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
	if c.reloader == nil {
		return nil
	}

	// do reload
	if err := c.reloader.Reload(); err != nil {
		c.logger.Error("reload of process failed", "error", err)

		return err
	}

	c.logger.Info("reload of process completed")

	return nil
}

func Execute(args []string) error {
	// Set up command
	root := &rootCommand{
		Command: simplecommand.New(
			"jwks-to-pem",
			"Retrieve keys from a JWKS URL and save as PEM encoded files",
			simplecommand.WithViper("jwks", strings.NewReplacer("-", "_", ".", "_")),
		),
	}

	// add child commands
	root.SubCommands = []simplecobra.Commander{
		&cronCommand{
			Command: simplecommand.New(
				"cron",
				"Run with a scheduled process",
				simplecommand.WithViper("jwks_cron", strings.NewReplacer("-", "_", ".", "_")),
			),
		},
	}

	// Set up simplecobra
	x, err := simplecobra.New(root)
	if err != nil {
		return err
	}

	// run things
	if _, err := x.Execute(context.Background(), args); err != nil {
		return err
	}

	return nil
}
