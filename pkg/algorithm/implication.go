package algorithm

import (
	"fmt"
	"strings"

	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
)

// Implication manages the implication operations for the FAN algorithm
type Implication struct {
	Circuit  *circuit.Circuit
	Logger   *utils.Logger
	Topo     *circuit.Topology
	Frontier *Frontier
}

// NewImplication creates a new Implication manager
func NewImplication(c *circuit.Circuit, f *Frontier, t *circuit.Topology, logger *utils.Logger) *Implication {
	return &Implication{
		Circuit:  c,
		Logger:   logger,
		Topo:     t,
		Frontier: f,
	}
}

// ImplyValues performs forward and backward implication until no more changes
func (i *Implication) ImplyValues() (bool, error) {
	i.Logger.Implication("Starting implication process")
	i.Logger.Indent()
	defer i.Logger.Outdent()

	changed := true
	iterations := 0

	// Continue until no more changes or conflict detected
	for changed && iterations < 100 { // Limit iterations to prevent infinite loops
		iterations++
		i.Logger.Trace("Implication iteration %d", iterations)

		// Forward propagation
		fwdChanged, err := i.ImplyForward()
		if err != nil {
			return false, err
		}

		// Backward justification
		bwdChanged, err := i.ImplyBackward()
		if err != nil {
			return false, err
		}

		// Update frontiers
		i.Frontier.UpdateDFrontier()
		i.Frontier.UpdateJFrontier()

		// Apply unique sensitization if D-frontier has a single gate
		usChanged := false
		if len(i.Frontier.DFrontier) == 1 {
			usChanged, err = i.ApplyUniqueSensitization(i.Frontier.DFrontier[0])
			if err != nil {
				return false, err
			}
		}

		changed = fwdChanged || bwdChanged || usChanged
	}

	i.Logger.Implication("Implication completed after %d iterations", iterations)

	// Check if there's a conflict in the current assignments
	if i.HasConflict() {
		i.Logger.Implication("Conflict detected during implication")
		return false, fmt.Errorf("value conflict detected during implication")
	}

	return true, nil
}

// ImplyForward performs forward implication (from inputs toward outputs)
func (i *Implication) ImplyForward() (bool, error) {
	// Use the circuit's built-in forward simulation
	changed := i.Circuit.SimulateForward()

	if changed {
		i.Logger.Trace("Forward implication made changes")
	}

	// Check for conflicts after implication
	if i.HasConflict() {
		return false, fmt.Errorf("conflict detected during forward implication")
	}

	return changed, nil
}

// ImplyBackward performs backward justification (from outputs toward inputs)
func (i *Implication) ImplyBackward() (bool, error) {
	changed := false

	// do backward imply for all gates
	for _, gate := range i.Circuit.Gates {
		if gate.Output.IsAssigned() {
			outputVal := gate.Output.Value

			switch gate.Type {
			case circuit.NOT:
				// NOT gate logic
				if len(gate.Inputs) == 1 && !gate.Inputs[0].IsAssigned() {
					var inputVal circuit.LogicValue
					switch outputVal {
					case circuit.Zero:
						inputVal = circuit.One
					case circuit.One:
						inputVal = circuit.Zero
					case circuit.D:
						inputVal = circuit.Dnot
					case circuit.Dnot:
						inputVal = circuit.D
					}

					if inputVal != circuit.X {
						gate.Inputs[0].SetValue(inputVal)
						changed = true
					}
				}

			case circuit.BUF:
				// BUF gate logic
				if len(gate.Inputs) == 1 && !gate.Inputs[0].IsAssigned() {
					gate.Inputs[0].SetValue(outputVal)
					changed = true
				}

			case circuit.AND, circuit.NAND:
				isAND := gate.Type == circuit.AND
				outputIsControl := (isAND && outputVal == circuit.Zero) ||
					(!isAND && outputVal == circuit.One)

				if outputIsControl {
					// Can't determine input values when output has controlling value
					continue
				}

				// For both AND and NAND, the non-controlling value is 1
				nonControlVal := circuit.One

				for _, input := range gate.Inputs {
					if !input.IsAssigned() {
						input.SetValue(nonControlVal)
						changed = true
					} else if input.Value != nonControlVal &&
						input.Value != circuit.D && input.Value != circuit.Dnot {
						return changed, fmt.Errorf("conflict in backward implication")
					}
				}

			case circuit.OR, circuit.NOR:
				isOR := gate.Type == circuit.OR
				outputIsControl := (isOR && outputVal == circuit.One) ||
					(!isOR && outputVal == circuit.Zero)

				if outputIsControl {
					// Can't determine input values when output has controlling value
					continue
				}

				// For both OR and NOR, the non-controlling value is 0
				nonControlVal := circuit.Zero

				for _, input := range gate.Inputs {
					if !input.IsAssigned() {
						input.SetValue(nonControlVal)
						changed = true
					} else if input.Value != nonControlVal &&
						input.Value != circuit.D && input.Value != circuit.Dnot {
						return changed, fmt.Errorf("conflict in backward implication")
					}
				}
			}
		}
	}

	if changed {
		i.Logger.Trace("Backward implication made changes")
	}

	if i.HasConflict() {
		return changed, fmt.Errorf("conflict detected during backward implication")
	}

	return changed, nil
}

