package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/kilan558/hllv-geofences/data"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: configcheck <config.yml> [config.yml ...]")
		os.Exit(2)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_ = godotenv.Load()
	failed := false
	for _, path := range os.Args[1:] {
		config, err := data.NewConfig(path, logger)
		if err == nil {
			err = config.ApplyServerEnvironment()
		}
		if err == nil {
			err = config.Validate()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			failed = true
			continue
		}
		fmt.Printf("%s: valid\n", path)
	}
	if failed {
		os.Exit(1)
	}
}
