package test

import (
	"github.com/fyerfyer/fan-atpg/pkg/algorithm"
	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
	"testing"
)

// TestApplyUniqueSensitization tests the unique sensitization functionality
func TestApplyUniqueSensitization(t *testing.T) {
	// Create a test circuit with unique paths for sensitization
	c := createSensitizationTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	topo := circuit.NewTopology(c)
	topo.Analyze()
	frontier := algorithm.NewFrontier(c, logger)
	implication := algorithm.NewImplication(c, frontier, topo, logger)
	sensitize := algorithm.NewSensitization(c, topo, implication, frontier, logger)

	// Set up a fault and D-frontier
	in1 := findLine(c, "in1")
	c.InjectFault(in1, circuit.Zero) // stuck-at-0 fault
	in1.SetValue(circuit.One)        // creates D'

	g1 := findGate(c, "g1")
	g1.Output.SetValue(circuit.X) // Make sure output is X to be in D-frontier

	// Set non-faulty input to make gate sensitizable
	in2 := findLine(c, "in2")
	in2.SetValue(circuit.One) // non-controlling value for AND gate

	// Update D-frontier
	frontier.UpdateDFrontier()

	// Verify g1 is in D-frontier
	if len(frontier.DFrontier) != 1 || frontier.DFrontier[0].Name != "g1" {
		t.Fatalf("Failed to set up D-frontier properly")
	}

	// Apply unique sensitization
	changed, err := sensitize.ApplyUniqueSensitization(g1)
	if err != nil {
		t.Fatalf("ApplyUniqueSensitization failed: %v", err)
	}

	// Check that unique sensitization made changes
	if !changed {
		t.Errorf("Expected unique sensitization to make changes")
	}

	// Check that appropriate side inputs were set to non-controlling values
	in4 := findLine(c, "in4")
	if in4.Value != circuit.One {
		t.Errorf("Expected side input in4 to be set to 1 (non-controlling value), got %v", in4.Value)
	}
}

// TestSensitizePathsToOutputs tests sensitizing paths from a gate to outputs
func TestSensitizePathsToOutputs(t *testing.T) {
	c := createEnhancedSensitizationCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	topo := circuit.NewTopology(c)
	topo.Analyze()
	frontier := algorithm.NewFrontier(c, logger)
	implication := algorithm.NewImplication(c, frontier, topo, logger)
	sensitize := algorithm.NewSensitization(c, topo, implication, frontier, logger)

	// Find source gate
	g2 := findGate(c, "g2")

	// Find unique paths from g2 to outputs
	paths := topo.FindUniquePathsToOutputs(g2)

	// Ensure we found paths
	if len(paths) == 0 {
		t.Fatalf("Failed to find paths from g2 to outputs")
	}

	// Try to sensitize the paths
	changed, err := sensitize.SensitizePathsToOutputs(g2, paths)
	if err != nil {
		t.Fatalf("sensitizePathsToOutputs failed: %v", err)
	}

	// Check that side inputs were set correctly
	in4 := findLine(c, "in4")
	in5 := findLine(c, "in5")

	// Both inputs should be set to non-controlling values for their gates
	if !changed || (in4.Value != circuit.One && in5.Value != circuit.One) {
		t.Errorf("Expected side inputs to be set to non-controlling values")
	}
}

// TestIdentifySensitizableGates tests identifying gates that can be sensitized
func TestIdentifySensitizableGates(t *testing.T) {
	c := createSensitizationTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	topo := circuit.NewTopology(c)
	topo.Analyze()
	frontier := algorithm.NewFrontier(c, logger)
	implication := algorithm.NewImplication(c, frontier, topo, logger)
	sensitize := algorithm.NewSensitization(c, topo, implication, frontier, logger)

	// Set up two gates in D-frontier, one with path to output, one without
	in1 := findLine(c, "in1")
	c.InjectFault(in1, circuit.Zero)
	in1.SetValue(circuit.One) // creates D'

	g1 := findGate(c, "g1")
	g1.Output.SetValue(circuit.X)
	in2 := findLine(c, "in2")
	in2.SetValue(circuit.One) // Non-controlling value

	// Add g1 to D-frontier
	frontier.DFrontier = []*circuit.Gate{g1}
	g1.IsInDFrontier = true

	// Find sensitizable gates
	gates := sensitize.IdentifySensitizableGates()

	// Should find g1 as sensitizable
	if len(gates) != 1 || gates[0].Name != "g1" {
		t.Errorf("Expected to find g1 as sensitizable gate")
	}
}

// TestTrySensitizePath tests trying to sensitize a specific path
func TestTrySensitizePath(t *testing.T) {
	c := createSensitizationTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	topo := circuit.NewTopology(c)
	topo.Analyze()
	frontier := algorithm.NewFrontier(c, logger)
	implication := algorithm.NewImplication(c, frontier, topo, logger)
	sensitize := algorithm.NewSensitization(c, topo, implication, frontier, logger)

	// Find a gate to sensitize
	g1 := findGate(c, "g1")

	// Try sensitizing path from this gate
	changed, err := sensitize.TrySensitizePath(g1)
	if err != nil {
		t.Fatalf("TrySensitizePath failed: %v", err)
	}

	// Check that path sensitization made changes to non-controlling inputs
	if !changed {
		t.Errorf("Expected path sensitization to make changes")
	}
}

