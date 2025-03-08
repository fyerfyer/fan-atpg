package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/fyerfyer/fan-atpg/pkg/algorithm"
	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
)

func main() {
	// Parse command-line arguments
	circuitFile := flag.String("circuit", "", "Circuit file in BENCH format")
	faultStr := flag.String("fault", "", "Fault to test (e.g., 'net42/1' for net42 stuck-at-1)")
	allFaults := flag.Bool("all", false, "Generate tests for all faults")
	outputFile := flag.String("output", "tests.txt", "Output file for test vectors")
	compactTests := flag.Bool("compact", true, "Compact test vectors")
	verbose := flag.Bool("verbose", false, "Verbose output")
	logFile := flag.String("log", "", "Log file (default: stdout)")
	flag.Parse()

	// Configure logger
	logLevel := utils.InfoLevel
	if *verbose {
		logLevel = utils.DebugLevel
	}

	var logger *utils.Logger
	var err error

	if *logFile != "" {
		logger, err = utils.NewFileLogger(logLevel, *logFile)
		if err != nil {
			fmt.Printf("Error creating log file: %v\n", err)
			os.Exit(1)
		}
	} else {
		logger = utils.NewLogger(logLevel)
	}

	// Check required arguments
	if *circuitFile == "" {
		fmt.Println("Error: Circuit file is required")
		flag.Usage()
		os.Exit(1)
	}

	if !*allFaults && *faultStr == "" {
		fmt.Println("Error: Either specify a fault or use -all flag")
		flag.Usage()
		os.Exit(1)
	}

	// Parse circuit file
	logger.Info("Parsing circuit from %s", *circuitFile)
	c, err := utils.ParseBenchFile(*circuitFile)
	if err != nil {
		logger.Error("Failed to parse circuit: %v", err)
		os.Exit(1)
	}

	// Create FAN algorithm instance
	fan := algorithm.NewFan(c, logger)

	var testVectors map[string]map[string]circuit.LogicValue

	if *allFaults {
		// Generate tests for all faults
		logger.Info("Generating tests for all faults")
		testVectors, err = fan.GenerateTestsForAllFaults()
		if err != nil {
			logger.Error("Error generating tests: %v", err)
			os.Exit(1)
		}
	} else {
		// Generate test for specific fault
		logger.Info("Generating test for fault: %s", *faultStr)

		lineName, faultTypeStr, found := strings.Cut(*faultStr, "/")
		if !found {
			logger.Error("Invalid fault format: %s (expected: net/value)", *faultStr)
			os.Exit(1)
		}

		// Find line by name
		var faultLine *circuit.Line
		for _, line := range c.Lines {
			if line.Name == lineName {
				faultLine = line
				break
			}
		}

		if faultLine == nil {
			logger.Error("Line not found: %s", lineName)
			os.Exit(1)
		}

		// Parse fault type
		var faultType circuit.LogicValue
		if faultTypeStr == "0" {
			faultType = circuit.Zero
		} else if faultTypeStr == "1" {
			faultType = circuit.One
		} else {
			logger.Error("Invalid fault type: %s (expected: 0 or 1)", faultTypeStr)
			os.Exit(1)
		}

		// Generate test
		test, err := fan.FindTest(faultLine, faultType)
		if err != nil {
			logger.Error("Failed to find test: %v", err)
			os.Exit(1)
		}

		testVectors = make(map[string]map[string]circuit.LogicValue)
		testVectors[*faultStr] = test
	}

	// Compact tests if requested
	var finalTests []map[string]circuit.LogicValue
	if *compactTests && len(testVectors) > 1 {
		logger.Info("Compacting test vectors")
		finalTests = fan.CompactTests(testVectors)
	} else {
		// Convert map to slice
		finalTests = make([]map[string]circuit.LogicValue, 0, len(testVectors))
		for _, test := range testVectors {
			finalTests = append(finalTests, test)
		}
	}

	// Write output file
	logger.Info("Writing %d test vectors to %s", len(finalTests), *outputFile)
	err = utils.WriteTestVectors(*outputFile, finalTests)
	if err != nil {
		logger.Error("Error writing test vectors: %v", err)
		os.Exit(1)
	}

	// Print summary
	logger.Info("ATPG complete")
	logger.Info("Circuit: %s", c.Name)
	logger.Info("Gates: %d", len(c.Gates))
	logger.Info("Lines: %d", len(c.Lines))
	logger.Info("Primary inputs: %d", len(c.Inputs))
	logger.Info("Primary outputs: %d", len(c.Outputs))
	logger.Info("Tests generated: %d", len(finalTests))
}
