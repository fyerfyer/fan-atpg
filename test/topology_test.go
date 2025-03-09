package test

import (
	"testing"

	"github.com/fyerfyer/fan-atpg/pkg/circuit"
)

// TestComputeLevels tests the level computation in circuit topology
func TestComputeLevels(t *testing.T) {
	// Create a simple circuit for testing
	c := createLevelTestCircuit()

	// Create and analyze topology
	topo := circuit.NewTopology(c)
	topo.ComputeLevels()

	// Check levels
	// Primary inputs should be level 0
	for _, input := range c.Inputs {
		if level, exists := topo.LevelMap[input]; !exists || level != 0 {
			t.Errorf("Expected input %s to be at level 0, got %d", input.Name, level)
		}
	}

	// Check specific gate outputs - depend on the test circuit structure
	// Find specific lines by name
	var w1, w2, w3, out *circuit.Line
	for _, line := range c.Lines {
		switch line.Name {
		case "w1":
			w1 = line
		case "w2":
			w2 = line
		case "w3":
			w3 = line
		case "out":
			out = line
		}
	}

	// w1 should be at level 1 (one gate from inputs)
	if level, exists := topo.LevelMap[w1]; !exists || level != 1 {
		t.Errorf("Expected w1 to be at level 1, got %d", level)
	}

	// w2 should be at level 2 (two gates from inputs)
	if level, exists := topo.LevelMap[w2]; !exists || level != 2 {
		t.Errorf("Expected w2 to be at level 2, got %d", level)
	}

	// w3 should be at level 1 (one gate from inputs)
	if level, exists := topo.LevelMap[w3]; !exists || level != 1 {
		t.Errorf("Expected w3 to be at level 1, got %d", level)
	}

	// Output should be at level 3 (three gates from inputs in longest path)
	if level, exists := topo.LevelMap[out]; !exists || level != 3 {
		t.Errorf("Expected output to be at level 3, got %d", level)
	}

	// Check maximum level
	if topo.MaxLevel != 3 {
		t.Errorf("Expected maximum level to be 3, got %d", topo.MaxLevel)
	}
}

// TestFanoutPointIdentification tests the identification of fanout points
func TestFanoutPointIdentification(t *testing.T) {
	// Create a circuit with fanout
	c := createFanoutTestCircuit()

	// Create and analyze topology
	topo := circuit.NewTopology(c)
	topo.IdentifyFanoutPoints()

	// Find the expected fanout point (in1)
	var in1 *circuit.Line
	for _, line := range c.Lines {
		if line.Name == "in1" {
			in1 = line
			break
		}
	}

	if in1 == nil {
		t.Fatalf("Could not find line 'in1'")
	}

	// Check that in1 is identified as a fanout point
	found := false
	for _, fanout := range topo.FanoutPoints {
		if fanout.ID == in1.ID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Line 'in1' should be identified as a fanout point")
	}

	// Check that we have the expected number of fanout points
	if len(topo.FanoutPoints) != 2 {
		t.Errorf("Expected 1 fanout point, got %d", len(topo.FanoutPoints))
	}
}

