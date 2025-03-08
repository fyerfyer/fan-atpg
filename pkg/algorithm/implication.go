package algorithm

import (
	"fmt"
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
	// Use the circuit's built-in backward simulation
	changed := i.Circuit.SimulateBackward()

	if changed {
		i.Logger.Trace("Backward implication made changes")
	}

	// Check for conflicts after implication
	if i.HasConflict() {
		return false, fmt.Errorf("conflict detected during backward justification")
	}

	return changed, nil
}

// HasConflict checks for logical conflicts in the current circuit state
func (i *Implication) HasConflict() bool {
	// Check for conflicts at fault site
	if i.Circuit.FaultSite != nil {
		if i.Circuit.FaultSite.Value != circuit.X {
			goodValue := i.Circuit.FaultSite.GetGoodValue()
			//faultyValue := i.Circuit.FaultSite.GetFaultyValue()

			// If the good value equals the fault value, there's no way to detect the fault
			if goodValue == i.Circuit.FaultType {
				i.Logger.Implication("Conflict: fault site %s has good value equal to fault type %v",
					i.Circuit.FaultSite.Name, i.Circuit.FaultType)
				return true
			}
		}
	}

	// Check each gate to see if its output is consistent with its inputs
	for _, gate := range i.Circuit.Gates {
		if gate.IsInputsAssigned() && gate.Output.IsAssigned() {
			expectedOutput := gate.Evaluate()
			if expectedOutput != gate.Output.Value {
				i.Logger.Implication("Conflict: gate %s has inconsistent output %v, expected %v",
					gate.Name, gate.Output.Value, expectedOutput)
				return true
			}
		}
	}

	// Check if D-frontier has disappeared but no faulty signal reached outputs
	if len(i.Frontier.DFrontier) == 0 {
		// Check if any faulty signals reached outputs
		faultyOutputExists := false
		for _, output := range i.Circuit.Outputs {
			if output.IsFaulty() {
				faultyOutputExists = true
				break
			}
		}

		// Check if any internal line has faulty value
		faultySignalExists := false
		for _, line := range i.Circuit.Lines {
			if line.IsFaulty() {
				faultySignalExists = true
				break
			}
		}

		// If we have faulty signals but none at outputs and no D-frontier,
		// then the fault effect is blocked
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

	// Find unique paths from this gate to primary outputs
	uniquePaths := i.Topo.FindUniquePathsToOutputs(gate)
	if len(uniquePaths) == 0 {
		i.Logger.Trace("No unique paths found for gate %s", gate.Name)
		return false, nil
	}

	i.Logger.Trace("Found %d unique paths that must be sensitized", len(uniquePaths))

	// For each line in the unique paths, we need to sensitize gates along the path
	changed := false

	// Group lines by their input gate (to handle reconvergent paths)
	linesByInputGate := make(map[*circuit.Gate][]*circuit.Line)
	for _, line := range uniquePaths {
		if line.InputGate != nil {
			linesByInputGate[line.InputGate] = append(linesByInputGate[line.InputGate], line)
		}
	}

	// For each gate, set its side inputs to non-controlling values
	for gate, lines := range linesByInputGate {
		// Skip if the gate isn't on the path or is the fault site
		shouldSensitize := false
		for _, line := range lines {
			if line.IsFaulty() {
				shouldSensitize = true
				break
			}
		}

		if !shouldSensitize {
			continue
		}

		// Sensitize the gate by setting all other inputs to non-controlling values
		nonControllingValue := gate.GetNonControllingValue()
		if nonControllingValue == circuit.X {
			// Skip gates like XOR that don't have a simple non-controlling value
			continue
		}

		for _, input := range gate.Inputs {
			// Skip inputs that are already in the faulty path
			inputIsInPath := false
			for _, pathLine := range uniquePaths {
				if input.ID == pathLine.ID {
					inputIsInPath = true
					break
				}
			}

			if !inputIsInPath && !input.IsAssigned() {
				i.Logger.Trace("Setting line %s to non-controlling value %v to sensitize path",
					input.Name, nonControllingValue)
				input.SetValue(nonControllingValue)
				changed = true
			}
		}
	}

	// If we made changes, perform implication to propagate the effects
	if changed {
		i.Circuit.Implication()
		i.Frontier.UpdateDFrontier()
		i.Frontier.UpdateJFrontier()
	}

	return changed, nil
}

// JustifyLine attempts to justify a line to have a specific value
// Returns true if successful, false if impossible
func (i *Implication) JustifyLine(line *circuit.Line, targetValue circuit.LogicValue) (bool, error) {
	i.Logger.Implication("Attempting to justify line %s to value %v", line.Name, targetValue)

	// If line already has the target value, we're done
	if line.Value == targetValue {
		return true, nil
	}

	// If line has a different non-X value, justification is impossible
	if line.IsAssigned() && line.Value != targetValue {
		return false, fmt.Errorf("line %s already has conflicting value %v", line.Name, line.Value)
	}

	// Set the line to the target value
	line.SetValue(targetValue)

	// Perform implication to propagate the effects
	ok, err := i.ImplyValues()
	if !ok || err != nil {
		// Reset the line value if justification failed
		line.Value = circuit.X
		return false, err
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
	// Find all lines with D or D'
	faultyLines := make([]*circuit.Line, 0)
	for _, line := range i.Circuit.Lines {
		if line.IsFaulty() {
			faultyLines = append(faultyLines, line)
		}
	}

	if len(faultyLines) == 0 {
		return false
	}

	// For each faulty line, check if there's a path to a primary output
	// where all gates can be sensitized (no controlling values blocking)
	for _, faultyLine := range faultyLines {
		// Simple BFS to find a path
		visited := make(map[*circuit.Line]bool)
		queue := []*circuit.Line{faultyLine}

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			if current.Type == circuit.PrimaryOutput {
				return true // Found path to output
			}

			if visited[current] {
				continue
			}
			visited[current] = true

			// Check all gates this line feeds into
			for _, gate := range current.OutputGates {
				// Skip if output is already determined and blocks propagation
				if gate.Output.IsAssigned() && !gate.Output.IsFaulty() {
					continue
				}

				// Check if gate can be sensitized (no controlling values on other inputs)
				canBeSensitized := true
				for _, input := range gate.Inputs {
					if input == current {
						continue // Skip the current input
					}

					if input.IsAssigned() && input.Value == gate.GetControllingValue() {
						canBeSensitized = false
						break
					}
				}

				if canBeSensitized {
					queue = append(queue, gate.Output)
				}
			}
		}
	}

	return false
}
