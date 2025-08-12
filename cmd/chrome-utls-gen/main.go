package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "chrome-utls-gen",
		Short: "Chrome-Stable (N-2) uTLS Template Generator",
		Long: `A utility to generate a deterministic TLS ClientHello identical to Chrome 
Stable (N or N-2), verify it via JA3 fingerprint self-test, and auto-refresh 
when Chrome stable tags update.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	}

	// Add version template
	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}