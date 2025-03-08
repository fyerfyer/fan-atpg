package utils

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/fyerfyer/fan-atpg/pkg/circuit"
)

// Regular expressions for parsing BENCH format
var (
	inputRegex  = regexp.MustCompile(`^INPUT\((\w+)\)$`)
	outputRegex = regexp.MustCompile(`^OUTPUT\((\w+)\)$`)
	gateRegex   = regexp.MustCompile(`^(\w+)\s*=\s*(\w+)\((.+)\)$`)
)

// ParseBenchFile reads a circuit description in BENCH format and returns a Circuit object
func ParseBenchFile(filename string) (*circuit.Circuit, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Extract circuit name from filename
	circuitName := strings.TrimSuffix(strings.Split(filename, "/")[len(strings.Split(filename, "/"))-1], ".bench")
	c := circuit.NewCircuit(circuitName)

	// Maps to store line names to their IDs for easy lookup
	lineMap := make(map[string]*circuit.Line)
	gateMap := make(map[string]*circuit.Gate)
	nextLineID := 0
	nextGateID := 0

	// First pass: identify all lines (inputs, outputs, and internal wires)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle INPUT declaration
		if matches := inputRegex.FindStringSubmatch(line); matches != nil {
			lineName := matches[1]
			if _, exists := lineMap[lineName]; !exists {
				l := circuit.NewLine(nextLineID, lineName, circuit.PrimaryInput)
				lineMap[lineName] = l
				c.AddLine(l)
				nextLineID++
			}
			continue
		}

		// Handle OUTPUT declaration
		if matches := outputRegex.FindStringSubmatch(line); matches != nil {
			lineName := matches[1]
			if l, exists := lineMap[lineName]; exists {
				l.Type = circuit.PrimaryOutput
			} else {
				l := circuit.NewLine(nextLineID, lineName, circuit.PrimaryOutput)
				lineMap[lineName] = l
				c.AddLine(l)
				nextLineID++
			}
			continue
		}

		// Handle gate declaration - just extract output line for now
		if matches := gateRegex.FindStringSubmatch(line); matches != nil {
			outputName := matches[1]
			if _, exists := lineMap[outputName]; !exists {
				l := circuit.NewLine(nextLineID, outputName, circuit.Normal)
				lineMap[outputName] = l
				c.AddLine(l)
				nextLineID++
			}

			// Identify input lines
			inputs := strings.Split(matches[3], ",")
			for _, inputName := range inputs {
				inputName = strings.TrimSpace(inputName)
				if _, exists := lineMap[inputName]; !exists {
					l := circuit.NewLine(nextLineID, inputName, circuit.Normal)
					lineMap[inputName] = l
					c.AddLine(l)
					nextLineID++
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Second pass: create gates and connect them
	file.Seek(0, 0) // Reset to beginning of file
	scanner = bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments, empty lines, and input/output declarations
		if line == "" || strings.HasPrefix(line, "#") ||
			inputRegex.MatchString(line) || outputRegex.MatchString(line) {
			continue
		}

		// Process gate declaration
		if matches := gateRegex.FindStringSubmatch(line); matches != nil {
			outputName := matches[1]
			gateTypeName := strings.ToUpper(matches[2])
			inputNames := strings.Split(matches[3], ",")

			// Create gate and connect it
			gate := circuit.NewGate(nextGateID, fmt.Sprintf("g%d", nextGateID), parseGateType(gateTypeName))
			gateMap[outputName] = gate
			nextGateID++

			// Connect output
			outputLine := lineMap[outputName]
			gate.SetOutput(outputLine)

			// Connect inputs
			for _, inputName := range inputNames {
				inputName = strings.TrimSpace(inputName)
				inputLine := lineMap[inputName]
				gate.AddInput(inputLine)
			}

			c.AddGate(gate)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Analyze circuit topology
	c.AnalyzeTopology()

	return c, nil
}

// parseGateType converts string gate type to GateType enum
func parseGateType(typeString string) circuit.GateType {
	switch typeString {
	case "AND":
		return circuit.AND
	case "OR":
		return circuit.OR
	case "NOT", "INV":
		return circuit.NOT
	case "NAND":
		return circuit.NAND
	case "NOR":
		return circuit.NOR
	case "XOR":
		return circuit.XOR
	case "XNOR":
		return circuit.XNOR
	case "BUF":
		return circuit.BUF
	default:
		// Default to buffer for unsupported gate types
		return circuit.BUF
	}
}

// ParseFaultString parses a fault string like "a/0" or "net34/1"
func ParseFaultString(faultStr string, c *circuit.Circuit) (*circuit.Line, circuit.LogicValue, error) {
	parts := strings.Split(faultStr, "/")
	if len(parts) != 2 {
		return nil, circuit.X, fmt.Errorf("invalid fault string format: %s", faultStr)
	}

	lineName := parts[0]
	var line *circuit.Line

	// Search for line by name
	for _, l := range c.Lines {
		if l.Name == lineName {
			line = l
			break
		}
	}

	if line == nil {
		return nil, circuit.X, fmt.Errorf("line not found: %s", lineName)
	}

	// Parse fault type (0 or 1)
	var faultType circuit.LogicValue
	if parts[1] == "0" {
		faultType = circuit.Zero
	} else if parts[1] == "1" {
		faultType = circuit.One
	} else {
		return nil, circuit.X, fmt.Errorf("invalid fault type: %s", parts[1])
	}

	return line, faultType, nil
}

// WriteTestVectors writes test vectors to a file
func WriteTestVectors(filename string, testVectors []map[string]circuit.LogicValue) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Get all input names from the first test vector
	var inputNames []string
	if len(testVectors) > 0 {
		for name := range testVectors[0] {
			inputNames = append(inputNames, name)
		}
	}

	// Write header
	writer.WriteString("# Test vectors generated by FAN-ATPG\n")
	writer.WriteString("# Format: ")
	for _, name := range inputNames {
		writer.WriteString(name + " ")
	}
	writer.WriteString("\n")

	// Write each test vector
	for i, vector := range testVectors {
		writer.WriteString(fmt.Sprintf("# Test vector %d\n", i+1))
		for _, name := range inputNames {
			value := vector[name]
			switch value {
			case circuit.Zero:
				writer.WriteString("0 ")
			case circuit.One:
				writer.WriteString("1 ")
			default:
				writer.WriteString("X ")
			}
		}
		writer.WriteString("\n")
	}

	return nil
}
