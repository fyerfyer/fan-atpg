package test

import (
	"testing"

	"github.com/fyerfyer/fan-atpg/pkg/circuit"
)

// TestLineCreation tests the creation and basic operations of lines
func TestLineCreation(t *testing.T) {
	// Create a new line
	line := circuit.NewLine(1, "test_line", circuit.PrimaryInput)

	// Check initialization
	if line.ID != 1 {
		t.Errorf("Expected line.ID to be 1, got %d", line.ID)
	}
	if line.Name != "test_line" {
		t.Errorf("Expected line.Name to be 'test_line', got '%s'", line.Name)
	}
	if line.Type != circuit.PrimaryInput {
		t.Errorf("Expected line.Type to be PrimaryInput")
	}
	if line.Value != circuit.X {
		t.Errorf("Expected initial line.Value to be X, got %v", line.Value)
	}
	if !line.IsFree {
		t.Errorf("Expected new line to be marked as free")
	}
	if line.IsBound {
		t.Errorf("Expected new line not to be bound")
	}
	if line.IsHeadLine {
		t.Errorf("Expected new line not to be a head line")
	}

	// Test SetValue
	line.SetValue(circuit.One)
	if line.Value != circuit.One {
		t.Errorf("Expected line.Value to be One after SetValue(One), got %v", line.Value)
	}
	if line.AssignmentCount != 1 {
		t.Errorf("Expected AssignmentCount to be 1, got %d", line.AssignmentCount)
	}

	// Test Reset
	line.Reset()
	if line.Value != circuit.X {
		t.Errorf("Expected line.Value to be X after Reset(), got %v", line.Value)
	}

	// Test IsAssigned
	if line.IsAssigned() {
		t.Errorf("Expected IsAssigned() to be false after Reset()")
	}
	line.SetValue(circuit.Zero)
	if !line.IsAssigned() {
		t.Errorf("Expected IsAssigned() to be true after SetValue(Zero)")
	}

	// Test String representation
	if line.String() != "test_line=0" {
		t.Errorf("Expected line.String() to be 'test_line=0', got '%s'", line.String())
	}
}

// TestFaultyLineValues tests operations specific to faulty lines (D and D')
func TestFaultyLineValues(t *testing.T) {
	line1 := circuit.NewLine(1, "d_line", circuit.Normal)
	line2 := circuit.NewLine(2, "dnot_line", circuit.Normal)

	line1.SetValue(circuit.D)
	line2.SetValue(circuit.Dnot)

	// Test IsFaulty
	if !line1.IsFaulty() {
		t.Errorf("Expected line with D value to be faulty")
	}
	if !line2.IsFaulty() {
		t.Errorf("Expected line with D' value to be faulty")
	}

	// Test GetGoodValue
	if line1.GetGoodValue() != circuit.Zero {
		t.Errorf("Expected good value for D to be 0")
	}
	if line2.GetGoodValue() != circuit.One {
		t.Errorf("Expected good value for D' to be 1")
	}

	// Test GetFaultyValue
	if line1.GetFaultyValue() != circuit.One {
		t.Errorf("Expected faulty value for D to be 1")
	}
	if line2.GetFaultyValue() != circuit.Zero {
		t.Errorf("Expected faulty value for D' to be 0")
	}
}

// TestGateCreation tests the creation and basic gate functionality
func TestGateCreation(t *testing.T) {
	// Create a gate
	gate := circuit.NewGate(1, "g1", circuit.AND)

	// Check initialization
	if gate.ID != 1 {
		t.Errorf("Expected gate.ID to be 1, got %d", gate.ID)
	}
	if gate.Name != "g1" {
		t.Errorf("Expected gate.Name to be 'g1', got '%s'", gate.Name)
	}
	if gate.Type != circuit.AND {
		t.Errorf("Expected gate.Type to be AND")
	}
	if len(gate.Inputs) != 0 {
		t.Errorf("Expected gate.Inputs to be empty, got %d inputs", len(gate.Inputs))
	}
	if gate.Output != nil {
		t.Errorf("Expected gate.Output to be nil")
	}
	if gate.IsInDFrontier {
		t.Errorf("Expected new gate not to be in D-frontier")
	}

	// Test string representation
	if gate.String() != "g1(AND)" {
		t.Errorf("Expected gate.String() to be 'g1(AND)', got '%s'", gate.String())
	}

	// Test gate type string representation
	if gate.Type.String() != "AND" {
		t.Errorf("Expected AND gate type string to be 'AND', got '%s'", gate.Type.String())
	}
}

// TestGateEvaluation tests the evaluation of different gate types
func TestGateEvaluation(t *testing.T) {
	// Testing AND gate
	andGate := createTestGate(circuit.AND, "and")
	testGateEvaluation(t, andGate, []circuit.LogicValue{circuit.One, circuit.One}, circuit.One)
	testGateEvaluation(t, andGate, []circuit.LogicValue{circuit.One, circuit.Zero}, circuit.Zero)
	testGateEvaluation(t, andGate, []circuit.LogicValue{circuit.X, circuit.One}, circuit.X)
	testGateEvaluation(t, andGate, []circuit.LogicValue{circuit.D, circuit.One}, circuit.D)
	testGateEvaluation(t, andGate, []circuit.LogicValue{circuit.Dnot, circuit.One}, circuit.Dnot)

	// Testing OR gate
	orGate := createTestGate(circuit.OR, "or")
	testGateEvaluation(t, orGate, []circuit.LogicValue{circuit.Zero, circuit.Zero}, circuit.Zero)
	testGateEvaluation(t, orGate, []circuit.LogicValue{circuit.One, circuit.Zero}, circuit.One)
	testGateEvaluation(t, orGate, []circuit.LogicValue{circuit.X, circuit.Zero}, circuit.X)
	testGateEvaluation(t, orGate, []circuit.LogicValue{circuit.D, circuit.Zero}, circuit.D)
	testGateEvaluation(t, orGate, []circuit.LogicValue{circuit.Dnot, circuit.Zero}, circuit.Dnot)

	// Testing NOT gate
	notGate := createTestGate(circuit.NOT, "not")
	testGateEvaluation(t, notGate, []circuit.LogicValue{circuit.Zero}, circuit.One)
	testGateEvaluation(t, notGate, []circuit.LogicValue{circuit.One}, circuit.Zero)
	testGateEvaluation(t, notGate, []circuit.LogicValue{circuit.X}, circuit.X)
	testGateEvaluation(t, notGate, []circuit.LogicValue{circuit.D}, circuit.Dnot)
	testGateEvaluation(t, notGate, []circuit.LogicValue{circuit.Dnot}, circuit.D)
}

