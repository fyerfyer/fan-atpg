package test

import (
	"github.com/fyerfyer/fan-atpg/pkg/algorithm"
	"testing"

	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
)

// TestBacktraceFromDFrontier tests backtracing from the D-frontier
func TestBacktraceFromDFrontier(t *testing.T) {
	// Create a test circuit that allows for clear backtracing paths
	c := createBacktraceTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)

	// Create support objects
	topo := circuit.NewTopology(c)
	topo.Analyze()
	frontier := algorithm.NewFrontier(c, logger)
	implication := algorithm.NewImplication(c, frontier, topo, logger)
	backtrace := algorithm.NewBacktrace(c, topo, frontier, implication, logger)

	// Set up D-frontier scenario:
	// 1. Inject a fault at a specific line
	in1 := findLine(c, "in1")
	c.InjectFault(in1, circuit.Zero) // stuck-at-0 fault
	in1.SetValue(circuit.One)        // Set to opposite of fault value -> creates D'

	// 2. Set up the D-frontier
	g1 := findGate(c, "g1")
	in2 := findLine(c, "in2")
	in2.SetValue(circuit.X) // input needs to be set for sensitization

	// Force add g1 to the D-frontier directly
	frontier.DFrontier = []*circuit.Gate{g1}
	g1.IsInDFrontier = true

	// Perform backtrace from D-frontier
	line, value := backtrace.BacktraceFromDFrontier()

	// Verify that backtrace identified in2 to be set to 1 (non-controlling value for AND gate)
	if line == nil {
		t.Error("BacktraceFromDFrontier returned nil line")
	} else {
		if line.Name != "in2" {
			t.Errorf("Expected to backtrack to line in2, got %s", line.Name)
		}
		if value != circuit.One {
			t.Errorf("Expected to set in2 to 1 (non-controlling value for AND), got %v", value)
		}
	}
}

// TestBacktraceFromJFrontier tests backtracing from the J-frontier
func TestBacktraceFromJFrontier(t *testing.T) {
	// Create a test circuit
	c := createBacktraceTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)

	// Create support objects
	topo := circuit.NewTopology(c)
	topo.Analyze()
	frontier := algorithm.NewFrontier(c, logger)
	implication := algorithm.NewImplication(c, frontier, topo, logger)
	backtrace := algorithm.NewBacktrace(c, topo, frontier, implication, logger)

	// Set up J-frontier scenario:
	// 1. Set a gate output value but leave inputs unassigned
	g2 := findGate(c, "g2")
	g2.Output.SetValue(circuit.One) // OR gate output = 1

	// Force add g2 to the J-frontier
	frontier.JFrontier = []*circuit.Gate{g2}

	// Perform backtrace from J-frontier
	line, value := backtrace.BacktraceFromJFrontier()

	// Verify that backtrace identified an input to be set to 1 (for OR gate with output 1)
	if line == nil {
		t.Error("BacktraceFromJFrontier returned nil line")
	} else {
		// For OR gate with output=1, we need at least one input=1
		if value != circuit.One {
			t.Errorf("Expected value to be 1 for OR gate justification, got %v", value)
		}
	}
}

// TestDirectBacktrace tests direct backtracing for a specific line and value
func TestDirectBacktrace(t *testing.T) {
	// Create a test circuit
	c := createBacktraceTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)

	// Create support objects
	topo := circuit.NewTopology(c)
	topo.Analyze()
	frontier := algorithm.NewFrontier(c, logger)
	implication := algorithm.NewImplication(c, frontier, topo, logger)
	backtrace := algorithm.NewBacktrace(c, topo, frontier, implication, logger)

	// Choose a line to backtrack from
	w2 := findLine(c, "w2")

	// Perform direct backtrace
	line, _ := backtrace.DirectBacktrace(w2, circuit.One)

	// Verify that backtrace found a valid primary input or head line
	if line == nil {
		t.Error("DirectBacktrace returned nil line")
	} else {
		// The line should be either a primary input or a head line
		if line.Type != circuit.PrimaryInput && !line.IsHeadLine {
			t.Errorf("Expected line to be primary input or head line, got %s (type=%v, isHeadLine=%v)",
				line.Name, line.Type, line.IsHeadLine)
		}
	}
}