// TestFreeAndBoundRegions tests the identification of free and bound regions
func TestFreeAndBoundRegions(t *testing.T) {
	// Create a circuit with fanout
	c := createFanoutTestCircuit()

	// Create and analyze topology
	topo := circuit.NewTopology(c)
	topo.IdentifyFanoutPoints()
	topo.IdentifyFreeAndBoundRegions()

	// Find specific lines by name
	var in1, in2, w1, w2, out1, out2 *circuit.Line
	for _, line := range c.Lines {
		switch line.Name {
		case "in1":
			in1 = line
		case "in2":
			in2 = line
		case "w1":
			w1 = line
		case "w2":
			w2 = line
		case "out1":
			out1 = line
		case "out2":
			out2 = line
		}
	}

	// Check which lines are free and bound
	// in1 is a fanout point, should be free
	if !in1.IsFree || in1.IsBound {
		t.Errorf("Line 'in1' should be free and not bound")
	}

	// in2 is not reachable from a fanout point, should be free
	if !in2.IsFree || in2.IsBound {
		t.Errorf("Line 'in2' should be free and not bound")
	}

	// w1 and w2 are reachable from fanout point in1, should be bound
	if w1.IsFree || !w1.IsBound {
		t.Errorf("Line 'w1' should be bound and not free")
	}

	if w2.IsFree || !w2.IsBound {
		t.Errorf("Line 'w2' should be bound and not free")
	}

	// out1 and out2 are reachable from fanout point in1, should be bound
	if out1.IsFree || !out1.IsBound {
		t.Errorf("Line 'out1' should be bound and not free")
	}

	if out2.IsFree || !out2.IsBound {
		t.Errorf("Line 'out2' should be bound and not free")
	}
}

// TestHeadLineIdentification tests the identification of head lines
func TestHeadLineIdentification(t *testing.T) {
	// Create a circuit with head lines
	c := createHeadLineTestCircuit()

	// Create and analyze topology
	topo := circuit.NewTopology(c)
	topo.Analyze() // This will compute levels, identify fanouts, free/bound regions, and head lines

	// Find specific lines by name
	var in2, in3 *circuit.Line
	for _, line := range c.Lines {
		switch line.Name {
		//case "in1": in1 = line
		case "in2":
			in2 = line
		case "in3":
			in3 = line
			//case "w1": w1 = line
			//case "w2": w2 = line
			//case "w3": w3 = line
			//case "out": out = line
		}
	}

	// in2 should be a head line (feeds into a bound region)
	if !in2.IsHeadLine {
		t.Errorf("Line 'in2' should be identified as a head line")
	}

	// in3 should be a head line (feeds into a bound region)
	if !in3.IsHeadLine {
		t.Errorf("Line 'in3' should be identified as a head line")
	}

	// Check that we have the expected number of head lines
	if len(topo.HeadLinesList) != 2 {
		t.Errorf("Expected 2 head lines, got %d", len(topo.HeadLinesList))
	}

	// Verify that head lines are sorted by level
	if len(topo.HeadLinesList) >= 2 {
		level1 := topo.LevelMap[topo.HeadLinesList[0]]
		level2 := topo.LevelMap[topo.HeadLinesList[1]]
		if level1 > level2 {
			t.Errorf("Head lines should be sorted by level, but %d > %d", level1, level2)
		}
	}
}

// TestReconvergentPathIdentification tests the identification of reconvergent paths
func TestReconvergentPathIdentification(t *testing.T) {
	// Create a circuit with reconvergent paths
	c := createReconvergentPathTestCircuit()

	// Create and analyze topology
	topo := circuit.NewTopology(c)
	topo.IdentifyFanoutPoints()
	topo.IdentifyReconvergentPaths()

	// Find specific lines by name
	var w3, out *circuit.Line
	for _, line := range c.Lines {
		switch line.Name {
		case "w3":
			w3 = line
		case "out":
			out = line
		}
	}

	// w3 should be identified as a reconvergent point
	if !topo.ReconvPoints[w3] {
		t.Errorf("Line 'w3' should be identified as a reconvergent point")
	}

	// out should be identified as a reconvergent point
	if !topo.ReconvPoints[out] {
		t.Errorf("Line 'out' should be identified as a reconvergent point")
	}

	// Check that we have the expected number of reconvergent points
	expectedReconvPoints := 2
	if len(topo.ReconvPoints) != expectedReconvPoints {
		t.Errorf("Expected %d reconvergent points, got %d", expectedReconvPoints, len(topo.ReconvPoints))
	}
}

