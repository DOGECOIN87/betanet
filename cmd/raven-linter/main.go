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
		Use:   "raven-linter",
		Short: "Raven Betanet 1.1 Spec-Compliance Linter CLI",
		Long: `A command-line utility to run all 11 compliance checks described in §11 
of the Raven Betanet 1.1 spec against a candidate binary, generate a Software 
Bill of Materials (SBOM), and integrate into CI/CD via GitHub Actions.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
	}

	// Add version template
	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}