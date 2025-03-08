package circuit

import (
	"fmt"
)

// LogicValue represents the possible values for a signal line
type LogicValue int

const (
	X    LogicValue = iota // Unknown/unassigned
	Zero                   // Logic 0
	One                    // Logic 1
	D                      // Good circuit: 0, Faulty circuit: 1
	Dnot                   // Good circuit: 1, Faulty circuit: 0
)

// String returns a string representation of the logic value
func (v LogicValue) String() string {
	switch v {
	case X:
		return "X"
	case Zero:
		return "0"
	case One:
		return "1"
	case D:
		return "D"
	case Dnot:
		return "D'"
	default:
		return "?"
	}
}

// LineType represents the classification of a line in the circuit
type LineType int

const (
	Normal LineType = iota
	PrimaryInput
	PrimaryOutput
	FaultSite
)

// Line represents a signal line in the circuit
type Line struct {
	ID          int        // Unique identifier
	Name        string     // Name of the line
	Value       LogicValue // Current value
	Type        LineType   // Type of the line
	InputGate   *Gate      // Gate driving this line (nil for primary inputs)
	OutputGates []*Gate    // Gates to which this line is an input

	// Topological properties - will be set during preprocessing
	IsFree     bool // True if not reachable from any fanout point
	IsBound    bool // True if reachable from a fanout point
	IsHeadLine bool // True if a free line adjacent to a bound line

	// For statistics and debugging
	AssignmentCount int // Number of times this line was assigned a value
}

// NewLine creates a new Line with the given name and ID
func NewLine(id int, name string, lineType LineType) *Line {
	return &Line{
		ID:          id,
		Name:        name,
		Value:       X,
		Type:        lineType,
		OutputGates: make([]*Gate, 0),
		IsFree:      true, // Default assumption, will be updated in topology analysis
		IsBound:     false,
		IsHeadLine:  false,
	}
}

// SetValue sets the logic value of the line
func (l *Line) SetValue(value LogicValue) {
	l.Value = value
	l.AssignmentCount++
}

// Reset resets the line value to X
func (l *Line) Reset() {
	l.Value = X
}

// String returns a string representation of the line
func (l *Line) String() string {
	return fmt.Sprintf("%s=%s", l.Name, l.Value)
}

// IsAssigned returns true if the line has a definite value (not X)
func (l *Line) IsAssigned() bool {
	return l.Value != X
}

// IsFaulty returns true if the line has a faulty value (D or D')
func (l *Line) IsFaulty() bool {
	return l.Value == D || l.Value == Dnot
}

// GetGoodValue returns the good circuit value (0 for D, 1 for D')
func (l *Line) GetGoodValue() LogicValue {
	switch l.Value {
	case D:
		return Zero
	case Dnot:
		return One
	default:
		return l.Value
	}
}

// GetFaultyValue returns the faulty circuit value (1 for D, 0 for D')
func (l *Line) GetFaultyValue() LogicValue {
	switch l.Value {
	case D:
		return One
	case Dnot:
		return Zero
	default:
		return l.Value
	}
}

// AddOutputGate adds a gate that this line feeds into
func (l *Line) AddOutputGate(gate *Gate) {
	l.OutputGates = append(l.OutputGates, gate)
}

// SetInputGate sets the gate that drives this line
func (l *Line) SetInputGate(gate *Gate) {
	l.InputGate = gate
}
