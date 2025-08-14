package main

import (
	"fmt"
	"log"
	"os"

	"github.com/raven-betanet/dual-cli/internal/checks"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: go run test_binary_analysis.go <binary_path>")
	}

	binaryPath := os.Args[1]

	// Create registry and register binary analysis checks
	registry := checks.NewCheckRegistry()
	
	binaryChecks := []checks.ComplianceCheck{
		&checks.FileSignatureCheck{},
		&checks.BinaryMetadataCheck{},
		&checks.DependencyAnalysisCheck{},
		&checks.BinaryFormatCheck{},
	}

	for _, check := range binaryChecks {
		if err := registry.Register(check); err != nil {
			log.Fatalf("Failed to register check %s: %v", check.ID(), err)
		}
	}

	// Run all checks
	runner := checks.NewCheckRunner(registry)
	report, err := runner.RunAll(binaryPath)
	if err != nil {
		log.Fatalf("Failed to run checks: %v", err)
	}

	// Print results
	fmt.Printf("Binary Analysis Report for: %s\n", report.BinaryPath)
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
				fmt.Printf("    %s: %v\n", key, value)
			}
		}
		fmt.Println()
	}

	if report.IsReportPassing() {
		fmt.Println("✅ All binary analysis checks passed!")
	} else {
		fmt.Println("❌ Some binary analysis checks failed!")
		os.Exit(1)
	}
}