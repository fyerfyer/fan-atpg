package test

import (
	"github.com/fyerfyer/fan-atpg/pkg/algorithm"
	"testing"

	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
)

func TestUpdateDFrontier(t *testing.T) {
	c := createFrontierTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	frontier := algorithm.NewFrontier(c, logger)

	// Initially D-frontier should be empty
	frontier.UpdateDFrontier()
	if len(frontier.DFrontier) != 0 {
		t.Errorf("Expected empty D-frontier initially, got %d gates", len(frontier.DFrontier))
	}

	// Set up a fault and D value on in1
	in1 := findLine(c, "in1")
	c.InjectFault(in1, circuit.Zero) // Stuck-at-0 fault
	in1.SetValue(circuit.One)        // Set to opposite of fault value -> creates D'

	// Check that in1 now has a D' value
	if in1.Value != circuit.Dnot {
		t.Fatalf("Expected in1 to have D' value, got %v", in1.Value)
	}

	// Set g1's output to X
	findLine(c, "w1").SetValue(circuit.X)

	// Set non-faulty input to non-controlling value to make gate sensitizable
	findLine(c, "in2").SetValue(circuit.One) // AND gate needs 1 for non-controlling value

	// Update the D-frontier
	frontier.UpdateDFrontier()

	// Verify: g1 should be in D-frontier because it has a faulty input and X output
	if len(frontier.DFrontier) != 1 {
		t.Fatalf("Expected 1 gate in D-frontier, got %d", len(frontier.DFrontier))
	}

	if frontier.DFrontier[0].Name != "g1" {
		t.Errorf("Expected g1 in D-frontier, got %s", frontier.DFrontier[0].Name)
	}

	// If we set the output of g1 to a non-X value, it should leave the D-frontier
	findLine(c, "w1").SetValue(circuit.One)
	frontier.UpdateDFrontier()
	if len(frontier.DFrontier) != 0 {
		t.Errorf("Expected empty D-frontier after setting g1 output, got %d gates", len(frontier.DFrontier))
	}
}

func TestUpdateJFrontier(t *testing.T) {
	c := createFrontierTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	frontier := algorithm.NewFrontier(c, logger)

	// Initially J-frontier should be empty (all values are X)
	frontier.UpdateJFrontier()
	if len(frontier.JFrontier) != 0 {
		t.Errorf("Expected empty J-frontier initially, got %d gates", len(frontier.JFrontier))
	}

	// Set the output of a gate but leave inputs as X
	g3 := findGate(c, "g3")
	g3.Output.SetValue(circuit.One) // Assign output value

	// Update J-frontier
	frontier.UpdateJFrontier()

	// Verify: g3 should be in J-frontier because it has assigned output but unassigned inputs
	if len(frontier.JFrontier) != 1 {
		t.Fatalf("Expected 1 gate in J-frontier, got %d", len(frontier.JFrontier))
	}

	if frontier.JFrontier[0].Name != "g3" {
		t.Errorf("Expected g3 in J-frontier, got %s", frontier.JFrontier[0].Name)
	}

	// If we assign all inputs of g3, it should leave J-frontier
	for _, input := range g3.Inputs {
		input.SetValue(circuit.Zero)
	}

	frontier.UpdateJFrontier()

	// Check that g3 is no longer in J-frontier
	g3Found := false
	for _, gate := range frontier.JFrontier {
		if gate.ID == g3.ID {
			g3Found = true
			break
		}
	}

	if g3Found {
		t.Errorf("Expected g3 to leave J-frontier after setting all inputs")
	}
}

func TestGetObjectivesFromDFrontier(t *testing.T) {
	c := createFrontierTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	frontier := algorithm.NewFrontier(c, logger)

	// Set up a fault and D value on in1
	in1 := findLine(c, "in1")
	c.InjectFault(in1, circuit.Zero)
	in1.SetValue(circuit.One) // Creates D'

	// Find the AND gate g1 and set its output to X
	g1 := findGate(c, "g1")
	g1.Output.SetValue(circuit.X)

	// Set in2 to X (one unassigned side input on g1)
	in2 := findLine(c, "in2")
	in2.SetValue(circuit.X)

	// Don't update D-frontier normally
	// frontier.UpdateDFrontier()

	// Force add g1 to the D-frontier directly
	frontier.DFrontier = []*circuit.Gate{g1}
	// Mark the gate as being in the D-frontier
	g1.IsInDFrontier = true

	// Verify g1 is in D-frontier
	if len(frontier.DFrontier) != 1 || frontier.DFrontier[0].Name != "g1" {
		t.Fatalf("Setup failed: g1 not in D-frontier")
	}

	// Get objectives
	objectives := frontier.GetObjectivesFromDFrontier()

	// For AND gate, non-controlling value is 1
	// We expect an objective to set in2 to 1
	if len(objectives) != 1 {
		t.Fatalf("Expected 1 objective, got %d", len(objectives))
	}

	if objectives[0].Line.Name != "in2" || objectives[0].Value != circuit.One {
		t.Errorf("Expected objective for in2=1, got %s=%v",
			objectives[0].Line.Name, objectives[0].Value)
	}
}

