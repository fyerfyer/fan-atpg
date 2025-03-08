package circuit

import "fmt"

// GateType represents the type of logic gate
type GateType int

const (
	AND GateType = iota
	OR
	NOT
	NAND
	NOR
	XOR
	XNOR
	BUF // Buffer gate
)

// String returns a string representation of the gate type
func (gt GateType) String() string {
	switch gt {
	case AND:
		return "AND"
	case OR:
		return "OR"
	case NOT:
		return "NOT"
	case NAND:
		return "NAND"
	case NOR:
		return "NOR"
	case XOR:
		return "XOR"
	case XNOR:
		return "XNOR"
	case BUF:
		return "BUF"
	default:
		return "UNKNOWN"
	}
}

// Gate represents a logic gate in the circuit
type Gate struct {
	ID            int      // Unique identifier
	Name          string   // Name of the gate
	Type          GateType // Type of the gate
	Inputs        []*Line  // Input lines
	Output        *Line    // Output line
	ControlID     int      // ID of the easiest-to-control input (cached for efficiency)
	IsInDFrontier bool     // Whether the gate is in D-frontier
}

// NewGate creates a new gate with the given parameters
func NewGate(id int, name string, gateType GateType) *Gate {
	return &Gate{
		ID:            id,
		Name:          name,
		Type:          gateType,
		Inputs:        make([]*Line, 0),
		IsInDFrontier: false,
	}
}

// AddInput adds an input line to the gate
func (g *Gate) AddInput(line *Line) {
	g.Inputs = append(g.Inputs, line)
	line.AddOutputGate(g)
}

// SetOutput sets the output line of the gate
func (g *Gate) SetOutput(line *Line) {
	g.Output = line
	line.SetInputGate(g)
}

// String returns a string representation of the gate
func (g *Gate) String() string {
	return fmt.Sprintf("%s(%s)", g.Name, g.Type.String())
}

// Evaluate computes the output value of the gate based on its inputs
func (g *Gate) Evaluate() LogicValue {
	switch g.Type {
	case AND:
		return g.evaluateAND()
	case OR:
		return g.evaluateOR()
	case NOT:
		return g.evaluateNOT()
	case NAND:
		return g.evaluateNAND()
	case NOR:
		return g.evaluateNOR()
	case XOR:
		return g.evaluateXOR()
	case XNOR:
		return g.evaluateXNOR()
	case BUF:
		return g.evaluateBUF()
	default:
		return X
	}
}

func (g *Gate) evaluateAND() LogicValue {
	result := One
	hasDorDnot := false
	dType := X

	for _, input := range g.Inputs {
		switch input.Value {
		case Zero:
			return Zero // Short-circuit for AND gate
		case X:
			result = X
		case D:
			hasDorDnot = true
			dType = D
		case Dnot:
			hasDorDnot = true
			dType = Dnot
		}
	}

	if result == Zero {
		return Zero
	}

	if hasDorDnot && result != X {
		return dType
	}

	return result
}

func (g *Gate) evaluateOR() LogicValue {
	result := Zero
	hasDorDnot := false
	dType := X

	for _, input := range g.Inputs {
		switch input.Value {
		case One:
			return One // Short-circuit for OR gate
		case X:
			result = X
		case D:
			hasDorDnot = true
			dType = D
		case Dnot:
			hasDorDnot = true
			dType = Dnot
		}
	}

	if result == One {
		return One
	}

	if hasDorDnot && result != X {
		return dType
	}

	return result
}

func (g *Gate) evaluateNOT() LogicValue {
	if len(g.Inputs) != 1 {
		return X // Error case
	}

	switch g.Inputs[0].Value {
	case Zero:
		return One
	case One:
		return Zero
	case D:
		return Dnot
	case Dnot:
		return D
	default:
		return X
	}
}

func (g *Gate) evaluateNAND() LogicValue {
	val := g.evaluateAND()
	switch val {
	case Zero:
		return One
	case One:
		return Zero
	case D:
		return Dnot
	case Dnot:
		return D
	default:
		return X
	}
}

