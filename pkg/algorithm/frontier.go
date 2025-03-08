package algorithm

import (
	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
	"sort"
	"strings"
)

// Frontier manages the D-frontier and J-frontier for the FAN algorithm
type Frontier struct {
	Circuit   *circuit.Circuit
	Logger    *utils.Logger
	DFrontier []*circuit.Gate // Gates with D/D' inputs and X output
	JFrontier []*circuit.Gate // Gates with assigned output and some unassigned inputs
}

// NewFrontier creates a new Frontier manager
func NewFrontier(c *circuit.Circuit, logger *utils.Logger) *Frontier {
	return &Frontier{
		Circuit:   c,
		Logger:    logger,
		DFrontier: make([]*circuit.Gate, 0),
		JFrontier: make([]*circuit.Gate, 0),
	}
}

// UpdateDFrontier identifies all gates in the D-frontier
func (f *Frontier) UpdateDFrontier() {
	f.DFrontier = make([]*circuit.Gate, 0)

	for _, gate := range f.Circuit.Gates {
		if f.isGateInDFrontier(gate) {
			f.DFrontier = append(f.DFrontier, gate)
			gate.IsInDFrontier = true
		} else {
			gate.IsInDFrontier = false
		}
	}

	f.Logger.Frontier("D-Frontier updated, now contains %d gates", len(f.DFrontier))
	for _, gate := range f.DFrontier {
		f.Logger.Trace("D-Frontier gate: %s, inputs: %v", gate.Name, f.getInputValuesString(gate))
	}
}

// getInputValuesString returns a string representation of gate input values
func (f *Frontier) getInputValuesString(gate *circuit.Gate) string {
	values := make([]string, len(gate.Inputs))
	for i, input := range gate.Inputs {
		values[i] = input.String()
	}
	return "[" + strings.Join(values, ", ") + "]"
}

// UpdateJFrontier identifies all gates in the J-frontier
func (f *Frontier) UpdateJFrontier() {
	f.JFrontier = make([]*circuit.Gate, 0)

	for _, gate := range f.Circuit.Gates {
		if f.isGateInJFrontier(gate) {
			f.JFrontier = append(f.JFrontier, gate)
		}
	}

	f.Logger.Frontier("J-Frontier updated, now contains %d gates", len(f.JFrontier))
}

// isGateInDFrontier checks if a gate belongs in the D-frontier
func (f *Frontier) isGateInDFrontier(gate *circuit.Gate) bool {
	// A gate is in D-frontier if:
	// 1. At least one input has D or D'
	// 2. Output is X
	// 3. The gate is sensitizable (all other inputs have non-controlling values)

	hasFaultyInput := false
	for _, input := range gate.Inputs {
		if input.IsFaulty() {
			hasFaultyInput = true
			break
		}
	}

	return hasFaultyInput && gate.Output.Value == circuit.X && gate.IsSensitizable()
}

// isGateInJFrontier checks if a gate belongs in the J-frontier
func (f *Frontier) isGateInJFrontier(gate *circuit.Gate) bool {
	// A gate is in J-frontier if:
	// 1. Output is assigned (not X)
	// 2. At least one input is not assigned (is X)

	if !gate.Output.IsAssigned() {
		return false
	}

	for _, input := range gate.Inputs {
		if !input.IsAssigned() {
			return true
		}
	}

	return false
}

// GetDFrontierGate selects the most appropriate gate from the D-frontier
// for fault propagation. The selection strategy can be varied.
func (f *Frontier) GetDFrontierGate() *circuit.Gate {
	if len(f.DFrontier) == 0 {
		return nil
	}

	// Simple strategy: choose the gate with the lowest number of inputs
	// This tends to be easier to sensitize
	sort.Slice(f.DFrontier, func(i, j int) bool {
		return len(f.DFrontier[i].Inputs) < len(f.DFrontier[j].Inputs)
	})

	return f.DFrontier[0]
}