// TestFindUniquePathsToOutputs tests finding unique paths to outputs
func TestFindUniquePathsToOutputs(t *testing.T) {
	// Create a circuit with unique paths
	c := createUniquePathTestCircuit()

	// Create and analyze topology
	topo := circuit.NewTopology(c)
	topo.ComputeLevels()

	// Find specific gates and lines
	var g1 *circuit.Gate
	var w3, out *circuit.Line

	for _, gate := range c.Gates {
		if gate.Name == "g1" {
			g1 = gate
			break
		}
	}

	for _, line := range c.Lines {
		switch line.Name {
		case "w3":
			w3 = line
		case "out":
			out = line
		}
	}

	if g1 == nil {
		t.Fatalf("Could not find gate 'g1'")
	}

	// Find unique paths from g1 to outputs
	uniquePaths := topo.FindUniquePathsToOutputs(g1)

	// Check that w3 and out are in the unique paths
	foundW3 := false
	foundOut := false

	for _, line := range uniquePaths {
		if line.ID == w3.ID {
			foundW3 = true
		}
		if line.ID == out.ID {
			foundOut = true
		}
	}

	if !foundW3 {
		t.Errorf("Line 'w3' should be in the unique paths")
	}

	if !foundOut {
		t.Errorf("Line 'out' should be in the unique paths")
	}

	// Check that we have the expected number of unique paths
	if len(uniquePaths) != 2 {
		t.Errorf("Expected 2 lines in unique paths, got %d", len(uniquePaths))
	}
}

// TestFindPathBetween tests finding a path between two lines
func TestFindPathBetween(t *testing.T) {
	// Create a simple circuit
	c := createLevelTestCircuit()

	// Create and analyze topology
	topo := circuit.NewTopology(c)

	// Find specific lines by name
	var in1, out *circuit.Line
	for _, line := range c.Lines {
		switch line.Name {
		case "in1":
			in1 = line
		case "out":
			out = line
		}
	}

	if in1 == nil || out == nil {
		t.Fatalf("Could not find required lines")
	}

	// Find path from in1 to out
	path := topo.FindPathBetween(in1, out)

	// Check that a path was found
	if path == nil {
		t.Errorf("No path found between 'in1' and 'out'")
	}

	// Check that the path starts and ends with the correct lines
	if len(path) > 0 {
		if path[0].ID != in1.ID {
			t.Errorf("Path should start with 'in1'")
		}
		if path[len(path)-1].ID != out.ID {
			t.Errorf("Path should end with 'out'")
		}

		// Check that the path contains the expected number of lines
		expectedPathLength := 4 // in1, w1, w2, out
		if len(path) != expectedPathLength {
			t.Errorf("Expected path length %d, got %d", expectedPathLength, len(path))
		}
	}
}

// Helper function: Create a circuit for testing level computation
func createLevelTestCircuit() *circuit.Circuit {
	c := circuit.NewCircuit("level_test_circuit")

	// Create lines
	in1 := circuit.NewLine(0, "in1", circuit.PrimaryInput)
	in2 := circuit.NewLine(1, "in2", circuit.PrimaryInput)
	w1 := circuit.NewLine(2, "w1", circuit.Normal)
	w2 := circuit.NewLine(3, "w2", circuit.Normal)
	w3 := circuit.NewLine(4, "w3", circuit.Normal)
	out := circuit.NewLine(5, "out", circuit.PrimaryOutput)

	// Add lines to circuit
	c.AddLine(in1)
	c.AddLine(in2)
	c.AddLine(w1)
	c.AddLine(w2)
	c.AddLine(w3)
	c.AddLine(out)

	// Create gates
	g1 := circuit.NewGate(0, "g1", circuit.AND)
	g1.AddInput(in1)
	g1.AddInput(in2)
	g1.SetOutput(w1)

	g2 := circuit.NewGate(1, "g2", circuit.OR)
	g2.AddInput(w1)
	g2.AddInput(in2)
	g2.SetOutput(w2)

	g3 := circuit.NewGate(2, "g3", circuit.NOT)
	g3.AddInput(in2)
	g3.SetOutput(w3)

	g4 := circuit.NewGate(3, "g4", circuit.AND)
	g4.AddInput(w2)
	g4.AddInput(w3)
	g4.SetOutput(out)

	// Add gates to circuit
	c.AddGate(g1)
	c.AddGate(g2)
	c.AddGate(g3)
	c.AddGate(g4)

	return c
}