// TestGateSensitization tests gate sensitization
func TestGateSensitization(t *testing.T) {
	// Create an AND gate
	andGate := createTestGate(circuit.AND, "and")

	// Case 1: One input faulty, other input is non-controlling (1) -> Should be sensitizable
	andGate.Inputs[0].SetValue(circuit.D)
	andGate.Inputs[1].SetValue(circuit.One)
	if !andGate.IsSensitizable() {
		t.Errorf("AND gate with inputs D,1 should be sensitizable")
	}

	// Case 2: One input faulty, other input is controlling (0) -> Should not be sensitizable
	andGate.Inputs[1].SetValue(circuit.Zero)
	if andGate.IsSensitizable() {
		t.Errorf("AND gate with inputs D,0 should not be sensitizable")
	}

	// Case 3: One input faulty, other input is X -> Should not be sensitizable
	andGate.Inputs[1].SetValue(circuit.X)
	if andGate.IsSensitizable() {
		t.Errorf("AND gate with inputs D,X should not be sensitizable")
	}
}

// TestCircuitFaultInjection tests fault injection and propagation
func TestCircuitFaultInjection(t *testing.T) {
	c := createTestCircuit()

	// Get a line to inject fault
	line := c.Lines[0]

	// Inject stuck-at-0 fault
	c.InjectFault(line, circuit.Zero)
	if c.FaultSite != line {
		t.Errorf("Expected FaultSite to be set correctly")
	}
	if c.FaultType != circuit.Zero {
		t.Errorf("Expected FaultType to be Zero")
	}

	// Set line value to opposite of fault (should create D/D' value)
	line.SetValue(circuit.One)
	if line.Value != circuit.Dnot { // One in good circuit, Zero in faulty (D')
		t.Errorf("Expected D' value after setting line with s-a-0 to 1, got %v", line.Value)
	}

	// Reset and try with stuck-at-1
	c.Reset()
	c.InjectFault(line, circuit.One)
	line.SetValue(circuit.Zero)
	if line.Value != circuit.D { // Zero in good circuit, One in faulty (D)
		t.Errorf("Expected D value after setting line with s-a-1 to 0, got %v", line.Value)
	}
}

// Helper: Create a test gate with inputs and output
func createTestGate(gateType circuit.GateType, name string) *circuit.Gate {
	gate := circuit.NewGate(1, name, gateType)

	// Create inputs and output
	in1 := circuit.NewLine(1, "in1", circuit.Normal)
	in2 := circuit.NewLine(2, "in2", circuit.Normal)
	out := circuit.NewLine(3, "out", circuit.Normal)

	// Add inputs
	gate.AddInput(in1)
	if gateType != circuit.NOT { // NOT gate has only one input
		gate.AddInput(in2)
	}

	// Set output
	gate.SetOutput(out)

	return gate
}

// Helper: Test gate evaluation with specific input values
func testGateEvaluation(t *testing.T, gate *circuit.Gate, inputValues []circuit.LogicValue, expectedOutput circuit.LogicValue) {
	// Set input values
	for i, value := range inputValues {
		gate.Inputs[i].SetValue(value)
	}

	// Evaluate gate
	result := gate.Evaluate()

	// Check result
	if result != expectedOutput {
		t.Errorf("Gate %s with inputs %v expected output %v, got %v",
			gate.Name, inputValues, expectedOutput, result)
	}
}

// Helper: Create a simple test circuit
func createTestCircuit() *circuit.Circuit {
	c := circuit.NewCircuit("test_circuit")

	// Create lines
	in1 := circuit.NewLine(0, "in1", circuit.PrimaryInput)
	in2 := circuit.NewLine(1, "in2", circuit.PrimaryInput)
	w1 := circuit.NewLine(2, "w1", circuit.Normal)
	w2 := circuit.NewLine(3, "w2", circuit.Normal)
	out := circuit.NewLine(4, "out", circuit.PrimaryOutput)

	// Add lines to circuit
	c.AddLine(in1)
	c.AddLine(in2)
	c.AddLine(w1)
	c.AddLine(w2)
	c.AddLine(out)

	// Create gates
	g1 := circuit.NewGate(0, "g1", circuit.AND)
	g2 := circuit.NewGate(1, "g2", circuit.OR)

	// Connect gates
	g1.AddInput(in1)
	g1.AddInput(in2)
	g1.SetOutput(w1)

	g2.AddInput(w1)
	g2.AddInput(in2)
	g2.SetOutput(out)

	// Add gates to circuit
	c.AddGate(g1)
	c.AddGate(g2)

	return c
}
