package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/raven-betanet/dual-cli/internal/checks"
	"github.com/raven-betanet/dual-cli/internal/sbom"
	"github.com/raven-betanet/dual-cli/internal/utils"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// CLI flags
var (
	outputFormat string
	logLevel     string
	verbose      bool
	sbomFormat   string
	sbomOutput   string
	generateSBOM bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "raven-linter",
		Short: "Raven Betanet 1.1 Spec-Compliance Linter CLI",
		Long: `A command-line utility to run all 11 compliance checks described in §11 
of the Raven Betanet 1.1 spec against a candidate binary, generate a Software 
Bill of Materials (SBOM), and integrate into CI/CD via GitHub Actions.

Examples:
  raven-linter check ./my-binary
  raven-linter check ./my-binary --format json
  raven-linter check ./my-binary --format text --verbose`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logger based on flags
			loggerConfig := utils.LoggerConfig{
				Level:  utils.LogLevel(logLevel),
				Format: utils.LogFormatText,
			}
			if verbose {
				loggerConfig.Level = utils.LogLevelDebug
			}
			logger := utils.NewLogger(loggerConfig)
			
			// Store logger in context for use by subcommands
			cmd.SetContext(utils.WithLogger(cmd.Context(), logger))
			return nil
		},
	}

	// Add version template
	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)

	// Add persistent flags
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Set log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output (equivalent to --log-level debug)")

	// Add subcommands
	rootCmd.AddCommand(newCheckCommand())

	// Show help when run without arguments
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		cmd.Help()
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// newCheckCommand creates the check subcommand
func newCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check <binary-path>",
		Short: "Run compliance checks against a binary",
		Long: `Run all 11 compliance checks from §11 of the Raven Betanet 1.1 spec 
against the specified binary and generate a compliance report.

The command will execute all compliance checks and output the results in the 
specified format. If any check fails, the command will exit with status code 1.

SBOM generation can be enabled to create a Software Bill of Materials alongside
the compliance checks.

Examples:
  raven-linter check ./my-binary
  raven-linter check ./my-binary --format json
  raven-linter check ./my-binary --format text --sbom
  raven-linter check ./my-binary --sbom --sbom-format cyclonedx --sbom-output ./sbom.json`,
		Args: cobra.ExactArgs(1),
		RunE: runCheckCommand,
	}

	// Add command-specific flags
	cmd.Flags().StringVarP(&outputFormat, "format", "f", "text", "Output format (json, text)")
	cmd.Flags().BoolVar(&generateSBOM, "sbom", false, "Generate Software Bill of Materials (SBOM)")
	cmd.Flags().StringVar(&sbomFormat, "sbom-format", "cyclonedx", "SBOM format (cyclonedx, spdx)")
	cmd.Flags().StringVar(&sbomOutput, "sbom-output", "sbom.json", "SBOM output file path")

	return cmd
}