// TestGetNextObjective tests the prioritization of objectives
func TestGetNextObjective(t *testing.T) {
	// Create a test circuit
	c := createBacktraceTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)

	// Create support objects
	topo := circuit.NewTopology(c)
	topo.Analyze()
	frontier := algorithm.NewFrontier(c, logger)
	implication := algorithm.NewImplication(c, frontier, topo, logger)
	backtrace := algorithm.NewBacktrace(c, topo, frontier, implication, logger)

	// Scenario 1: Need to activate fault - should be highest priority
	in1 := findLine(c, "in1")
	c.InjectFault(in1, circuit.Zero)
	line, value, shouldContinue := backtrace.GetNextObjective()

	if line == nil || !shouldContinue {
		t.Error("GetNextObjective failed to identify fault activation objective")
	} else {
		if line.ID != in1.ID {
			t.Errorf("Expected to target fault site in1, got %s", line.Name)
		}
		if value != circuit.One {
			t.Errorf("Expected to set stuck-at-0 fault site to 1, got %v", value)
		}
	}

	// Scenario 2: D-frontier exists with no active fault
	c.Reset()
	g1 := findGate(c, "g1")
	in2 := findLine(c, "in2")

	// Manually create a D-frontier
	in1 = findLine(c, "in1")
	in1.SetValue(circuit.D)
	g1.Output.SetValue(circuit.X)
	in2.SetValue(circuit.X)
	frontier.DFrontier = []*circuit.Gate{g1}
	g1.IsInDFrontier = true

	line, value, shouldContinue = backtrace.GetNextObjective()

	if !shouldContinue {
		t.Error("GetNextObjective with D-frontier should continue")
	}
	if line == nil {
		t.Error("GetNextObjective with D-frontier returned nil line")
	}

	// Scenario 3: J-frontier exists with no D-frontier
	c.Reset()
	g2 := findGate(c, "g2")
	g2.Output.SetValue(circuit.One)
	frontier.DFrontier = []*circuit.Gate{}
	frontier.JFrontier = []*circuit.Gate{g2}

	line, value, shouldContinue = backtrace.GetNextObjective()

	if !shouldContinue {
		t.Error("GetNextObjective with J-frontier should continue")
	}
	if line == nil {
		t.Error("GetNextObjective with J-frontier returned nil line")
	}
}

// TestMultipleBacktraceSetup tests the setup of multiple backtrace objectives
func TestMultipleBacktraceSetup(t *testing.T) {
	// Create a test circuit
	c := createBacktraceTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)

	// Create multiple backtrace object
	topo := circuit.NewTopology(c)
	topo.Analyze()
	mbt := algorithm.NewMultipleBacktrace(c, topo, logger)

	// Create initial objectives
	in1 := findLine(c, "in1")
	in2 := findLine(c, "in2")
	initialObjs := []algorithm.InitialObjective{
		{Line: in1, Value: circuit.One},
		{Line: in2, Value: circuit.Zero},
	}

	// Set initial objectives
	mbt.SetInitialObjectives(initialObjs)

	// Verify correct conversion to current objectives
	if len(mbt.CurrentObjs) != 2 {
		t.Errorf("Expected 2 current objectives, got %d", len(mbt.CurrentObjs))
	}

	// Verify first objective (in1=1)
	if mbt.CurrentObjs[0].Line.ID != in1.ID {
		t.Errorf("Expected first objective to be for line in1, got %s", mbt.CurrentObjs[0].Line.Name)
	}
	if mbt.CurrentObjs[0].N0 != 0 || mbt.CurrentObjs[0].N1 != 1 {
		t.Errorf("Expected first objective to be (n₀=0, n₁=1), got (n₀=%d, n₁=%d)",
			mbt.CurrentObjs[0].N0, mbt.CurrentObjs[0].N1)
	}

	// Verify second objective (in2=0)
	if mbt.CurrentObjs[1].Line.ID != in2.ID {
		t.Errorf("Expected second objective to be for line in2, got %s", mbt.CurrentObjs[1].Line.Name)
	}
	if mbt.CurrentObjs[1].N0 != 1 || mbt.CurrentObjs[1].N1 != 0 {
		t.Errorf("Expected second objective to be (n₀=1, n₁=0), got (n₀=%d, n₁=%d)",
			mbt.CurrentObjs[1].N0, mbt.CurrentObjs[1].N1)
	}
}

// TestMultipleBacktracePerform tests performing multiple backtrace
func TestMultipleBacktracePerform(t *testing.T) {
	// Create a test circuit with a more complex structure for multiple backtrace
	c := createMultipleBacktraceTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	topo := circuit.NewTopology(c)
	topo.Analyze()
	mbt := algorithm.NewMultipleBacktrace(c, topo, logger)

	// Create initial objectives
	w3 := findLine(c, "w3")
	initialObjs := []algorithm.InitialObjective{
		{Line: w3, Value: circuit.One},
	}

	// Set initial objectives
	mbt.SetInitialObjectives(initialObjs)

	// Perform multiple backtrace
	mbt.PerformBacktrace()

	// Check that we have final objectives
	if len(mbt.FinalObjs) == 0 {
		t.Error("Multiple backtrace did not produce any final objectives")
	}

	// Get the best final objective
	line, _ := mbt.GetBestFinalObjective()
	if line == nil {
		t.Error("GetBestFinalObjective returned nil line")
	} else {
		// The line should be either a primary input or a head line
		if line.Type != circuit.PrimaryInput && !line.IsHeadLine {
			t.Errorf("Expected line to be primary input or head line, got %s (type=%v, isHeadLine=%v)",
				line.Name, line.Type, line.IsHeadLine)
		}
	}
}

