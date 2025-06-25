package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/andrewheberle/simplecommand"
	"github.com/bep/simplecobra"
	"github.com/go-co-op/gocron/v2"
)

type cronCommand struct {
	cronPattern string

	logger *slog.Logger

	*simplecommand.Command
}

func (c *cronCommand) Init(cd *simplecobra.Commandeer) error {
	if err := c.Command.Init(cd); err != nil {
		return err
	}

	// command line flags
	cmd := cd.CobraCommand
	cmd.Flags().StringVar(&c.cronPattern, "schedule", "", "Cron pattern for scheduling check of JWKS")

	// require a cron pattern
	cmd.MarkFlagRequired("schedule")

	return nil
}

func (c *cronCommand) PreRun(this, runner *simplecobra.Commandeer) error {
	if err := c.Command.PreRun(this, runner); err != nil {
		return err
	}

	// inherit logger from root
	root, ok := this.Root.Command.(*rootCommand)
	if !ok {
		return fmt.Errorf("could not access root command")
	}
	c.logger = root.logger

	return nil
}

func (c *cronCommand) Run(ctx context.Context, cd *simplecobra.Commandeer, args []string) error {
	// set up scheduler
	s, err := gocron.NewScheduler()
	if err != nil {
		return err
	}

	// add job to scheduler
	if _, err := s.NewJob(
		gocron.CronJob(c.cronPattern, false),
		gocron.NewTask(cd.Root.Command.Run, ctx, cd, args),
	); err != nil {
		return err
	}

	// start scheduler
	s.Start()

	// let them know we started
	c.logger.Info("starting cron process", "schedule", c.cronPattern)

	// wait until we are done
	<-ctx.Done()

	return s.Shutdown()
}
