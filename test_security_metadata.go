package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/raven-betanet/dual-cli/internal/checks"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: go run test_security_metadata.go <binary_path>")
	}

	binaryPath := os.Args[1]

	// Create check registry and register the new security and metadata checks
	registry := checks.NewCheckRegistry()
	
	// Register the new checks
	securityCheck := &checks.SecurityFlagValidationCheck{}
	versionCheck := &checks.VersionInformationCheck{}
	licenseCheck := &checks.LicenseComplianceCheck{}
	
	if err := registry.Register(securityCheck); err != nil {
		log.Fatalf("Failed to register security check: %v", err)
	}
	
	if err := registry.Register(versionCheck); err != nil {
		log.Fatalf("Failed to register version check: %v", err)
	}
	
	if err := registry.Register(licenseCheck); err != nil {
		log.Fatalf("Failed to register license check: %v", err)
	}

	// Create check runner
	runner := checks.NewCheckRunner(registry)

	// Run all checks
	report, err := runner.RunAll(binaryPath)
	if err != nil {
		log.Fatalf("Failed to run checks: %v", err)
	}

	// Print report
	fmt.Printf("Security and Metadata Analysis Report for: %s\n", binaryPath)
	fmt.Printf("Binary Hash: %s\n", report.BinaryHash)
	fmt.Printf("Total Checks: %d\n", report.TotalChecks)
	fmt.Printf("Passed: %d, Failed: %d\n", report.PassedChecks, report.FailedChecks)
	fmt.Printf("Duration: %v\n\n", report.Duration)

	for _, result := range report.Results {
		fmt.Printf("Check: %s\n", result.ID)
		fmt.Printf("  Description: %s\n", result.Description)
		fmt.Printf("  Status: %s\n", result.Status)
		fmt.Printf("  Details: %s\n", result.Details)
		fmt.Printf("  Duration: %v\n", result.Duration)
		
		if len(result.Metadata) > 0 {
			fmt.Printf("  Metadata:\n")
			for key, value := range result.Metadata {
				// Pretty print metadata
				switch v := value.(type) {
				case []string:
					if len(v) > 0 {
						fmt.Printf("    %s: %v\n", key, v)
					} else {
						fmt.Printf("    %s: []\n", key)
					}
				case map[string]interface{}:
					jsonData, _ := json.MarshalIndent(v, "      ", "  ")
					fmt.Printf("    %s: %s\n", key, string(jsonData))
				default:
					fmt.Printf("    %s: %v\n", key, value)
				}
			}
		}
		fmt.Println()
	}

	if report.IsReportPassing() {
		fmt.Println("✅ All security and metadata checks passed!")
	} else {
		fmt.Println("❌ Some security and metadata checks failed!")
		os.Exit(1)
	}
}