// HasConflict checks for logical conflicts in the current circuit state
func (i *Implication) HasConflict() bool {
	// Check for conflicts in gate outputs (simulated vs. assigned)
	for _, gate := range i.Circuit.Gates {
		// Skip gates with unassigned outputs
		if !gate.Output.IsAssigned() {
			continue
		}

		// Skip gates with unassigned inputs (we can't simulate them)
		allInputsAssigned := true
		for _, input := range gate.Inputs {
			if !input.IsAssigned() {
				allInputsAssigned = false
				break
			}
		}

		if allInputsAssigned {
			// Simulate gate output
			simulated := gate.Evaluate()

			// Skip D/D' comparison to X since it's not a conflict
			if gate.Output.Value == circuit.X || simulated == circuit.X {
				continue
			}

			// Check for inconsistency between simulated and assigned
			if gate.Output.Value != simulated {
				i.Logger.Implication("Conflict detected: gate %s output is %v but should be %v",
					gate.Name, gate.Output.Value, simulated)
				return true
			}
		}
	}

	// For fault sites, handle special cases
	if i.Circuit.FaultSite != nil && i.Circuit.FaultSite.IsAssigned() {
		faultLine := i.Circuit.FaultSite
		faultType := i.Circuit.FaultType

		// Special case from first implementation:
		// If the good value equals the fault type, we have a conflict
		// This is because we're trying to set the line to its fault value
		// which means the fault won't be activated
		if faultLine.Value == faultType {
			i.Logger.Implication("Fault site conflict: trying to set fault site %s to its fault value %v",
				faultLine.Name, faultType)
			return true
		}

		// From second implementation: Check for values incompatible with fault type
		if faultType == circuit.Zero {
			// For s-a-0, only allowed values are 0 and D'
			// But we've already flagged 0 as a conflict above, so just check it's not 1 or D
			if faultLine.Value == circuit.One || faultLine.Value == circuit.D {
				i.Logger.Implication("Fault site conflict: s-a-0 fault site has invalid value %v",
					faultLine.Value)
				return true
			}
		} else if faultType == circuit.One {
			// For s-a-1, only allowed values are 1 and D
			// But we've already flagged 1 as a conflict above, so just check it's not 0 or D'
			if faultLine.Value == circuit.Zero || faultLine.Value == circuit.Dnot {
				i.Logger.Implication("Fault site conflict: s-a-1 fault site has invalid value %v",
					faultLine.Value)
				return true
			}
		}
	}

	return false
}