// Helper: Create a test circuit for backtrace testing
func createBacktraceTestCircuit() *circuit.Circuit {
	c := circuit.NewCircuit("backtrace_test_circuit")

	// Create lines
	in1 := circuit.NewLine(1, "in1", circuit.PrimaryInput)
	in2 := circuit.NewLine(2, "in2", circuit.PrimaryInput)
	in3 := circuit.NewLine(3, "in3", circuit.PrimaryInput)
	w1 := circuit.NewLine(4, "w1", circuit.Normal)
	w2 := circuit.NewLine(5, "w2", circuit.Normal)
	w3 := circuit.NewLine(6, "w3", circuit.Normal)
	out := circuit.NewLine(7, "out", circuit.PrimaryOutput)

	// Add lines to circuit
	c.AddLine(in1)
	c.AddLine(in2)
	c.AddLine(in3)
	c.AddLine(w1)
	c.AddLine(w2)
	c.AddLine(w3)
	c.AddLine(out)

	// Create gates
	g1 := circuit.NewGate(1, "g1", circuit.AND)
	g1.AddInput(in1)
	g1.AddInput(in2)
	g1.SetOutput(w1)

	g2 := circuit.NewGate(2, "g2", circuit.OR)
	g2.AddInput(w1)
	g2.AddInput(in3)
	g2.SetOutput(w2)

	g3 := circuit.NewGate(3, "g3", circuit.AND)
	g3.AddInput(w1)
	g3.AddInput(w2)
	g3.SetOutput(w3)

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

// Create a more complex test circuit for multiple backtrace testing
func createMultipleBacktraceTestCircuit() *circuit.Circuit {
	c := circuit.NewCircuit("multiple_backtrace_test_circuit")

	// Create lines - more complex structure with multiple paths
	in1 := circuit.NewLine(1, "in1", circuit.PrimaryInput)
	in2 := circuit.NewLine(2, "in2", circuit.PrimaryInput)
	in3 := circuit.NewLine(3, "in3", circuit.PrimaryInput)
	in4 := circuit.NewLine(4, "in4", circuit.PrimaryInput)
	in5 := circuit.NewLine(5, "in5", circuit.PrimaryInput)
	w1 := circuit.NewLine(6, "w1", circuit.Normal) // Fanout point
	w2 := circuit.NewLine(7, "w2", circuit.Normal)
	w3 := circuit.NewLine(8, "w3", circuit.Normal) // Target line for backtrace
	w4 := circuit.NewLine(9, "w4", circuit.Normal)
	w5 := circuit.NewLine(10, "w5", circuit.Normal)
	w6 := circuit.NewLine(11, "w6", circuit.Normal) // Reconvergent point
	out := circuit.NewLine(12, "out", circuit.PrimaryOutput)

	// Add lines to circuit
	c.AddLine(in1)
	c.AddLine(in2)
	c.AddLine(in3)
	c.AddLine(in4)
	c.AddLine(in5)
	c.AddLine(w1)
	c.AddLine(w2)
	c.AddLine(w3)
	c.AddLine(w4)
	c.AddLine(w5)
	c.AddLine(w6)
	c.AddLine(out)

	// Create gates with a structure that supports multiple backtrace paths
	g1 := circuit.NewGate(1, "g1", circuit.AND)
	g1.AddInput(in1)
	g1.AddInput(in2)
	g1.SetOutput(w1) // Fanout point

	g2 := circuit.NewGate(2, "g2", circuit.OR)
	g2.AddInput(w1)
	g2.AddInput(in3)
	g2.SetOutput(w2)

	g3 := circuit.NewGate(3, "g3", circuit.AND)
	g3.AddInput(w2)
	g3.AddInput(in4)
	g3.SetOutput(w3) // Target for backtrace

	g4 := circuit.NewGate(4, "g4", circuit.NAND)
	g4.AddInput(w1) // Path from fanout
	g4.AddInput(in5)
	g4.SetOutput(w4)

	g5 := circuit.NewGate(5, "g5", circuit.OR)
	g5.AddInput(w3)
	g5.AddInput(w4)
	g5.SetOutput(w6) // Reconvergent point

	g6 := circuit.NewGate(6, "g6", circuit.NOT)
	g6.AddInput(w6)
	g6.SetOutput(out)

	// Add gates to circuit
	c.AddGate(g1)
	c.AddGate(g2)
	c.AddGate(g3)
	c.AddGate(g4)
	c.AddGate(g5)
	c.AddGate(g6)

	return c
}
