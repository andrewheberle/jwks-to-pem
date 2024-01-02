package main

import (
	"log/slog"
	"os"
	"text/template"
	"time"

	"github.com/andrewheberle/jwks-to-pem/internal/pkg/jwks"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	// command line flags
	pflag.String("jwks", "", "URL for JSON Web Key Set (JWKS)")
	pflag.String("dir", "", "Output directory")
	pflag.String("pattern", "{{ .KeyID }}.pem", "Output pattern")
	pflag.Duration("timeout", time.Second*5, "Timeout to retrieve JWKS")
	pflag.Bool("debug", false, "Enable debug logging")
	pflag.Parse()

	// bind to viper
	viper.BindPFlags(pflag.CommandLine)

	// set up logging
	logLevel := new(slog.LevelVar)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})))
	if viper.GetBool("debug") {
		logLevel.Set(slog.LevelDebug)
	}
	slog.Debug("command line flags", "jwks", viper.GetString("jwks"), "dir", viper.GetString("dir"), "pattern", viper.GetString("pattern"), "timeout", viper.GetDuration("timeout"))

	// parse template early to fail immediately
	_, err := template.New("pattern").Parse(viper.GetString("pattern"))
	if err != nil {
		slog.Error("problem parsing pattern", "error", err)
		os.Exit(1)
	}

	// fetch JWKS
	j, err := jwks.GetJWKS(viper.GetString("jwks"), viper.GetDuration("timeout"))
	if err != nil {
		slog.Error("problem fetching JWKS", "error", err)
		os.Exit(1)
	}

	// write keys based on pattern
	changed, err := j.WriteKeys(viper.GetString("pattern"), viper.GetString("dir"))
	if err != nil {
		slog.Error("problem processing keys", "error", err)
		os.Exit(1)
	}

	// check if any changes were written
	if !changed {
		slog.Info("no changes to keys")
		os.Exit(0)
	}
}