// ApplyUniqueSensitization implements the unique sensitization strategy from the FAN algorithm
func (i *Implication) ApplyUniqueSensitization(gate *circuit.Gate) (bool, error) {
	i.Logger.Implication("Attempting unique sensitization for gate %s", gate.Name)

	// Find all unique paths from the gate to primary outputs
	uniquePaths := i.Topo.FindUniquePathsToOutputs(gate)
	if len(uniquePaths) == 0 {
		i.Logger.Trace("No unique paths found for gate %s", gate.Name)
		return false, nil
	}

	i.Logger.Trace("Found %d unique paths that must be sensitized", len(uniquePaths))

	// Print paths for debugging
	for idx, path := range uniquePaths {
		pathNames := []string{}
		for _, line := range path {
			pathNames = append(pathNames, line.Name)
		}
		i.Logger.Trace("Path %d: %s", idx+1, strings.Join(pathNames, " → "))
	}

	changed := false

	// Process each path
	for _, path := range uniquePaths {
		// For each line in the path
		for _, line := range path {
			// Find the gate that produces this line (if any)
			if line.InputGate == nil {
				continue
			}

			// Skip the D-frontier gate itself
			if line.InputGate.ID == gate.ID {
				continue
			}

			inputGate := line.InputGate
			nonControllingValue := inputGate.GetNonControllingValue()

			// For non-X non-controlling values, set all unassigned side inputs
			if nonControllingValue != circuit.X {
				for _, input := range inputGate.Inputs {
					// Skip inputs that are part of the path we're sensitizing
					isOnPath := false
					for _, pathLine := range path {
						if input.ID == pathLine.ID {
							isOnPath = true
							break
						}
					}

					// Set unassigned side inputs to non-controlling values
					if !isOnPath && !input.IsAssigned() {
						i.Logger.Algorithm("Setting line %s to %v for unique sensitization",
							input.Name, nonControllingValue)
						input.SetValue(nonControllingValue)
						changed = true
					}
				}
			}
		}
	}

	return changed, nil
}

// JustifyLine attempts to justify a line to have a specific value
// Returns true if successful, false if impossible
func (i *Implication) JustifyLine(line *circuit.Line, targetValue circuit.LogicValue) (bool, error) {
	i.Logger.Implication("Attempting to justify line %s to value %v", line.Name, targetValue)

	// if the line already has the target value, return true
	if line.Value == targetValue {
		return true, nil
	}

	// if the line already has a different non-X value, return false
	if line.IsAssigned() && line.Value != targetValue {
		return false, fmt.Errorf("line %s already has conflicting value %v", line.Name, line.Value)
	}

	// save current state for rollback
	savedState := make(map[int]circuit.LogicValue)
	for id, line := range i.Circuit.Lines {
		savedState[id] = line.Value
	}

	// set the line to the target value
	line.SetValue(targetValue)

	// do implication to propagate the effect
	ok, err := i.ImplyValues()
	if !ok || err != nil {
		// if implication fails, restore the line values
		for id, val := range savedState {
			if l, exists := i.Circuit.Lines[id]; exists {
				l.Value = val
			}
		}
		return false, nil
	}

	return true, nil
}

// TryValueOnLine tries a specific value on a line and checks if it leads to conflicts
func (i *Implication) TryValueOnLine(line *circuit.Line, value circuit.LogicValue) (bool, error) {
	// Save the current circuit state
	savedValues := make(map[int]circuit.LogicValue)
	for id, line := range i.Circuit.Lines {
		savedValues[id] = line.Value
	}

	// Try setting the value
	line.SetValue(value)
	i.Logger.Trace("Trying value %v on line %s", value, line.Name)

	// Perform implication
	ok, err := i.ImplyValues()

	// If conflict or error, restore circuit and return false
	if !ok || err != nil {
		// Restore original values
		for id, val := range savedValues {
			i.Circuit.Lines[id].Value = val
		}
		i.Logger.Trace("Value %v on line %s leads to conflict", value, line.Name)
		return false, nil
	}

	i.Logger.Trace("Value %v on line %s is consistent", value, line.Name)
	return true, nil
}

// CheckIfXPathExists checks if a path exists from a D/D' value to a primary output
func (i *Implication) CheckIfXPathExists() bool {
	faultyLines := make([]*circuit.Line, 0)
	for _, line := range i.Circuit.Lines {
		if line.IsFaulty() {
			faultyLines = append(faultyLines, line)
		}
	}

	if len(faultyLines) == 0 {
		return false
	}

	// for every faulty line, check if there is a path to a primary output
	// where all gates can be sensitized (no blocking control values)
	for _, faultyLine := range faultyLines {
		// bfs search for the path
		visited := make(map[*circuit.Line]bool)
		queue := []*circuit.Line{faultyLine}

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			if current.Type == circuit.PrimaryOutput {
				// find the output path
				return true
			}

			if visited[current] {
				continue
			}
			visited[current] = true

			// check all gates connected to this line
			for _, gate := range current.OutputGates {
				// check if the gate can be sensitized
				if gate.IsSensitizable() {
					queue = append(queue, gate.Output)
				}
			}
		}
	}

	return false
}