// TestFindCriticalInputs tests identifying inputs critical for fault propagation
func TestFindCriticalInputs(t *testing.T) {
	c := createSensitizationTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	topo := circuit.NewTopology(c)
	topo.Analyze()
	frontier := algorithm.NewFrontier(c, logger)
	implication := algorithm.NewImplication(c, frontier, topo, logger)
	sensitize := algorithm.NewSensitization(c, topo, implication, frontier, logger)

	// Set up a gate in D-frontier
	g1 := findGate(c, "g1")
	in1 := findLine(c, "in1")
	in2 := findLine(c, "in2")

	// Make in1 faulty
	c.InjectFault(in1, circuit.Zero)
	in1.SetValue(circuit.One) // D'

	// Keep in2 unassigned
	in2.SetValue(circuit.X)

	// Keep output as X to be in D-frontier
	g1.Output.SetValue(circuit.X)

	// Add to D-frontier
	frontier.DFrontier = []*circuit.Gate{g1}
	g1.IsInDFrontier = true

	// Find critical inputs
	objectives := sensitize.FindCriticalInputs()

	// Should identify in2 as critical with value 1 (non-controlling for AND)
	if len(objectives) != 1 || objectives[0].Line.Name != "in2" || objectives[0].Value != circuit.One {
		t.Errorf("Failed to identify critical input correctly")
	}
}

// TestIsPathSensitized tests if a path is correctly determined to be sensitized
func TestIsPathSensitized(t *testing.T) {
	c := createSensitizationTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	topo := circuit.NewTopology(c)
	topo.Analyze()
	frontier := algorithm.NewFrontier(c, logger)
	implication := algorithm.NewImplication(c, frontier, topo, logger)
	sensitize := algorithm.NewSensitization(c, topo, implication, frontier, logger)

	// Set up a sensitized path: in1 -> g1 -> w1 -> g3 -> w3 -> g4 -> out
	in1 := findLine(c, "in1")
	in2 := findLine(c, "in2")
	in3 := findLine(c, "in3")
	in4 := findLine(c, "in4")
	w1 := findLine(c, "w1")
	w2 := findLine(c, "w2")
	w3 := findLine(c, "w3")

	// Set values to create sensitized path
	in1.SetValue(circuit.One)
	in2.SetValue(circuit.One)  // non-controlling value for AND
	in3.SetValue(circuit.Zero) // non-controlling value for OR
	in4.SetValue(circuit.One)  // non-controlling value for AND
	w1.SetValue(circuit.One)
	w2.SetValue(circuit.Zero)
	w3.SetValue(circuit.One)

	// Check if path from w1 is sensitized
	if !sensitize.IsPathSensitized(w1) {
		t.Errorf("Expected path from w1 to be sensitized")
	}
}

// TestGetSensitizationObjectives tests getting sensitization objectives
func TestGetSensitizationObjectives(t *testing.T) {
	c := createSensitizationTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	topo := circuit.NewTopology(c)
	topo.Analyze()
	frontier := algorithm.NewFrontier(c, logger)
	implication := algorithm.NewImplication(c, frontier, topo, logger)
	sensitize := algorithm.NewSensitization(c, topo, implication, frontier, logger)

	// Set up D-frontier
	g1 := findGate(c, "g1")
	in1 := findLine(c, "in1")

	// Make in1 faulty
	c.InjectFault(in1, circuit.Zero)
	in1.SetValue(circuit.One) // D'

	// Keep output as X to be in D-frontier
	g1.Output.SetValue(circuit.X)

	// Add to D-frontier
	frontier.DFrontier = []*circuit.Gate{g1}
	g1.IsInDFrontier = true

	// Get sensitization objectives
	objectives := sensitize.GetSensitizationObjectives()

	// Should include objectives to set necessary side inputs
	if len(objectives) == 0 {
		t.Errorf("Expected sensitization objectives, got none")
	}
}