// runCheckCommand executes the check command
func runCheckCommand(cmd *cobra.Command, args []string) error {
	binaryPath := args[0]
	
	// Get logger from context
	logger := utils.LoggerFromContext(cmd.Context())
	if logger == nil {
		logger = utils.NewDefaultLogger()
	}

	// Validate output format
	if !isValidOutputFormat(outputFormat) {
		return fmt.Errorf("invalid output format: %s (supported: json, text)", outputFormat)
	}

	// Validate SBOM format if SBOM generation is enabled
	if generateSBOM {
		if !isValidSBOMFormat(sbomFormat) {
			return fmt.Errorf("invalid SBOM format: %s (supported: cyclonedx, spdx)", sbomFormat)
		}
		
		// Validate SBOM output path
		if sbomOutput == "" {
			return fmt.Errorf("SBOM output path cannot be empty when SBOM generation is enabled")
		}
		
		// Convert to absolute path
		absSBOMPath, err := filepath.Abs(sbomOutput)
		if err != nil {
			return fmt.Errorf("failed to resolve SBOM output path: %w", err)
		}
		sbomOutput = absSBOMPath
	}

	// Validate binary path
	absPath, err := filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to resolve binary path: %w", err)
	}

	logger.WithComponent("check").Infof("Running compliance checks on binary: %s", absPath)
	if generateSBOM {
		logger.WithComponent("check").Infof("SBOM generation enabled: format=%s, output=%s", sbomFormat, sbomOutput)
	}

	// Create check registry and register all checks
	registry := checks.NewCheckRegistry()
	if err := registerAllChecks(registry); err != nil {
		return fmt.Errorf("failed to register checks: %w", err)
	}

	logger.WithComponent("check").Debugf("Registered %d compliance checks", registry.Count())

	// Create check runner
	runner := checks.NewCheckRunner(registry)
	
	// Show progress if not in JSON output mode
	if strings.ToLower(outputFormat) != "json" {
		fmt.Printf("Running %d compliance checks", registry.Count())
		if generateSBOM {
			fmt.Printf(" and generating SBOM")
		}
		fmt.Printf("...\n")
	}
	
	// Run compliance checks and SBOM generation concurrently
	var report *checks.ComplianceReport
	var sbomPath string
	var wg sync.WaitGroup
	var checkErr, sbomErr error
	
	// Start compliance checks
	wg.Add(1)
	go func() {
		defer wg.Done()
		report, checkErr = runner.RunAll(absPath)
	}()
	
	// Start SBOM generation if enabled
	if generateSBOM {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sbomPath, sbomErr = generateSBOMFile(absPath, sbomFormat, sbomOutput, logger)
		}()
	}
	
	// Wait for both operations to complete
	wg.Wait()
	
	// Check for errors
	if checkErr != nil {
		return fmt.Errorf("failed to run compliance checks: %w", checkErr)
	}
	
	if generateSBOM && sbomErr != nil {
		logger.WithComponent("sbom").Warnf("SBOM generation failed: %v", sbomErr)
		// Don't fail the entire command if SBOM generation fails, just warn
	} else if generateSBOM && sbomPath != "" {
		// Add SBOM path to report
		report.SBOMPath = sbomPath
		logger.WithComponent("sbom").Infof("SBOM generated successfully: %s", sbomPath)
	}

	logger.WithComponent("check").Infof("Completed %d checks in %v", report.TotalChecks, report.Duration)

	// Output results in requested format
	if err := outputReport(report, outputFormat); err != nil {
		return fmt.Errorf("failed to output report: %w", err)
	}

	// Exit with error code if any checks failed
	if !report.IsReportPassing() {
		logger.WithComponent("check").Warnf("Compliance check failed: %d/%d checks passed", report.PassedChecks, report.TotalChecks)
		os.Exit(1)
	}

	logger.WithComponent("check").Infof("All compliance checks passed: %d/%d", report.PassedChecks, report.TotalChecks)
	return nil
}

// registerAllChecks registers all available compliance checks
func registerAllChecks(registry *checks.CheckRegistry) error {
	// Binary analysis checks (checks 1-4)
	if err := registry.Register(&checks.FileSignatureCheck{}); err != nil {
		return fmt.Errorf("failed to register file signature check: %w", err)
	}
	
	if err := registry.Register(&checks.BinaryMetadataCheck{}); err != nil {
		return fmt.Errorf("failed to register binary metadata check: %w", err)
	}
	
	if err := registry.Register(&checks.DependencyAnalysisCheck{}); err != nil {
		return fmt.Errorf("failed to register dependency analysis check: %w", err)
	}
	
	if err := registry.Register(&checks.BinaryFormatCheck{}); err != nil {
		return fmt.Errorf("failed to register binary format check: %w", err)
	}
	
	// Cryptographic validation checks (checks 5-8)
	if err := registry.Register(&checks.CertificateValidationCheck{}); err != nil {
		return fmt.Errorf("failed to register certificate validation check: %w", err)
	}
	
	if err := registry.Register(&checks.SignatureVerificationCheck{}); err != nil {
		return fmt.Errorf("failed to register signature verification check: %w", err)
	}
	
	if err := registry.Register(&checks.HashIntegrityCheck{}); err != nil {
		return fmt.Errorf("failed to register hash integrity check: %w", err)
	}
	
	if err := registry.Register(&checks.EncryptionStandardCheck{}); err != nil {
		return fmt.Errorf("failed to register encryption standard check: %w", err)
	}
	
	// Security and metadata checks (checks 9-11)
	if err := registry.Register(&checks.SecurityFlagValidationCheck{}); err != nil {
		return fmt.Errorf("failed to register security flag validation check: %w", err)
	}
	
	if err := registry.Register(&checks.VersionInformationCheck{}); err != nil {
		return fmt.Errorf("failed to register version information check: %w", err)
	}
	
	if err := registry.Register(&checks.LicenseComplianceCheck{}); err != nil {
		return fmt.Errorf("failed to register license compliance check: %w", err)
	}
	
	return nil
}