func (g *Gate) evaluateNOR() LogicValue {
	val := g.evaluateOR()
	switch val {
	case Zero:
		return One
	case One:
		return Zero
	case D:
		return Dnot
	case Dnot:
		return D
	default:
		return X
	}
}

func (g *Gate) evaluateXOR() LogicValue {
	// Simplified XOR for two inputs
	if len(g.Inputs) != 2 {
		return X // Not handling multi-input XOR
	}

	a := g.Inputs[0].Value
	b := g.Inputs[1].Value

	if a == X || b == X {
		return X
	}

	// Handle normal cases
	if (a == Zero && b == Zero) || (a == One && b == One) {
		return Zero
	}
	if (a == Zero && b == One) || (a == One && b == Zero) {
		return One
	}

	// Handle fault cases (simplified)
	if a.IsFaulty() || b.IsFaulty() {
		// For proper handling, we'd need to check the actual good and faulty values
		return X // Simplified - in real implementation we would calculate D or D'
	}

	return X
}

func (g *Gate) evaluateXNOR() LogicValue {
	val := g.evaluateXOR()
	switch val {
	case Zero:
		return One
	case One:
		return Zero
	case D:
		return Dnot
	case Dnot:
		return D
	default:
		return X
	}
}

func (g *Gate) evaluateBUF() LogicValue {
	if len(g.Inputs) != 1 {
		return X // Error case
	}
	return g.Inputs[0].Value
}

// IsInputsAssigned returns true if all inputs have non-X values
func (g *Gate) IsInputsAssigned() bool {
	for _, input := range g.Inputs {
		if !input.IsAssigned() {
			return false
		}
	}
	return true
}

// HasFaultyInput returns true if any input has a faulty value (D or D')
func (g *Gate) HasFaultyInput() bool {
	for _, input := range g.Inputs {
		if input.IsFaulty() {
			return true
		}
	}
	return false
}

// GetControllingValue returns the controlling value for the gate type
// (e.g., 0 for AND, 1 for OR)
func (g *Gate) GetControllingValue() LogicValue {
	switch g.Type {
	case AND, NAND:
		return Zero
	case OR, NOR:
		return One
	default:
		return X // No controlling value for NOT, XOR, etc.
	}
}

// GetNonControllingValue returns the non-controlling value for the gate type
// (e.g., 1 for AND, 0 for OR)
func (g *Gate) GetNonControllingValue() LogicValue {
	switch g.Type {
	case AND, NAND:
		return One
	case OR, NOR:
		return Zero
	default:
		return X // No non-controlling value for NOT, XOR, etc.
	}
}

// IsSensitizable returns true if the gate can propagate a fault from input to output
func (g *Gate) IsSensitizable() bool {
	switch g.Type {
	case AND, NAND, OR, NOR:
		// For these gates, all non-target inputs must have non-controlling values
		controllingValue := g.GetControllingValue()

		for _, input := range g.Inputs {
			if input.IsFaulty() {
				continue // Skip the faulty input
			}

			if input.Value == controllingValue {
				return false // A controlling value will block fault propagation
			}

			if input.Value == X {
				return false // Cannot determine if sensitizable with X values
			}
		}
		return true

	case NOT, BUF:
		return true // Always sensitizable

	case XOR, XNOR:
		// For XOR/XNOR, all other inputs must be known
		for _, input := range g.Inputs {
			if !input.IsFaulty() && input.Value == X {
				return false
			}
		}
		return true

	default:
		return false
	}
}

// FindEasiestControlInput determines and caches the easiest-to-control input
func (g *Gate) FindEasiestControlInput() int {
	// Simplified implementation - could be enhanced with testability metrics
	// For now just return the first input
	if len(g.Inputs) > 0 {
		g.ControlID = 0
		return 0
	}
	return -1
}

// IsFaulty returns true if the value is D or D'
func (v LogicValue) IsFaulty() bool {
	return v == D || v == Dnot
}
