package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	utls "github.com/refraction-networking/utls"
	"github.com/raven-betanet/dual-cli/internal/tlsgen"
	"github.com/raven-betanet/dual-cli/internal/utils"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Global flags
var (
	configFile string
	logLevel   string
	logFormat  string
	verbose    bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "chrome-utls-gen",
		Short: "Chrome-Stable (N-2) uTLS Template Generator",
		Long: `A utility to generate a deterministic TLS ClientHello identical to Chrome 
Stable (N or N-2), verify it via JA3 fingerprint self-test, and auto-refresh 
when Chrome stable tags update.

Examples:
  # Generate ClientHello for latest Chrome stable
  chrome-utls-gen generate --output clienthello.bin

  # Test JA3 fingerprint against a server
  chrome-utls-gen ja3-test --target example.com:443

  # Update Chrome version templates
  chrome-utls-gen update

  # Show version information
  chrome-utls-gen version`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initializeGlobals()
		},
	}

	// Add global flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file (default is $HOME/.raven-betanet/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format (text, json)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Add version template
	rootCmd.SetVersionTemplate(`{{printf "%s\n" .Version}}`)

	// Add subcommands
	rootCmd.AddCommand(newGenerateCmd())
	rootCmd.AddCommand(newJA3TestCmd())
	rootCmd.AddCommand(newUpdateCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// initializeGlobals initializes global configuration and logging
func initializeGlobals() error {
	// Override log level if verbose is set
	if verbose {
		logLevel = "debug"
	}

	// Initialize logger
	loggerConfig := utils.LoggerConfig{
		Level:  utils.LogLevel(logLevel),
		Format: utils.LogFormat(logFormat),
	}
	
	logger := utils.NewLogger(loggerConfig)
	logger.WithComponent("chrome-utls-gen").Debug("Initialized logging")

	return nil
}

// newGenerateCmd creates the generate subcommand
func newGenerateCmd() *cobra.Command {
	var (
		outputFile    string
		chromeVersion string
		templateCache string
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate Chrome TLS ClientHello template",
		Long: `Generate a deterministic TLS ClientHello blob identical to Chrome Stable handshake bytes.
The generated template can be used for TLS fingerprinting and testing purposes.

The tool supports Chrome Stable (N) and Chrome Stable (N-2) versions.`,
		Example: `  # Generate ClientHello for latest Chrome stable
  chrome-utls-gen generate --output clienthello.bin

  # Generate for specific Chrome version
  chrome-utls-gen generate --version 120.0.6099.109 --output chrome120.bin

  # Use custom template cache directory
  chrome-utls-gen generate --cache ./templates --output clienthello.bin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(outputFile, chromeVersion, templateCache)
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "clienthello.bin", "output file for ClientHello binary blob")
	cmd.Flags().StringVar(&chromeVersion, "version", "", "specific Chrome version (default: latest stable)")
	cmd.Flags().StringVar(&templateCache, "cache", "", "template cache directory (default: ~/.raven-betanet/templates)")

	return cmd
}

// newJA3TestCmd creates the ja3-test subcommand
func newJA3TestCmd() *cobra.Command {
	var (
		target        string
		chromeVersion string
		timeout       string
		expectedJA3   string
	)

	cmd := &cobra.Command{
		Use:   "ja3-test",
		Short: "Test JA3 fingerprint against target server",
		Long: `Connect to a target server using Chrome TLS fingerprint and extract the JA3 fingerprint.
Verify that the fingerprint matches expected Chrome signatures.

This command helps validate that the generated ClientHello produces the correct JA3 fingerprint
when connecting to real servers.`,
		Example: `  # Test JA3 fingerprint against example.com
  chrome-utls-gen ja3-test --target example.com:443

  # Test with specific Chrome version
  chrome-utls-gen ja3-test --target example.com:443 --version 120.0.6099.109

  # Test with expected JA3 hash verification
  chrome-utls-gen ja3-test --target example.com:443 --expected cd08e31494f9531f560d64c695473da9

  # Test with custom timeout
  chrome-utls-gen ja3-test --target example.com:443 --timeout 30s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJA3Test(target, chromeVersion, timeout, expectedJA3)
		},
	}

	cmd.Flags().StringVarP(&target, "target", "t", "", "target server (host:port)")
	cmd.Flags().StringVar(&chromeVersion, "version", "", "Chrome version to use (default: latest stable)")
	cmd.Flags().StringVar(&timeout, "timeout", "10s", "connection timeout")
	cmd.Flags().StringVar(&expectedJA3, "expected", "", "expected JA3 hash for verification")

	cmd.MarkFlagRequired("target")

	return cmd
}