// isValidOutputFormat checks if the output format is supported
func isValidOutputFormat(format string) bool {
	switch strings.ToLower(format) {
	case "json", "text":
		return true
	default:
		return false
	}
}

// isValidSBOMFormat checks if the SBOM format is supported
func isValidSBOMFormat(format string) bool {
	switch strings.ToLower(format) {
	case "cyclonedx", "spdx":
		return true
	default:
		return false
	}
}

// generateSBOMFile generates an SBOM file for the given binary
func generateSBOMFile(binaryPath, format, outputPath string, logger *utils.Logger) (string, error) {
	// Create SBOM generator
	generator := sbom.NewGenerator()
	
	// Parse SBOM format
	var sbomFormat sbom.SBOMFormat
	switch strings.ToLower(format) {
	case "cyclonedx":
		sbomFormat = sbom.CycloneDX
	case "spdx":
		sbomFormat = sbom.SPDX
	default:
		return "", fmt.Errorf("unsupported SBOM format: %s", format)
	}
	
	logger.WithComponent("sbom").Debugf("Generating %s SBOM for binary: %s", format, binaryPath)
	
	// Generate SBOM
	sbomData, err := generator.Generate(binaryPath, sbomFormat)
	if err != nil {
		return "", fmt.Errorf("failed to generate SBOM: %w", err)
	}
	
	logger.WithComponent("sbom").Debugf("Generated SBOM with %d components", sbomData.GetComponentCount())
	
	// Write SBOM to file
	err = generator.WriteToFile(sbomData, outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to write SBOM file: %w", err)
	}
	
	return outputPath, nil
}

// outputReport outputs the compliance report in the specified format
func outputReport(report *checks.ComplianceReport, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return outputJSONReport(report)
	case "text":
		return outputTextReport(report)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// outputJSONReport outputs the report in JSON format
func outputJSONReport(report *checks.ComplianceReport) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// outputTextReport outputs the report in human-readable text format
func outputTextReport(report *checks.ComplianceReport) error {
	fmt.Printf("Raven Betanet 1.1 Compliance Report\n")
	fmt.Printf("===================================\n\n")
	
	fmt.Printf("Binary: %s\n", report.BinaryPath)
	fmt.Printf("Hash: %s\n", report.BinaryHash)
	fmt.Printf("Timestamp: %s\n", report.Timestamp.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("Duration: %v\n\n", report.Duration)
	
	fmt.Printf("Summary: %d/%d checks passed\n", report.PassedChecks, report.TotalChecks)
	if report.SBOMPath != "" {
		fmt.Printf("SBOM: %s\n", report.SBOMPath)
	}
	fmt.Printf("\n")
	
	// Create tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "CHECK ID\tSTATUS\tDESCRIPTION\tDETAILS\n")
	fmt.Fprintf(w, "--------\t------\t-----------\t-------\n")
	
	for _, result := range report.Results {
		status := strings.ToUpper(result.Status)
		if result.Status == "pass" {
			status = "✓ PASS"
		} else {
			status = "✗ FAIL"
		}
		
		details := result.Details
		if len(details) > 50 {
			details = details[:47] + "..."
		}
		
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", result.ID, status, result.Description, details)
	}
	
	w.Flush()
	
	// Show failed checks details if any
	if report.FailedChecks > 0 {
		fmt.Printf("\nFailed Checks Details:\n")
		fmt.Printf("======================\n")
		for _, result := range report.Results {
			if result.Status == "fail" {
				fmt.Printf("\n%s: %s\n", result.ID, result.Description)
				fmt.Printf("Details: %s\n", result.Details)
				if len(result.Metadata) > 0 {
					fmt.Printf("Metadata:\n")
					for key, value := range result.Metadata {
						fmt.Printf("  %s: %v\n", key, value)
					}
				}
			}
		}
	}
	
	return nil
}