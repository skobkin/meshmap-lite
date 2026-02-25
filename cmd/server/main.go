package main

import (
	"flag"
	"fmt"
	"os"

	"meshmap-lite/internal/app"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "path to YAML config")
	flag.Parse()
	if err := app.Run(configPath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