// Helper function to create a test circuit specifically designed for sensitization testing
func createSensitizationTestCircuit() *circuit.Circuit {
	c := circuit.NewCircuit("sensitization_test_circuit")

	// Create lines
	in1 := circuit.NewLine(1, "in1", circuit.PrimaryInput)
	in2 := circuit.NewLine(2, "in2", circuit.PrimaryInput)
	in3 := circuit.NewLine(3, "in3", circuit.PrimaryInput)
	in4 := circuit.NewLine(4, "in4", circuit.PrimaryInput)
	w1 := circuit.NewLine(5, "w1", circuit.Normal)
	w2 := circuit.NewLine(6, "w2", circuit.Normal)
	w3 := circuit.NewLine(7, "w3", circuit.Normal)
	out := circuit.NewLine(8, "out", circuit.PrimaryOutput)

	// Add lines to circuit
	c.AddLine(in1)
	c.AddLine(in2)
	c.AddLine(in3)
	c.AddLine(in4)
	c.AddLine(w1)
	c.AddLine(w2)
	c.AddLine(w3)
	c.AddLine(out)

	// Create gates
	// AND gate (g1) with inputs in1, in2 and output w1
	g1 := circuit.NewGate(1, "g1", circuit.AND)
	g1.AddInput(in1)
	g1.AddInput(in2)
	g1.SetOutput(w1)

	// OR gate (g2) with inputs w1, in3 and output w2
	g2 := circuit.NewGate(2, "g2", circuit.OR)
	g2.AddInput(w1)
	g2.AddInput(in3)
	g2.SetOutput(w2)

	// AND gate (g3) with inputs w1, w2, in4 and output w3
	g3 := circuit.NewGate(3, "g3", circuit.AND)
	g3.AddInput(w1)
	g3.AddInput(w2)
	g3.AddInput(in4)
	g3.SetOutput(w3)

	// NOT gate (g4) with input w3 and output out
	g4 := circuit.NewGate(4, "g4", circuit.NOT)
	g4.AddInput(w3)
	g4.SetOutput(out)

	// Add gates to circuit
	c.AddGate(g1)
	c.AddGate(g2)
	c.AddGate(g3)
	c.AddGate(g4)

	return c
}

// Create a better circuit with clearer unique paths for enhanced sensitization testing
func createEnhancedSensitizationCircuit() *circuit.Circuit {
	c := circuit.NewCircuit("enhanced_sensitization_circuit")

	// Create lines with clearer purpose
	in1 := circuit.NewLine(1, "in1", circuit.PrimaryInput) // Will contain D value
	in2 := circuit.NewLine(2, "in2", circuit.PrimaryInput) // Side input for g1
	in3 := circuit.NewLine(3, "in3", circuit.PrimaryInput) // Side input for g2
	in4 := circuit.NewLine(4, "in4", circuit.PrimaryInput) // Side input for g3 (path 1)
	in5 := circuit.NewLine(5, "in5", circuit.PrimaryInput) // Side input for g4 (path 2)
	in6 := circuit.NewLine(6, "in6", circuit.PrimaryInput) // Side input for g6

	w1 := circuit.NewLine(7, "w1", circuit.Normal)  // Output of g1
	w2 := circuit.NewLine(8, "w2", circuit.Normal)  // Output of g2 (D-frontier)
	w3 := circuit.NewLine(9, "w3", circuit.Normal)  // Output of g3 (path 1)
	w4 := circuit.NewLine(10, "w4", circuit.Normal) // Output of g4 (path 2)
	w5 := circuit.NewLine(11, "w5", circuit.Normal) // Output of g5 (join point)
	out := circuit.NewLine(12, "out", circuit.PrimaryOutput)

	// Add all lines to circuit
	c.AddLine(in1)
	c.AddLine(in2)
	c.AddLine(in3)
	c.AddLine(in4)
	c.AddLine(in5)
	c.AddLine(in6)
	c.AddLine(w1)
	c.AddLine(w2)
	c.AddLine(w3)
	c.AddLine(w4)
	c.AddLine(w5)
	c.AddLine(out)

	// Create gates in a structure that ensures unique paths
	// g1: AND gate creating the D value
	g1 := circuit.NewGate(1, "g1", circuit.AND)
	g1.AddInput(in1)
	g1.AddInput(in2)
	g1.SetOutput(w1) // w1 will have D value

	// g2: OR gate which is the D-frontier gate
	g2 := circuit.NewGate(2, "g2", circuit.OR)
	g2.AddInput(w1)  // D input
	g2.AddInput(in3) // Side input (will be set to 0)
	g2.SetOutput(w2)

	// PATH 1: AND gate with w2 and in4
	g3 := circuit.NewGate(3, "g3", circuit.AND)
	g3.AddInput(w2)  // From D-frontier
	g3.AddInput(in4) // Side input that should be set by unique sensitization
	g3.SetOutput(w3)

	// PATH 2: AND gate with w2 and in5
	g4 := circuit.NewGate(4, "g4", circuit.AND)
	g4.AddInput(w2)  // From D-frontier
	g4.AddInput(in5) // Side input that should be set by unique sensitization
	g4.SetOutput(w4)

	// Join paths with OR gate
	g5 := circuit.NewGate(5, "g5", circuit.OR)
	g5.AddInput(w3)
	g5.AddInput(w4)
	g5.SetOutput(w5)

	// Final gate to output
	g6 := circuit.NewGate(6, "g6", circuit.AND)
	g6.AddInput(w5)
	g6.AddInput(in6) // Another side input that should be set
	g6.SetOutput(out)

	// Add all gates to circuit
	c.AddGate(g1)
	c.AddGate(g2)
	c.AddGate(g3)
	c.AddGate(g4)
	c.AddGate(g5)
	c.AddGate(g6)

	return c
}