// GetJFrontierGate selects the most appropriate gate from the J-frontier
// for justification. The selection strategy can be varied.
func (f *Frontier) GetJFrontierGate() *circuit.Gate {
	if len(f.JFrontier) == 0 {
		return nil
	}

	// Simple strategy: choose the gate with the fewest unassigned inputs
	sort.Slice(f.JFrontier, func(i, j int) bool {
		unassignedI := f.countUnassignedInputs(f.JFrontier[i])
		unassignedJ := f.countUnassignedInputs(f.JFrontier[j])
		return unassignedI < unassignedJ
	})

	return f.JFrontier[0]
}

// countUnassignedInputs counts the number of unassigned inputs for a gate
func (f *Frontier) countUnassignedInputs(gate *circuit.Gate) int {
	count := 0
	for _, input := range gate.Inputs {
		if !input.IsAssigned() {
			count++
		}
	}
	return count
}

// GetObjectivesFromDFrontier returns objectives for propagating fault through D-frontier
func (f *Frontier) GetObjectivesFromDFrontier() []InitialObjective {
	objectives := make([]InitialObjective, 0)

	// If D-frontier is empty, there's nothing to propagate
	if len(f.DFrontier) == 0 {
		return objectives
	}

	// Choose a gate from D-frontier for propagation
	gate := f.GetDFrontierGate()
	if gate == nil {
		return objectives
	}

	// For the chosen gate, we need to set all non-faulty inputs to non-controlling values
	nonControlValue := gate.GetNonControllingValue()

	for _, input := range gate.Inputs {
		if !input.IsFaulty() && !input.IsAssigned() {
			// Create an objective to set this input to the non-controlling value
			obj := InitialObjective{
				Line:  input,
				Value: nonControlValue,
			}
			objectives = append(objectives, obj)
		}
	}

	return objectives
}

// GetObjectivesFromJFrontier returns objectives for justifying values in J-frontier
func (f *Frontier) GetObjectivesFromJFrontier() []InitialObjective {
	objectives := make([]InitialObjective, 0)

	// If J-frontier is empty, there's nothing to justify
	if len(f.JFrontier) == 0 {
		return objectives
	}

	// Choose a gate from J-frontier for justification
	gate := f.GetJFrontierGate()
	if gate == nil {
		return objectives
	}

	// Based on gate type and output value, determine required input values
	switch gate.Type {
	case circuit.AND:
		if gate.Output.Value == circuit.One {
			// All inputs must be 1
			for _, input := range gate.Inputs {
				if !input.IsAssigned() {
					objectives = append(objectives, InitialObjective{
						Line:  input,
						Value: circuit.One,
					})
				}
			}
		} else if gate.Output.Value == circuit.Zero {
			// At least one input must be 0, choose an unassigned one
			for _, input := range gate.Inputs {
				if !input.IsAssigned() {
					objectives = append(objectives, InitialObjective{
						Line:  input,
						Value: circuit.Zero,
					})
					break
				}
			}
		}

	case circuit.OR:
		if gate.Output.Value == circuit.Zero {
			// All inputs must be 0
			for _, input := range gate.Inputs {
				if !input.IsAssigned() {
					objectives = append(objectives, InitialObjective{
						Line:  input,
						Value: circuit.Zero,
					})
				}
			}
		} else if gate.Output.Value == circuit.One {
			// At least one input must be 1, choose an unassigned one
			for _, input := range gate.Inputs {
				if !input.IsAssigned() {
					objectives = append(objectives, InitialObjective{
						Line:  input,
						Value: circuit.One,
					})
					break
				}
			}
		}

	case circuit.NOT:
		if len(gate.Inputs) == 1 && !gate.Inputs[0].IsAssigned() {
			var inputVal circuit.LogicValue
			switch gate.Output.Value {
			case circuit.Zero:
				inputVal = circuit.One
			case circuit.One:
				inputVal = circuit.Zero
			case circuit.D:
				inputVal = circuit.Dnot
			case circuit.Dnot:
				inputVal = circuit.D
			}

			objectives = append(objectives, InitialObjective{
				Line:  gate.Inputs[0],
				Value: inputVal,
			})
		}

	case circuit.NAND, circuit.NOR, circuit.XOR, circuit.XNOR:
		// Similar logic for other gate types
		// (simplified for brevity)
	}

	return objectives
}
