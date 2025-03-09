package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
)

// TestParseBenchFile tests parsing a BENCH format circuit description
func TestParseBenchFile(t *testing.T) {
	// Create a temporary BENCH file
	tempDir := t.TempDir()
	benchFile := filepath.Join(tempDir, "test_circuit.bench")

	benchContent := `# Simple test circuit
INPUT(a)
INPUT(b)
OUTPUT(f)
d = AND(a, b)
e = NOT(b)
f = OR(d, e)
`
	err := os.WriteFile(benchFile, []byte(benchContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test BENCH file: %v", err)
	}

	// Parse the BENCH file
	c, err := utils.ParseBenchFile(benchFile)
	if err != nil {
		t.Fatalf("Failed to parse BENCH file: %v", err)
	}

	// Test circuit name extraction
	if c.Name != "test_circuit" {
		t.Errorf("Expected circuit name 'test_circuit', got '%s'", c.Name)
	}

	// Test component counts
	if len(c.Gates) != 3 {
		t.Errorf("Expected 3 gates, got %d", len(c.Gates))
	}
	if len(c.Lines) != 5 {
		t.Errorf("Expected 5 lines, got %d", len(c.Lines))
	}
	if len(c.Inputs) != 2 {
		t.Errorf("Expected 2 inputs, got %d", len(c.Inputs))
	}
	if len(c.Outputs) != 1 {
		t.Errorf("Expected 1 output, got %d", len(c.Outputs))
	}

	// Check specific components
	var foundInputA, foundInputB, foundOutputF bool
	var foundANDGate, foundNOTGate, foundORGate bool

	for _, line := range c.Lines {
		if line.Name == "a" && line.Type == circuit.PrimaryInput {
			foundInputA = true
		}
		if line.Name == "b" && line.Type == circuit.PrimaryInput {
			foundInputB = true
		}
		if line.Name == "f" && line.Type == circuit.PrimaryOutput {
			foundOutputF = true
		}
	}

	for _, gate := range c.Gates {
		if gate.Type == circuit.AND && len(gate.Inputs) == 2 && gate.Output.Name == "d" {
			foundANDGate = true
		}
		if gate.Type == circuit.NOT && len(gate.Inputs) == 1 && gate.Output.Name == "e" {
			foundNOTGate = true
		}
		if gate.Type == circuit.OR && len(gate.Inputs) == 2 && gate.Output.Name == "f" {
			foundORGate = true
		}
	}

	// Verify all components were found
	if !foundInputA {
		t.Error("Input 'a' not found or not properly identified")
	}
	if !foundInputB {
		t.Error("Input 'b' not found or not properly identified")
	}
	if !foundOutputF {
		t.Error("Output 'f' not found or not properly identified")
	}
	if !foundANDGate {
		t.Error("AND gate not found or not properly identified")
	}
	if !foundNOTGate {
		t.Error("NOT gate not found or not properly identified")
	}
	if !foundORGate {
		t.Error("OR gate not found or not properly identified")
	}

	// Verify connections
	// Find the OR gate and check its inputs
	var orGate *circuit.Gate
	for _, gate := range c.Gates {
		if gate.Type == circuit.OR && gate.Output.Name == "f" {
			orGate = gate
			break
		}
	}

	if orGate != nil && len(orGate.Inputs) == 2 {
		// OR gate should have inputs named 'd' and 'e'
		inputNames := []string{orGate.Inputs[0].Name, orGate.Inputs[1].Name}
		found_d := false
		found_e := false
		for _, name := range inputNames {
			if name == "d" {
				found_d = true
			}
			if name == "e" {
				found_e = true
			}
		}
		if !found_d || !found_e {
			t.Errorf("OR gate inputs incorrect. Expected 'd' and 'e', got %v", inputNames)
		}
	} else {
		t.Error("OR gate not found or has incorrect number of inputs")
	}
}

// TestParseInvalidBenchFile tests error handling for invalid BENCH files
func TestParseInvalidBenchFile(t *testing.T) {
	// Test non-existent file
	_, err := utils.ParseBenchFile("non_existent_file.bench")
	if err == nil {
		t.Error("Expected error when parsing non-existent file, got nil")
	}
}

// TestParseComplexBenchFile tests parsing a more complex BENCH file with more gate types
func TestParseComplexBenchFile(t *testing.T) {
	tempDir := t.TempDir()
	benchFile := filepath.Join(tempDir, "complex.bench")

	benchContent := `# Complex test circuit with more gate types
INPUT(a)
INPUT(b)
INPUT(c)
OUTPUT(y)
OUTPUT(z)

# Gates with various types
d = AND(a, b)
e = OR(b, c)
f = NAND(c, d)
g = NOR(a, e)
h = XOR(d, e)
i = XNOR(f, g)
j = NOT(h)
y = BUF(i)
z = AND(j, a)
`
	err := os.WriteFile(benchFile, []byte(benchContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create complex BENCH file: %v", err)
	}

	c, err := utils.ParseBenchFile(benchFile)
	if err != nil {
		t.Fatalf("Failed to parse complex BENCH file: %v", err)
	}

	// Check gate types
	gateTypeCount := make(map[circuit.GateType]int)
	for _, gate := range c.Gates {
		gateTypeCount[gate.Type]++
	}

	expectedCounts := map[circuit.GateType]int{
		circuit.AND:  2,
		circuit.OR:   1,
		circuit.NAND: 1,
		circuit.NOR:  1,
		circuit.XOR:  1,
		circuit.XNOR: 1,
		circuit.NOT:  1,
		circuit.BUF:  1,
	}

	for gateType, expectedCount := range expectedCounts {
		if gateTypeCount[gateType] != expectedCount {
			t.Errorf("Expected %d gates of type %s, got %d",
				expectedCount, gateType.String(), gateTypeCount[gateType])
		}
	}
}

// TestParseFaultString tests parsing fault strings
func TestParseFaultString(t *testing.T) {
	// Create a simple circuit for testing
	c := circuit.NewCircuit("test")

	// Add some lines
	a := circuit.NewLine(0, "a", circuit.PrimaryInput)
	b := circuit.NewLine(1, "b", circuit.PrimaryInput)
	c.AddLine(a)
	c.AddLine(b)

	// Test valid fault string
	line, faultType, err := utils.ParseFaultString("a/0", c)
	if err != nil {
		t.Errorf("Failed to parse valid fault string: %v", err)
	}
	if line.Name != "a" {
		t.Errorf("Expected line name 'a', got '%s'", line.Name)
	}
	if faultType != circuit.Zero {
		t.Errorf("Expected fault type Zero, got %v", faultType)
	}

	// Test fault type 1
	line, faultType, err = utils.ParseFaultString("b/1", c)
	if err != nil {
		t.Errorf("Failed to parse valid fault string: %v", err)
	}
	if line.Name != "b" {
		t.Errorf("Expected line name 'b', got '%s'", line.Name)
	}
	if faultType != circuit.One {
		t.Errorf("Expected fault type One, got %v", faultType)
	}

	// Test invalid format
	_, _, err = utils.ParseFaultString("invalid", c)
	if err == nil {
		t.Error("Expected error for invalid fault string format, got nil")
	}

	// Test non-existent line
	_, _, err = utils.ParseFaultString("nonexistent/0", c)
	if err == nil {
		t.Error("Expected error for non-existent line, got nil")
	}

	// Test invalid fault type
	_, _, err = utils.ParseFaultString("a/2", c)
	if err == nil {
		t.Error("Expected error for invalid fault type, got nil")
	}
}

// TestWriteTestVectors tests writing test vectors to a file
func TestWriteTestVectors(t *testing.T) {
	// Create test vectors
	vector1 := map[string]circuit.LogicValue{
		"a": circuit.Zero,
		"b": circuit.One,
	}
	vector2 := map[string]circuit.LogicValue{
		"a": circuit.One,
		"b": circuit.Zero,
	}
	testVectors := []map[string]circuit.LogicValue{vector1, vector2}

	// Write to temporary file
	tempFile := filepath.Join(t.TempDir(), "test_vectors.txt")
	err := utils.WriteTestVectors(tempFile, testVectors)
	if err != nil {
		t.Fatalf("Failed to write test vectors: %v", err)
	}

	// Read back the file
	content, err := os.ReadFile(tempFile)
	if err != nil {
		t.Fatalf("Failed to read test vectors file: %v", err)
	}

	// Basic verification of content
	contentStr := string(content)
	if !contains(contentStr, "# Test vectors generated by FAN-ATPG") {
		t.Error("Output file missing header")
	}
	if !contains(contentStr, "# Test vector 1") {
		t.Error("Output file missing first test vector marker")
	}
	if !contains(contentStr, "# Test vector 2") {
		t.Error("Output file missing second test vector marker")
	}

	// Check for input values (simplified check)
	if !contains(contentStr, "0 1") && !contains(contentStr, "1 0") {
		t.Error("Output file missing expected test vector values")
	}
}

// Helper function: contains checks if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
