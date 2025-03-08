package algorithm

import (
	"fmt"
	"time"

	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
)

// Stats contains statistics about the FAN algorithm execution
type Stats struct {
	Decisions         int           // Number of decisions made
	Backtracks        int           // Number of backtracks performed
	Implications      int           // Number of implications performed
	TestsFound        int           // Number of tests found
	UndetectedFaults  int           // Number of undetected faults
	TotalTime         time.Duration // Total execution time
	MaxDecisionDepth  int           // Maximum decision tree depth reached
	UniqueAssignments int           // Number of unique line assignments
}

// Fan implements the FAN (FAN-Alternative-Node) algorithm for test pattern generation
type Fan struct {
	Circuit     *circuit.Circuit
	Logger      *utils.Logger
	Topology    *circuit.Topology
	Frontier    *Frontier
	Implication *Implication
	Backtrace   *Backtrace
	Decision    *Decision
	Sensitize   *Sensitization
	Stats       Stats
}

// NewFan creates a new FAN algorithm instance
func NewFan(c *circuit.Circuit, logger *utils.Logger) *Fan {
	topo := circuit.NewTopology(c)
	topo.Analyze()

	frontier := NewFrontier(c, logger)
	implication := NewImplication(c, frontier, topo, logger)
	backtrace := NewBacktrace(c, topo, frontier, implication, logger)
	decision := NewDecision(c, topo, frontier, implication, backtrace, logger)
	sensitize := NewSensitization(c, topo, implication, frontier, logger)

	return &Fan{
		Circuit:     c,
		Logger:      logger,
		Topology:    topo,
		Frontier:    frontier,
		Implication: implication,
		Backtrace:   backtrace,
		Decision:    decision,
		Sensitize:   sensitize,
	}
}

// FindTest generates a test for a specific fault (line stuck at value)
func (f *Fan) FindTest(faultSite *circuit.Line, faultType circuit.LogicValue) (map[string]circuit.LogicValue, error) {
	startTime := time.Now()
	f.Logger.Info("Starting test generation for %s stuck-at-%v", faultSite.Name, faultType)
	f.Logger.Indent()
	defer f.Logger.Outdent()

	// Reset circuit and statistics
	f.Circuit.Reset()
	f.resetStats()

	// Inject the fault
	f.Circuit.InjectFault(faultSite, faultType)
	f.Logger.Info("Injected fault: %s stuck-at-%v", faultSite.Name, faultType)

	// Initial implication
	_, err := f.Implication.ImplyValues()
	if err != nil {
		f.Logger.Error("Initial implication failed: %v", err)
		return nil, err
	}
	f.Stats.Implications++

	// Update frontiers
	f.Frontier.UpdateDFrontier()
	f.Frontier.UpdateJFrontier()

	// Main FAN algorithm loop
	found, err := f.runFanAlgorithm()
	if err != nil {
		f.Logger.Error("FAN algorithm failed: %v", err)
		return nil, err
	}

	// Update statistics
	f.Stats.TotalTime = time.Since(startTime)
	if found {
		f.Stats.TestsFound++
		f.logStats()
		testVector := f.Circuit.GetCurrentTest()
		f.Logger.Info("Test found: %v", testVector)
		return testVector, nil
	}

	f.Stats.UndetectedFaults++
	f.logStats()
	f.Logger.Info("No test possible for this fault")
	return nil, fmt.Errorf("no test possible for %s stuck-at-%v", faultSite.Name, faultType)
}

// runFanAlgorithm runs the main FAN algorithm loop
func (f *Fan) runFanAlgorithm() (bool, error) {
	maxIterations := 10000 // Safety limit to prevent infinite loops
	iterations := 0

	for iterations < maxIterations {
		iterations++
		f.Logger.Trace("FAN iteration %d", iterations)

		// Check if we've found a test
		if f.Circuit.CheckTestStatus() {
			f.Logger.Algorithm("Test found! D/D' propagated to primary output")
			return true, nil
		}

		// Make a decision
		success, err := f.Decision.MakeDecision()
		if err != nil {
			return false, err
		}
		f.Stats.Decisions++

		// Track max decision depth
		currentDepth := f.Decision.GetCurrentDecisionDepth()
		if currentDepth > f.Stats.MaxDecisionDepth {
			f.Stats.MaxDecisionDepth = currentDepth
		}

		if !success {
			f.Stats.Backtracks++
			f.Logger.Algorithm("Backtrack failed, no test possible")
			return false, nil
		}

		// Perform implication
		_, err = f.Implication.ImplyValues()
		if err != nil {
			continue // Conflict detected, will backtrack in next iteration
		}
		f.Stats.Implications++

		// Update frontiers
		f.Frontier.UpdateDFrontier()
		f.Frontier.UpdateJFrontier()

		// Apply unique sensitization if D-frontier has a single gate
		if len(f.Frontier.DFrontier) == 1 {
			f.Sensitize.ApplyUniqueSensitization(f.Frontier.DFrontier[0])
		}
	}

	f.Logger.Warning("FAN algorithm reached iteration limit (%d)", maxIterations)
	return false, fmt.Errorf("iteration limit reached")
}

