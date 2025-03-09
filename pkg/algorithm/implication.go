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
	// detect conflict in the circuit
	if i.Circuit.FaultSite != nil && i.Circuit.FaultSite.Value != circuit.X {
		goodValue := i.Circuit.FaultSite.GetGoodValue()

		// if the good value is the same as the fault type, it's a conflict
		if goodValue == i.Circuit.FaultType {
			i.Logger.Implication("Conflict: fault site %s has good value equal to fault type %v",
				i.Circuit.FaultSite.Name, i.Circuit.FaultType)
			return true
		}
	}

	// check if any gate has inconsistent output
	for _, gate := range i.Circuit.Gates {
		if gate.IsInputsAssigned() && gate.Output.IsAssigned() {
			expectedOutput := gate.Evaluate()
			if expectedOutput != gate.Output.Value {
				i.Logger.Implication("Conflict: gate %s has inconsistent output %v, expected %v",
					gate.Name, gate.Output.Value, expectedOutput)
				return true
			}
		}

		switch gate.Type {
		case circuit.AND:
			if gate.Output.Value == circuit.One {
				for _, input := range gate.Inputs {
					if input.Value == circuit.Zero {
						i.Logger.Implication("Conflict: AND gate %s has output 1 but input %s is 0",
							gate.Name, input.Name)
						return true
					}
				}
			}
		case circuit.OR:
			if gate.Output.Value == circuit.Zero {
				for _, input := range gate.Inputs {
					if input.Value == circuit.One {
						i.Logger.Implication("Conflict: OR gate %s has output 0 but input %s is 1",
							gate.Name, input.Name)
						return true
					}
				}
			}
		}
	}

	// check if D-frontier has disappeared but fault effect hasn't reached outputs
	if len(i.Frontier.DFrontier) == 0 {
		// check if any output has a fault value
		faultyOutputExists := false
		for _, output := range i.Circuit.Outputs {
			if output.IsFaulty() {
				faultyOutputExists = true
				break
			}
		}

		// check if any internal line has a fault value
		faultySignalExists := false
		for _, line := range i.Circuit.Lines {
			if line.IsFaulty() {
				faultySignalExists = true
				break
			}
		}

		// if there are faulty signals but no output has a fault value,
		// the fault effect should be blocked
		if faultySignalExists && !faultyOutputExists {
			i.Logger.Implication("Conflict: D-frontier has disappeared without fault effect reaching outputs")
			return true
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
