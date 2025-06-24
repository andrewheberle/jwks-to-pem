package cmd

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"os"
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

	logger *slog.Logger

	*simplecommand.Command
}

func (c *rootCommand) Init(cd *simplecobra.Commandeer) error {
	if err := c.Command.Init(cd); err != nil {
		return err
	}

	// command line flags
	cmd := cd.CobraCommand
	cmd.Flags().StringVarP(&c.jwksUrl, "url", "u", "", "URL for JSON Web Key Set (JWKS)")
	cmd.Flags().StringVarP(&c.outputDir, "out", "o", "", "Output directory")
	cmd.Flags().StringVarP(&c.outputPattern, "pattern", "p", "{{ .KeyID }}.pem", "Output pattern")
	cmd.Flags().DurationVar(&c.timeout, "timeout", time.Second*5, "Timeout to retrieve JWKS")
	cmd.Flags().BoolVar(&c.debug, "debug", false, "Enable debug logging")

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

	// check if any changes were written
	if !changed {
		c.logger.Info("no changes to keys")
	}

	return nil
}

func Execute(args []string) error {
	// Set up command
	command := &rootCommand{
		Command: simplecommand.New("jwks-to-pem", "Retrieve keys from a JWKS URL and save as PEM encoded files"),
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