// GenerateTestsForAllFaults generates tests for all possible faults in the circuit
func (f *Fan) GenerateTestsForAllFaults() (map[string]map[string]circuit.LogicValue, error) {
	startTime := time.Now()
	f.Logger.Info("Starting test generation for all faults")

	// Reset global stats
	f.resetStats()

	// Map to store test vectors for detected faults
	testVectors := make(map[string]map[string]circuit.LogicValue)
	faultCount := 0

	// Process each fault site (excluding primary outputs for simplicity)
	for _, line := range f.Circuit.Lines {
		// Skip primary outputs as fault sites for simplicity
		if line.Type == circuit.PrimaryOutput {
			continue
		}

		// Try stuck-at-0
		faultCount++
		faultKey := fmt.Sprintf("%s/0", line.Name)
		test, err := f.FindTest(line, circuit.Zero)
		if err == nil {
			testVectors[faultKey] = test
		}

		// Try stuck-at-1
		faultCount++
		faultKey = fmt.Sprintf("%s/1", line.Name)
		test, err = f.FindTest(line, circuit.One)
		if err == nil {
			testVectors[faultKey] = test
		}
	}

	// Update final stats
	f.Stats.TotalTime = time.Since(startTime)
	f.Logger.Info("Test generation completed for %d faults", faultCount)
	f.Logger.Info("Tests found: %d", f.Stats.TestsFound)
	f.Logger.Info("Undetected faults: %d", f.Stats.UndetectedFaults)
	f.Logger.Info("Fault coverage: %.2f%%", float64(f.Stats.TestsFound)/float64(faultCount)*100)

	return testVectors, nil
}

// Replace the problematic functions with these corrected versions:

// CompactTests removes redundant test vectors
func (f *Fan) CompactTests(testVectors map[string]map[string]circuit.LogicValue) []map[string]circuit.LogicValue {
	f.Logger.Info("Compacting test vectors")

	// Simple greedy algorithm to reduce test count
	var compactedTests []map[string]circuit.LogicValue
	coveredFaults := make(map[string]bool)

	// Convert map to slice for easier processing
	type testInfo struct {
		faultName string
		test      map[string]circuit.LogicValue
	}

	allTests := make([]testInfo, 0, len(testVectors))
	for faultName, test := range testVectors {
		allTests = append(allTests, testInfo{faultName, test})
	}

	// Keep adding tests until all faults are covered
	for len(coveredFaults) < len(testVectors) {
		bestTestIdx := -1
		bestCoverage := 0

		// Find test that covers the most new faults
		for i, testData := range allTests {
			// Skip if this test's fault is already covered
			if coveredFaults[testData.faultName] {
				continue
			}

			// Count how many new faults this test might cover
			newCoverage := 0

			// In real implementation, we would simulate each test on each fault
			// For simplicity, we'll use a simple similarity heuristic
			for j, otherTest := range allTests {
				if !coveredFaults[otherTest.faultName] {
					if i == j || testMatch(testData.test, otherTest.test) {
						newCoverage++
					}
				}
			}

			if newCoverage > bestCoverage {
				bestCoverage = newCoverage
				bestTestIdx = i
			}
		}

		if bestTestIdx == -1 || bestCoverage == 0 {
			break // No more faults can be covered
		}

		// Add the best test and mark covered faults
		bestTest := allTests[bestTestIdx].test
		compactedTests = append(compactedTests, bestTest)

		// Mark faults as covered
		for _, testData := range allTests {
			if !coveredFaults[testData.faultName] {
				if testData.faultName == allTests[bestTestIdx].faultName ||
					testMatch(bestTest, testData.test) {
					coveredFaults[testData.faultName] = true
				}
			}
		}
	}

	f.Logger.Info("Compacted %d tests to %d tests", len(testVectors), len(compactedTests))
	return compactedTests
}

// testMatch is a simplified check to see if two test vectors are similar enough
// that one could detect faults covered by the other
func testMatch(test1, test2 map[string]circuit.LogicValue) bool {
	// This is a simplified version - in reality, we would use fault simulation
	// Here we just check if the tests are identical
	if len(test1) != len(test2) {
		return false
	}

	for key, val1 := range test1 {
		val2, exists := test2[key]
		if !exists || val1 != val2 {
			return false
		}
	}

	return true
}

// resetStats resets the statistics counters
func (f *Fan) resetStats() {
	f.Stats = Stats{}
}

// logStats logs the current statistics
func (f *Fan) logStats() {
	f.Logger.Info("FAN Statistics:")
	f.Logger.Info("- Decisions made: %d", f.Stats.Decisions)
	f.Logger.Info("- Backtracks performed: %d", f.Stats.Backtracks)
	f.Logger.Info("- Implications performed: %d", f.Stats.Implications)
	f.Logger.Info("- Maximum decision depth: %d", f.Stats.MaxDecisionDepth)
	f.Logger.Info("- Total time: %v", f.Stats.TotalTime)
}
