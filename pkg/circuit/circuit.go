package circuit

import (
	"fmt"
	"strings"
)

// Circuit represents a digital circuit consisting of gates and lines
type Circuit struct {
	Name      string
	Gates     map[int]*Gate
	Lines     map[int]*Line
	Inputs    []*Line
	Outputs   []*Line
	FaultSite *Line
	FaultType LogicValue // Stuck-at-0 (Zero) or stuck-at-1 (One)
	DFrontier []*Gate
	JFrontier []*Gate // Justification frontier
	HeadLines []*Line // Cached list of head lines
}

// NewCircuit creates a new circuit with the given name
func NewCircuit(name string) *Circuit {
	return &Circuit{
		Name:      name,
		Gates:     make(map[int]*Gate),
		Lines:     make(map[int]*Line),
		Inputs:    make([]*Line, 0),
		Outputs:   make([]*Line, 0),
		DFrontier: make([]*Gate, 0),
		JFrontier: make([]*Gate, 0),
		HeadLines: make([]*Line, 0),
	}
}

// AddGate adds a gate to the circuit
func (c *Circuit) AddGate(gate *Gate) {
	c.Gates[gate.ID] = gate
}

// AddLine adds a line to the circuit
func (c *Circuit) AddLine(line *Line) {
	c.Lines[line.ID] = line

	// Categorize inputs and outputs
	if line.Type == PrimaryInput {
		c.Inputs = append(c.Inputs, line)
	} else if line.Type == PrimaryOutput {
		c.Outputs = append(c.Outputs, line)
	}
}

// GetGate returns a gate by ID
func (c *Circuit) GetGate(id int) *Gate {
	return c.Gates[id]
}

// GetLine returns a line by ID
func (c *Circuit) GetLine(id int) *Line {
	return c.Lines[id]
}

// Reset resets all lines in the circuit to X
func (c *Circuit) Reset() {
	for _, line := range c.Lines {
		line.Reset()
	}
	c.FaultSite = nil
	c.FaultType = X
	c.DFrontier = make([]*Gate, 0)
	c.JFrontier = make([]*Gate, 0)
}

// InjectFault injects a fault into the circuit
func (c *Circuit) InjectFault(faultSite *Line, faultType LogicValue) {
	c.FaultSite = faultSite
	c.FaultType = faultType

	// Update the line's fault information
	faultSite.IsFaultSite = true
	faultSite.FaultType = faultType

	// If the fault site already has a value, adjust it
	if faultSite.Value != X {
		// Re-apply the current value which will convert to D/D' if needed
		currentValue := faultSite.Value
		faultSite.Value = X // Reset to avoid double counting assignments
		faultSite.SetValue(currentValue)
	}
}

// SimulateForward performs forward simulation from a specific starting point
func (c *Circuit) SimulateForward() bool {
	changed := false

	// Process gates in topological order (assumed here)
	for _, gate := range c.Gates {
		if gate.Output.Value == X {
			newValue := gate.Evaluate()
			if newValue != X {
				gate.Output.Value = newValue
				changed = true

				// Update D-frontier
				if newValue == X && gate.HasFaultyInput() {
					c.addToDFrontier(gate)
				}
			}
		}
	}

	return changed
}

// SimulateBackward performs backward justification
func (c *Circuit) SimulateBackward() bool {
	changed := false

	// Process gates in reverse topological order (assumed here)
	// This is a simplified version - in practice we'd need proper queue handling
	for _, gate := range c.Gates {
		if gate.Output.IsAssigned() {
			// Check if we can determine values for any gate inputs
			// based on the output value
			switch gate.Type {
			case NOT:
				if len(gate.Inputs) == 1 && !gate.Inputs[0].IsAssigned() {
					var inputVal LogicValue
					switch gate.Output.Value {
					case Zero:
						inputVal = One
					case One:
						inputVal = Zero
					case D:
						inputVal = Dnot
					case Dnot:
						inputVal = D
					}

					if inputVal != X {
						gate.Inputs[0].Value = inputVal
						changed = true
					}
				}

			case BUF:
				if len(gate.Inputs) == 1 && !gate.Inputs[0].IsAssigned() {
					gate.Inputs[0].Value = gate.Output.Value
					changed = true
				}

			case AND, NAND:
				// If output is controlling (0 for AND, 1 for NAND),
				// we can't determine input values
				if (gate.Type == AND && gate.Output.Value == Zero) ||
					(gate.Type == NAND && gate.Output.Value == One) {
					continue
				}

				// If output is non-controlling, all inputs must be 1 for AND, 0 for NAND
				nonControlVal := gate.GetNonControllingValue()
				for _, input := range gate.Inputs {
					if !input.IsAssigned() {
						input.Value = nonControlVal
						changed = true
					}
				}

			case OR, NOR:
				// If output is controlling (1 for OR, 0 for NOR),
				// we can't determine input values
				if (gate.Type == OR && gate.Output.Value == One) ||
					(gate.Type == NOR && gate.Output.Value == Zero) {
					continue
				}

				// If output is non-controlling, all inputs must be 0 for OR, 1 for NOR
				nonControlVal := gate.GetNonControllingValue()
				for _, input := range gate.Inputs {
					if !input.IsAssigned() {
						input.Value = nonControlVal
						changed = true
					}
				}

				// XOR and XNOR would need special handling for backward implication
			}
		}
	}

	return changed
}