func TestGetObjectivesFromJFrontier(t *testing.T) {
	c := createFrontierTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	frontier := algorithm.NewFrontier(c, logger)

	// Find OR gate g2 and set its output to 0
	g2 := findGate(c, "g2")
	g2.Output.SetValue(circuit.Zero)

	// Set one input to X and the other to already assigned
	g2.Inputs[0].SetValue(circuit.X)
	g2.Inputs[1].SetValue(circuit.Zero)

	// Update frontiers
	frontier.UpdateJFrontier()

	// Verify g2 is in J-frontier
	if len(frontier.JFrontier) != 1 || frontier.JFrontier[0].Name != "g2" {
		t.Fatalf("Setup failed: g2 not in J-frontier")
	}

	// Get objectives
	objectives := frontier.GetObjectivesFromJFrontier()

	// For OR gate with output=0, all inputs must be 0
	if len(objectives) != 1 {
		t.Fatalf("Expected 1 objective, got %d", len(objectives))
	}

	if objectives[0].Line.Name != g2.Inputs[0].Name || objectives[0].Value != circuit.Zero {
		t.Errorf("Expected objective for %s=0, got %s=%v",
			g2.Inputs[0].Name, objectives[0].Line.Name, objectives[0].Value)
	}
}

func TestGetDFrontierGate(t *testing.T) {
	c := createFrontierTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	frontier := algorithm.NewFrontier(c, logger)

	// Empty D-frontier should return nil
	if gate := frontier.GetDFrontierGate(); gate != nil {
		t.Errorf("Expected nil from empty D-frontier, got %v", gate)
	}

	// Set up gates for D-frontier
	in1 := findLine(c, "in1")
	c.InjectFault(in1, circuit.Zero)
	in1.SetValue(circuit.One) // Creates D'

	// Force add both g1 and g3 directly to D-frontier to ensure test consistency
	g1 := findGate(c, "g1")
	g3 := findGate(c, "g3")

	// Clear the D-frontier and manually add g1 and g3
	frontier.DFrontier = []*circuit.Gate{g3, g1} // Intentionally put g3 first

	// Verify D-frontier content
	if len(frontier.DFrontier) != 2 {
		t.Fatalf("Failed to set up test: D-frontier should have 2 gates, got %d", len(frontier.DFrontier))
	}

	// Print D-frontier for debugging
	t.Logf("D-frontier contains: %s (%d inputs), %s (%d inputs)",
		frontier.DFrontier[0].Name, len(frontier.DFrontier[0].Inputs),
		frontier.DFrontier[1].Name, len(frontier.DFrontier[1].Inputs))

	// Should choose g1 as it has fewer inputs
	gate := frontier.GetDFrontierGate()
	if gate == nil {
		t.Fatalf("GetDFrontierGate returned nil unexpectedly")
	}
	if gate.Name != "g1" {
		t.Errorf("Expected g1 to be selected from D-frontier, got %s", gate.Name)
	}
}

func TestGetJFrontierGate(t *testing.T) {
	c := createFrontierTestCircuit()
	logger := utils.NewLogger(utils.InfoLevel)
	frontier := algorithm.NewFrontier(c, logger)

	// Empty J-frontier should return nil
	if gate := frontier.GetJFrontierGate(); gate != nil {
		t.Errorf("Expected nil from empty J-frontier, got %v", gate)
	}

	// Create two gates in J-frontier with different numbers of unassigned inputs
	g2 := findGate(c, "g2")
	g2.Output.SetValue(circuit.Zero)
	// g2 has 2 unassigned inputs

	g3 := findGate(c, "g3")
	g3.Output.SetValue(circuit.One)
	// g3 has 3 unassigned inputs

	// Assign one input of g3 to make it have fewer unassigned inputs
	g3.Inputs[0].SetValue(circuit.One)
	g3.Inputs[1].SetValue(circuit.One)

	frontier.UpdateJFrontier()

	// Should choose g3 as it has fewer unassigned inputs
	gate := frontier.GetJFrontierGate()
	if gate == nil {
		t.Fatalf("GetJFrontierGate returned nil unexpectedly")
	}
	if gate.Name != "g3" {
		t.Errorf("Expected g3 to be selected from J-frontier, got %s", gate.Name)
	}
}

// Helper function to create a test circuit for frontier testing
func createFrontierTestCircuit() *circuit.Circuit {
	c := circuit.NewCircuit("frontier_test_circuit")

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