// Helper function: Create a circuit for testing fanout point identification
func createFanoutTestCircuit() *circuit.Circuit {
	c := circuit.NewCircuit("fanout_test_circuit")

	// Create lines
	in1 := circuit.NewLine(0, "in1", circuit.PrimaryInput)
	in2 := circuit.NewLine(1, "in2", circuit.PrimaryInput)
	w1 := circuit.NewLine(2, "w1", circuit.Normal)
	w2 := circuit.NewLine(3, "w2", circuit.Normal)
	out1 := circuit.NewLine(4, "out1", circuit.PrimaryOutput)
	out2 := circuit.NewLine(5, "out2", circuit.PrimaryOutput)

	// Add lines to circuit
	c.AddLine(in1)
	c.AddLine(in2)
	c.AddLine(w1)
	c.AddLine(w2)
	c.AddLine(out1)
	c.AddLine(out2)

	// Create gates with in1 having fanout to g1 and g2
	g1 := circuit.NewGate(0, "g1", circuit.AND)
	g1.AddInput(in1)
	g1.AddInput(in2)
	g1.SetOutput(w1)

	g2 := circuit.NewGate(1, "g2", circuit.OR)
	g2.AddInput(in1)
	g2.AddInput(in2)
	g2.SetOutput(w2)

	g3 := circuit.NewGate(2, "g3", circuit.NOT)
	g3.AddInput(w1)
	g3.SetOutput(out1)

	g4 := circuit.NewGate(3, "g4", circuit.NOT)
	g4.AddInput(w2)
	g4.SetOutput(out2)

	// Add gates to circuit
	c.AddGate(g1)
	c.AddGate(g2)
	c.AddGate(g3)
	c.AddGate(g4)

	return c
}

// Helper function: Create a circuit for testing head line identification
func createHeadLineTestCircuit() *circuit.Circuit {
	c := circuit.NewCircuit("head_line_test_circuit")

	// Create lines
	in1 := circuit.NewLine(0, "in1", circuit.PrimaryInput)  // Fanout point
	in2 := circuit.NewLine(1, "in2", circuit.PrimaryInput)  // Head line
	in3 := circuit.NewLine(2, "in3", circuit.PrimaryInput)  // Head line
	w1 := circuit.NewLine(3, "w1", circuit.Normal)          // Bound line
	w2 := circuit.NewLine(4, "w2", circuit.Normal)          // Bound line
	w3 := circuit.NewLine(5, "w3", circuit.Normal)          // Bound line
	out := circuit.NewLine(6, "out", circuit.PrimaryOutput) // Bound line

	// Add lines to circuit
	c.AddLine(in1)
	c.AddLine(in2)
	c.AddLine(in3)
	c.AddLine(w1)
	c.AddLine(w2)
	c.AddLine(w3)
	c.AddLine(out)

	// Create gates with in1 having fanout
	g1 := circuit.NewGate(0, "g1", circuit.AND)
	g1.AddInput(in1)
	g1.AddInput(in2) // Head line
	g1.SetOutput(w1)

	g2 := circuit.NewGate(1, "g2", circuit.OR)
	g2.AddInput(in1)
	g2.AddInput(in3) // Head line
	g2.SetOutput(w2)

	g3 := circuit.NewGate(2, "g3", circuit.AND)
	g3.AddInput(w1)
	g3.AddInput(w2)
	g3.SetOutput(w3)

	g4 := circuit.NewGate(3, "g4", circuit.NOT)
	g4.AddInput(w3)
	g4.SetOutput(out)

	// Add gates to circuit
	c.AddGate(g1)
	c.AddGate(g2)
	c.AddGate(g3)
	c.AddGate(g4)

	return c
}