// Implication performs both forward and backward implications until no more changes
func (c *Circuit) Implication() bool {
	changed := true
	anyChanged := false

	for changed {
		changed = c.SimulateForward()
		changed = c.SimulateBackward() || changed
		anyChanged = anyChanged || changed
	}

	return anyChanged
}

// UpdateDFrontier updates the D-frontier
func (c *Circuit) UpdateDFrontier() {
	c.DFrontier = make([]*Gate, 0)

	for _, gate := range c.Gates {
		// A gate is in D-frontier if:
		// 1. At least one input has D or D'
		// 2. Output is X
		if gate.HasFaultyInput() && gate.Output.Value == X {
			c.addToDFrontier(gate)
			gate.IsInDFrontier = true
		} else {
			gate.IsInDFrontier = false
		}
	}
}

// UpdateJFrontier updates the justification frontier
func (c *Circuit) UpdateJFrontier() {
	c.JFrontier = make([]*Gate, 0)

	for _, gate := range c.Gates {
		// A gate is in J-frontier if:
		// 1. Output is assigned
		// 2. At least one input is unassigned
		if gate.Output.IsAssigned() {
			for _, input := range gate.Inputs {
				if !input.IsAssigned() {
					c.JFrontier = append(c.JFrontier, gate)
					break
				}
			}
		}
	}
}

// addToDFrontier adds a gate to the D-frontier if not already present
func (c *Circuit) addToDFrontier(gate *Gate) {
	for _, g := range c.DFrontier {
		if g.ID == gate.ID {
			return
		}
	}
	c.DFrontier = append(c.DFrontier, gate)
}

// CheckTestStatus checks if current assignments constitute a test
func (c *Circuit) CheckTestStatus() bool {
	// A test is found when at least one primary output has D or D'
	for _, output := range c.Outputs {
		if output.IsFaulty() {
			return true
		}
	}
	return false
}

// AnalyzeTopology analyzes the circuit topology to identify free, bound, and head lines
func (c *Circuit) AnalyzeTopology() {
	// First, mark all lines as free
	for _, line := range c.Lines {
		line.IsFree = true
		line.IsBound = false
		line.IsHeadLine = false
	}

	// Find fanout points
	fanoutPoints := make([]*Line, 0)
	for _, line := range c.Lines {
		if len(line.OutputGates) > 1 {
			fanoutPoints = append(fanoutPoints, line)
		}
	}

	// Mark lines reachable from fanout points as bound
	for _, fanout := range fanoutPoints {
		// This is a simplified approach - in practice we would use
		// a more efficient graph traversal algorithm
		c.markReachableLines(fanout)
	}

	// Identify head lines (free lines adjacent to bound lines)
	c.HeadLines = make([]*Line, 0)
	for _, line := range c.Lines {
		if line.IsFree {
			// Check if this free line feeds into any bound line
			for _, gate := range line.OutputGates {
				if gate.Output.IsBound {
					line.IsHeadLine = true
					c.HeadLines = append(c.HeadLines, line)
					break
				}
			}
		}
	}
}

// markReachableLines marks all lines reachable from the given line as bound
func (c *Circuit) markReachableLines(startLine *Line) {
	startLine.IsBound = true
	startLine.IsFree = false

	// Process all gates this line feeds into
	for _, gate := range startLine.OutputGates {
		if !gate.Output.IsBound {
			gate.Output.IsBound = true
			gate.Output.IsFree = false

			// Recursively mark lines reachable from this gate's output
			c.markReachableLines(gate.Output)
		}
	}
}

// GetCurrentTest returns the current input test vector
func (c *Circuit) GetCurrentTest() map[string]LogicValue {
	test := make(map[string]LogicValue)
	for _, input := range c.Inputs {
		test[input.Name] = input.Value
	}
	return test
}

// String returns a string representation of the circuit state
func (c *Circuit) String() string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("Circuit: %s\n", c.Name))

	builder.WriteString("Inputs: ")
	for _, in := range c.Inputs {
		builder.WriteString(fmt.Sprintf("%s ", in))
	}

	builder.WriteString("\nOutputs: ")
	for _, out := range c.Outputs {
		builder.WriteString(fmt.Sprintf("%s ", out))
	}

	builder.WriteString("\nFault: ")
	if c.FaultSite != nil {
		if c.FaultType == Zero {
			builder.WriteString(fmt.Sprintf("%s stuck-at-0", c.FaultSite.Name))
		} else {
			builder.WriteString(fmt.Sprintf("%s stuck-at-1", c.FaultSite.Name))
		}
	} else {
		builder.WriteString("None")
	}

	builder.WriteString("\nD-Frontier: ")
	for _, gate := range c.DFrontier {
		builder.WriteString(fmt.Sprintf("%s ", gate.Name))
	}

	return builder.String()
}
