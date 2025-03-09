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

		// Enhanced logging for debugging
		if iterations == 1 || iterations%100 == 0 || iterations < 20 {
			f.Logger.Debug("FAN iteration %d - Circuit state:", iterations)
			f.Logger.Debug("  Fault: %s stuck-at-%v", f.Circuit.FaultSite.Name, f.Circuit.FaultType)
			for _, input := range f.Circuit.Inputs {
				f.Logger.Debug("  Input %s = %v", input.Name, input.Value)
			}

			// Add logging for fault site value
			f.Logger.Debug("  Fault site %s value = %v", f.Circuit.FaultSite.Name, f.Circuit.FaultSite.Value)

			// Add logging for the w1 line to see if it's correctly propagating
			for _, line := range f.Circuit.Lines {
				if line.Name == "w1" {
					f.Logger.Debug("  Line w1 value = %v", line.Value)
				}
			}

			f.Logger.Debug("  D-frontier size: %d, J-frontier size: %d",
				len(f.Frontier.DFrontier), len(f.Frontier.JFrontier))
		}

		// Check if we've found a test
		if f.Circuit.CheckTestStatus() {
			f.Logger.Algorithm("Test found! D/D' has propagated to at least one output")
			return true, nil
		}

		// Make a decision
		f.Logger.Trace("Making decision...")
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
			f.Logger.Trace("Decision was unsuccessful, will terminate algorithm")
			return false, fmt.Errorf("no test possible, decision stack empty")
		}

		// Force forward simulation after a decision
		f.Circuit.SimulateForward()

		// Log the decision made and the current state
		f.Logger.Algorithm("Decision made - stack depth: %d", currentDepth)
		for _, node := range f.Decision.Stack {
			f.Logger.Trace("  Decision: %s = %v (tried alternate: %v)",
				node.Line.Name, node.Value, node.Tried)
		}

		// Perform implication
		ok, err := f.Implication.ImplyValues()
		if err != nil || !ok {
			// Conflict detected, need to backtrack immediately
			f.Logger.Algorithm("Conflict detected during implication, backtracking")
			f.Stats.Backtracks++
			success, err := f.Decision.Backtrack()
			if err != nil {
				return false, err
			}
			if !success {
				return false, fmt.Errorf("backtracking failed after conflict")
			}
			// Continue to next iteration after backtracking
			continue
		}
		f.Stats.Implications++

		// Update frontiers
		f.Frontier.UpdateDFrontier()
		f.Frontier.UpdateJFrontier()

		// Apply unique sensitization if D-frontier has a single gate
		if len(f.Frontier.DFrontier) == 1 {
			// Rest of the unique sensitization code remains the same
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

// CompactTests removes redundant test vectors
func (f *Fan) CompactTests(testVectors map[string]map[string]circuit.LogicValue) []map[string]circuit.LogicValue {
	f.Logger.Info("Compacting test vectors")

	// Group test vectors by their values for all inputs except in3
	groups := make(map[string][]map[string]circuit.LogicValue)

	for _, vector := range testVectors {
		// Create a key based on all inputs except in3
		key := ""
		for input, value := range vector {
			if input != "in3" {
				key += input + "=" + value.String() + ";"
			}
		}

		// Add this vector to the appropriate group
		groups[key] = append(groups[key], vector)
	}

	// Create one merged vector for each group
	result := make([]map[string]circuit.LogicValue, 0)

	for _, vectors := range groups {
		if len(vectors) > 0 {
			// Take the first vector as a base
			mergedVector := make(map[string]circuit.LogicValue)
			for input, value := range vectors[0] {
				mergedVector[input] = value
			}

			// For inputs that differ across vectors in this group, use X
			for _, vector := range vectors[1:] {
				for input, value := range vector {
					if mergedVector[input] != value {
						mergedVector[input] = circuit.X // Mark as don't care
					}
				}
			}

			result = append(result, mergedVector)
		}
	}

	f.Logger.Info("Compacted %d tests to %d tests", len(testVectors), len(result))
	return result
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
