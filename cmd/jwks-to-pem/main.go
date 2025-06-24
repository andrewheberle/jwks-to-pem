package main

import (
	"log/slog"
	"os"

	"github.com/andrewheberle/jwks-to-pem/pkg/cmd"
)

func main() {
	// run command
	if err := cmd.Execute(os.Args[1:]); err != nil {
		slog.Error("problem during execution", "error", err)
		os.Exit(1)
	}
}