// newUpdateCmd creates the update subcommand
func newUpdateCmd() *cobra.Command {
	var (
		force         bool
		templateCache string
		dryRun        bool
	)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update Chrome version templates",
		Long: `Fetch the latest Chrome Stable versions and regenerate ClientHello templates.
This command checks for new Chrome releases and updates the cached templates accordingly.

The update process:
1. Fetches latest Chrome versions from Chromium API
2. Compares with cached versions
3. Regenerates templates if newer versions are found
4. Updates template cache and metadata`,
		Example: `  # Update templates if new Chrome versions are available
  chrome-utls-gen update

  # Force update even if versions haven't changed
  chrome-utls-gen update --force

  # Dry run to see what would be updated
  chrome-utls-gen update --dry-run

  # Use custom template cache directory
  chrome-utls-gen update --cache ./templates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(force, templateCache, dryRun)
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "force update even if versions haven't changed")
	cmd.Flags().StringVar(&templateCache, "cache", "", "template cache directory (default: ~/.raven-betanet/templates)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be updated without making changes")

	return cmd
}

// runGenerate handles the generate command
func runGenerate(outputFile, chromeVersion, templateCache string) error {
	logger := utils.NewLogger(utils.LoggerConfig{
		Level:  utils.LogLevel(logLevel),
		Format: utils.LogFormat(logFormat),
	}).WithComponent("generate")

	logger.Info("Starting ClientHello generation")

	// Initialize Chrome client and TLS generator
	chromeClient := tlsgen.NewChromeClient()
	tlsGenerator := tlsgen.NewTLSGenerator()

	// Determine which Chrome version to use
	var targetVersion *tlsgen.ChromeVersion
	var err error

	if chromeVersion != "" {
		// Parse specific version provided by user
		logger.WithField("version", chromeVersion).Debug("Using specified Chrome version")
		targetVersion, err = tlsgen.ParseVersion(chromeVersion)
		if err != nil {
			return fmt.Errorf("invalid Chrome version format: %w", err)
		}
	} else {
		// Fetch latest stable version
		logger.Debug("Fetching latest Chrome stable version")
		targetVersion, err = chromeClient.FetchLatestVersion()
		if err != nil {
			return fmt.Errorf("failed to fetch latest Chrome version: %w", err)
		}
		logger.WithField("version", targetVersion.String()).Info("Using latest Chrome stable version")
	}

	// Generate ClientHello template
	logger.WithField("version", targetVersion.String()).Info("Generating ClientHello template")
	template, err := tlsGenerator.GenerateTemplate(*targetVersion)
	if err != nil {
		return fmt.Errorf("failed to generate ClientHello template: %w", err)
	}

	// Write ClientHello bytes to output file
	logger.WithField("output", outputFile).Info("Writing ClientHello to file")
	if err := os.WriteFile(outputFile, template.Bytes, 0644); err != nil {
		return fmt.Errorf("failed to write ClientHello to file: %w", err)
	}

	// Cache the template if cache directory is specified
	if templateCache != "" {
		logger.WithField("cache", templateCache).Debug("Caching template")
		if err := cacheTemplate(template, templateCache); err != nil {
			logger.WithError(err).Warn("Failed to cache template")
		}
	}

	// Print generation summary
	fmt.Printf("✓ Generated ClientHello for Chrome %s\n", targetVersion.String())
	fmt.Printf("  Output file: %s (%d bytes)\n", outputFile, len(template.Bytes))
	fmt.Printf("  JA3 Hash: %s\n", template.JA3Hash)
	fmt.Printf("  JA3 String: %s\n", template.JA3String)
	fmt.Printf("  Generated at: %s\n", template.GeneratedAt.Format(time.RFC3339))

	return nil
}

// runJA3Test handles the ja3-test command
func runJA3Test(target, chromeVersion, timeout, expectedJA3 string) error {
	logger := utils.NewLogger(utils.LoggerConfig{
		Level:  utils.LogLevel(logLevel),
		Format: utils.LogFormat(logFormat),
	}).WithComponent("ja3-test")

	logger.Info("Starting JA3 fingerprint test")

	// Parse timeout duration
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout format: %w", err)
	}

	// Initialize Chrome client
	chromeClient := tlsgen.NewChromeClient()

	// Determine which Chrome version to use
	var targetVersion *tlsgen.ChromeVersion

	if chromeVersion != "" {
		// Parse specific version provided by user
		logger.WithField("version", chromeVersion).Debug("Using specified Chrome version")
		targetVersion, err = tlsgen.ParseVersion(chromeVersion)
		if err != nil {
			return fmt.Errorf("invalid Chrome version format: %w", err)
		}
	} else {
		// Fetch latest stable version
		logger.Debug("Fetching latest Chrome stable version")
		targetVersion, err = chromeClient.FetchLatestVersion()
		if err != nil {
			return fmt.Errorf("failed to fetch latest Chrome version: %w", err)
		}
		logger.WithField("version", targetVersion.String()).Info("Using latest Chrome stable version")
	}

	// Map Chrome version to ClientHelloID
	clientHelloID, err := mapChromeVersionToClientHelloID(*targetVersion)
	if err != nil {
		return fmt.Errorf("failed to map Chrome version to ClientHelloID: %w", err)
	}

	// Initialize JA3 calculator with custom timeout
	ja3Calculator := tlsgen.NewJA3CalculatorWithTimeout(timeoutDuration)

	// Test connection and extract JA3 fingerprint
	logger.WithFields(map[string]interface{}{
		"target":  target,
		"version": targetVersion.String(),
		"timeout": timeoutDuration,
	}).Info("Testing connection and extracting JA3 fingerprint")

	result, err := ja3Calculator.TestConnection(target, clientHelloID)
	if err != nil {
		return fmt.Errorf("failed to test connection: %w", err)
	}

	// Display connection results
	fmt.Printf("JA3 Fingerprint Test Results\n")
	fmt.Printf("============================\n\n")
	fmt.Printf("Target Server: %s\n", result.Target)
	fmt.Printf("Chrome Version: %s\n", targetVersion.String())
	fmt.Printf("Connection Timeout: %s\n", timeoutDuration)
	fmt.Printf("\n")

	if result.Connected {
		fmt.Printf("✓ Connection Status: SUCCESS\n")
		fmt.Printf("  Response Time: %v\n", result.ResponseTime)
		fmt.Printf("  TLS Version: %s\n", result.TLSVersion)
		fmt.Printf("  Cipher Suite: %s\n", result.CipherSuite)
		fmt.Printf("\n")
		fmt.Printf("JA3 Fingerprint Results:\n")
		fmt.Printf("  JA3 String: %s\n", result.JA3String)
		fmt.Printf("  JA3 Hash: %s\n", result.JA3Fingerprint)
	} else {
		fmt.Printf("✗ Connection Status: FAILED\n")
		fmt.Printf("  Error: %s\n", result.Error)
		fmt.Printf("  Response Time: %v\n", result.ResponseTime)
		fmt.Printf("\n")
		return fmt.Errorf("connection to target server failed: %s", result.Error)
	}

	// Verify JA3 fingerprint if expected value is provided
	if expectedJA3 != "" {
		fmt.Printf("\nJA3 Verification:\n")
		fmt.Printf("  Expected JA3: %s\n", expectedJA3)
		fmt.Printf("  Actual JA3:   %s\n", result.JA3Fingerprint)
		
		if strings.EqualFold(result.JA3Fingerprint, expectedJA3) {
			fmt.Printf("  Status: ✓ MATCH\n")
		} else {
			fmt.Printf("  Status: ✗ MISMATCH\n")
			return fmt.Errorf("JA3 fingerprint mismatch: expected %s, got %s", expectedJA3, result.JA3Fingerprint)
		}
	} else {
		// Compare against known Chrome JA3 hashes
		knownHashes := ja3Calculator.GetKnownChromeJA3Hashes()
		fmt.Printf("\nKnown Chrome JA3 Verification:\n")
		
		var matchFound bool
		var matchedVersion string
		
		for version, hashes := range knownHashes {
			if ja3Calculator.VerifyJA3Fingerprint(result.JA3Fingerprint, hashes) {
				matchFound = true
				matchedVersion = version
				break
			}
		}
		
		if matchFound {
			fmt.Printf("  Status: ✓ MATCHES known Chrome fingerprint (%s)\n", matchedVersion)
		} else {
			fmt.Printf("  Status: ⚠ UNKNOWN fingerprint (not in known Chrome signatures)\n")
			fmt.Printf("  Note: This may be expected for newer Chrome versions\n")
		}
	}

	// Display summary
	fmt.Printf("\nTest Summary:\n")
	if result.Connected {
		fmt.Printf("  Connection: ✓ Successful\n")
		fmt.Printf("  JA3 Extracted: ✓ %s\n", result.JA3Fingerprint)
		
		if expectedJA3 != "" {
			if strings.EqualFold(result.JA3Fingerprint, expectedJA3) {
				fmt.Printf("  Verification: ✓ Passed\n")
			} else {
				fmt.Printf("  Verification: ✗ Failed\n")
			}
		} else {
			fmt.Printf("  Verification: ℹ No expected JA3 provided\n")
		}
	} else {
		fmt.Printf("  Connection: ✗ Failed\n")
		fmt.Printf("  JA3 Extracted: ✗ N/A\n")
		fmt.Printf("  Verification: ✗ N/A\n")
	}

	logger.Info("JA3 fingerprint test completed")
	return nil
}

// mapChromeVersionToClientHelloID maps Chrome version to uTLS ClientHelloID
// This is a helper function extracted from the TLS generator for reuse
func mapChromeVersionToClientHelloID(version tlsgen.ChromeVersion) (utls.ClientHelloID, error) {
	// Map Chrome versions to appropriate uTLS fingerprints
	// This mapping is based on Chrome's TLS behavior patterns
	
	switch {
	case version.Major >= 133:
		// Chrome 133+ uses the latest fingerprint
		return utls.HelloChrome_133, nil
	case version.Major >= 131:
		// Chrome 131-132
		return utls.HelloChrome_131, nil
	case version.Major >= 120:
		// Chrome 120-130
		return utls.HelloChrome_120, nil
	case version.Major >= 115:
		// Chrome 115-119 with post-quantum support
		return utls.HelloChrome_115_PQ, nil
	case version.Major >= 106:
		// Chrome 106-114 with extension shuffling
		return utls.HelloChrome_106_Shuffle, nil
	case version.Major >= 102:
		// Chrome 102-105
		return utls.HelloChrome_102, nil
	case version.Major >= 100:
		// Chrome 100-101
		return utls.HelloChrome_100, nil
	case version.Major >= 96:
		// Chrome 96-99
		return utls.HelloChrome_96, nil
	case version.Major >= 87:
		// Chrome 87-95
		return utls.HelloChrome_87, nil
	case version.Major >= 83:
		// Chrome 83-86
		return utls.HelloChrome_83, nil
	case version.Major >= 72:
		// Chrome 72-82
		return utls.HelloChrome_72, nil
	case version.Major >= 70:
		// Chrome 70-71
		return utls.HelloChrome_70, nil
	default:
		// Fallback to Chrome 100 for older versions
		return utls.HelloChrome_100, nil
	}
}

// runUpdate handles the update command
func runUpdate(force bool, templateCache string, dryRun bool) error {
	logger := utils.NewLogger(utils.LoggerConfig{
		Level:  utils.LogLevel(logLevel),
		Format: utils.LogFormat(logFormat),
	}).WithComponent("update")

	logger.Info("Starting Chrome version update process")

	// Determine template cache directory
	if templateCache == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get user home directory: %w", err)
		}
		templateCache = fmt.Sprintf("%s/.raven-betanet/templates", homeDir)
	}

	// Initialize components
	chromeClient := tlsgen.NewChromeClient()
	tlsGenerator := tlsgen.NewTLSGenerator()
	
	// Use a version cache in the same directory as template cache for isolation
	versionCacheDir := filepath.Join(templateCache, "..", ".cache", "chrome-versions")
	versionCache := tlsgen.NewVersionCacheManagerWithPath(versionCacheDir, 24*time.Hour)

	// Fetch latest Chrome versions from API
	logger.Info("Fetching latest Chrome versions from API")
	latestVersions, err := chromeClient.FetchLatestVersions()
	if err != nil {
		return fmt.Errorf("failed to fetch latest Chrome versions: %w", err)
	}

	if len(latestVersions) == 0 {
		return fmt.Errorf("no Chrome versions found from API")
	}

	logger.WithField("count", len(latestVersions)).Info("Fetched Chrome versions from API")

	// Get cached versions for comparison
	cachedVersions, isValid, err := versionCache.GetCachedVersions()
	if err != nil {
		logger.WithError(err).Warn("Failed to read cached versions, treating as empty cache")
		cachedVersions = []tlsgen.ChromeVersion{}
	}

	// Determine if update is needed
	updateNeeded := force || !isValid || len(cachedVersions) == 0
	var versionsToUpdate []tlsgen.ChromeVersion

	if !updateNeeded {
		// Check if we have newer versions
		for _, latest := range latestVersions {
			found := false
			for _, cached := range cachedVersions {
				if latest.Equal(cached) {
					found = true
					break
				}
			}
			if !found {
				versionsToUpdate = append(versionsToUpdate, latest)
				updateNeeded = true
			}
		}
	} else {
		// Force update or cache invalid - update all versions
		versionsToUpdate = latestVersions
	}

	// Display update status
	fmt.Printf("Chrome Version Update Status\n")
	fmt.Printf("============================\n\n")
	fmt.Printf("Template Cache Directory: %s\n", templateCache)
	fmt.Printf("Force Update: %t\n", force)
	fmt.Printf("Dry Run: %t\n", dryRun)
	fmt.Printf("\n")

	if len(cachedVersions) > 0 {
		fmt.Printf("Cached Versions (%d):\n", len(cachedVersions))
		for i, version := range cachedVersions {
			if i < 5 { // Show first 5 cached versions
				fmt.Printf("  - %s (%s)\n", version.String(), version.Date.Format("2006-01-02"))
			}
		}
		if len(cachedVersions) > 5 {
			fmt.Printf("  ... and %d more\n", len(cachedVersions)-5)
		}
		fmt.Printf("\n")
	} else {
		fmt.Printf("Cached Versions: None\n\n")
	}

	fmt.Printf("Latest Versions from API (%d):\n", len(latestVersions))
	for i, version := range latestVersions {
		if i < 5 { // Show first 5 latest versions
			fmt.Printf("  - %s (%s)\n", version.String(), version.Date.Format("2006-01-02"))
		}
	}
	if len(latestVersions) > 5 {
		fmt.Printf("  ... and %d more\n", len(latestVersions)-5)
	}
	fmt.Printf("\n")

	if !updateNeeded {
		fmt.Printf("✓ Templates are up to date - no update needed\n")
		return nil
	}

	fmt.Printf("Update Required: %t\n", updateNeeded)
	fmt.Printf("Versions to Update: %d\n", len(versionsToUpdate))
	fmt.Printf("\n")

	if dryRun {
		fmt.Printf("DRY RUN - Would update the following versions:\n")
		for _, version := range versionsToUpdate {
			fmt.Printf("  - %s (%s)\n", version.String(), version.Date.Format("2006-01-02"))
		}
		fmt.Printf("\nDry run complete - no changes made\n")
		return nil
	}

	// Perform actual update
	fmt.Printf("Updating Chrome version templates...\n")
	
	// Focus on stable (N) and stable (N-2) versions for template generation
	currentStable, previousStable, err := chromeClient.FetchStableVersions()
	if err != nil {
		return fmt.Errorf("failed to fetch stable Chrome versions: %w", err)
	}

	templatesUpdated := 0
	versionsToGenerate := []*tlsgen.ChromeVersion{currentStable, previousStable}

	for _, version := range versionsToGenerate {
		logger.WithField("version", version.String()).Info("Generating template for Chrome version")
		
		fmt.Printf("  Generating template for Chrome %s...", version.String())
		
		template, err := tlsGenerator.GenerateTemplate(*version)
		if err != nil {
			fmt.Printf(" ✗ FAILED\n")
			logger.WithError(err).WithField("version", version.String()).Error("Failed to generate template")
			continue
		}

		// Cache the template
		if err := cacheTemplate(template, templateCache); err != nil {
			fmt.Printf(" ✗ CACHE FAILED\n")
			logger.WithError(err).WithField("version", version.String()).Error("Failed to cache template")
			continue
		}

		fmt.Printf(" ✓ SUCCESS\n")
		templatesUpdated++
	}

	// Update version cache
	if err := versionCache.CacheVersions(latestVersions); err != nil {
		logger.WithError(err).Warn("Failed to update version cache")
	}

	// Display update summary
	fmt.Printf("\nUpdate Summary:\n")
	fmt.Printf("  Templates Generated: %d\n", templatesUpdated)
	fmt.Printf("  Cache Directory: %s\n", templateCache)
	fmt.Printf("  Version Cache Updated: %t\n", err == nil)
	
	if templatesUpdated > 0 {
		fmt.Printf("  Current Stable: %s\n", currentStable.String())
		fmt.Printf("  Previous Stable: %s\n", previousStable.String())
	}

	fmt.Printf("\n✓ Update process completed successfully\n")
	
	logger.WithField("templates_updated", templatesUpdated).Info("Update process completed")
	return nil
}

// cacheTemplate caches a ClientHello template to the specified directory
func cacheTemplate(template *tlsgen.ClientHelloTemplate, cacheDir string) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Create template filename based on Chrome version
	filename := fmt.Sprintf("chrome_%s.json", template.Version.String())
	cachePath := fmt.Sprintf("%s/%s", cacheDir, filename)

	// Marshal template to JSON
	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal template: %w", err)
	}

	// Write template to cache file
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write template cache: %w", err)
	}

	return nil
}