// Helper function: Create a circuit with reconvergent paths
func createReconvergentPathTestCircuit() *circuit.Circuit {
	c := circuit.NewCircuit("reconvergent_test_circuit")

	// Create lines
	in1 := circuit.NewLine(0, "in1", circuit.PrimaryInput) // Fanout point
	in2 := circuit.NewLine(1, "in2", circuit.PrimaryInput)
	w1 := circuit.NewLine(2, "w1", circuit.Normal)
	w2 := circuit.NewLine(3, "w2", circuit.Normal)
	w3 := circuit.NewLine(4, "w3", circuit.Normal)          // Reconvergent point
	out := circuit.NewLine(5, "out", circuit.PrimaryOutput) // Reconvergent point

	// Add lines to circuit
	c.AddLine(in1)
	c.AddLine(in2)
	c.AddLine(w1)
	c.AddLine(w2)
	c.AddLine(w3)
	c.AddLine(out)

	// Create gates with in1 having fanout that reconverges at w3
	g1 := circuit.NewGate(0, "g1", circuit.AND)
	g1.AddInput(in1)
	g1.AddInput(in2)
	g1.SetOutput(w1)

	g2 := circuit.NewGate(1, "g2", circuit.NOT)
	g2.AddInput(in1)
	g2.SetOutput(w2)

	g3 := circuit.NewGate(2, "g3", circuit.OR)
	g3.AddInput(w1)
	g3.AddInput(w2)
	g3.SetOutput(w3) // Reconvergence of in1

	g4 := circuit.NewGate(3, "g4", circuit.NOT)
	g4.AddInput(w3)
	g4.SetOutput(out)

	// Add gates to circuit
	c.AddGate(g1)
	c.AddGate(g2)
	c.AddGate(g3)
	c.AddGate(g4)

	return c
}

// Helper function: Create a circuit for testing unique path finding
func createUniquePathTestCircuit() *circuit.Circuit {
	c := circuit.NewCircuit("unique_path_test_circuit")

	// Create lines
	in1 := circuit.NewLine(0, "in1", circuit.PrimaryInput)
	in2 := circuit.NewLine(1, "in2", circuit.PrimaryInput)
	w1 := circuit.NewLine(2, "w1", circuit.Normal)
	w2 := circuit.NewLine(3, "w2", circuit.Normal)
	w3 := circuit.NewLine(4, "w3", circuit.Normal)          // Part of unique path
	out := circuit.NewLine(5, "out", circuit.PrimaryOutput) // Part of unique path

	// Add lines to circuit
	c.AddLine(in1)
	c.AddLine(in2)
	c.AddLine(w1)
	c.AddLine(w2)
	c.AddLine(w3)
	c.AddLine(out)

	// Create gates where g1's output must go through w3 to reach the output
	g1 := circuit.NewGate(0, "g1", circuit.AND)
	g1.AddInput(in1)
	g1.AddInput(in2)
	g1.SetOutput(w1)

	g2 := circuit.NewGate(1, "g2", circuit.NOT)
	g2.AddInput(in2)
	g2.SetOutput(w2)

	g3 := circuit.NewGate(2, "g3", circuit.OR)
	g3.AddInput(w1)
	g3.AddInput(w2)
	g3.SetOutput(w3) // Mandatory path from g1

	g4 := circuit.NewGate(3, "g4", circuit.NOT)
	g4.AddInput(w3)
	g4.SetOutput(out) // Mandatory path from g1

	// Add gates to circuit
	c.AddGate(g1)
	c.AddGate(g2)
	c.AddGate(g3)
	c.AddGate(g4)

	return c
}
