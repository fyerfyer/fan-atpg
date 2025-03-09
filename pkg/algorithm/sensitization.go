package algorithm

import (
	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
)

// Sensitization manages the path sensitization operations for the FAN algorithm
type Sensitization struct {
	Circuit     *circuit.Circuit
	Logger      *utils.Logger
	Topology    *circuit.Topology
	Implication *Implication
	Frontier    *Frontier
}

// NewSensitization creates a new Sensitization manager
func NewSensitization(c *circuit.Circuit, t *circuit.Topology, i *Implication, f *Frontier, logger *utils.Logger) *Sensitization {
	return &Sensitization{
		Circuit:     c,
		Logger:      logger,
		Topology:    t,
		Implication: i,
		Frontier:    f,
	}
}

// ApplyUniqueSensitization applies unique sensitization from a gate in the D-frontier
// This is a key optimization in the FAN algorithm to reduce backtracking
func (s *Sensitization) ApplyUniqueSensitization(gate *circuit.Gate) (bool, error) {
	s.Logger.Algorithm("Attempting unique sensitization for gate %s", gate.Name)

	// Find unique paths from this gate to primary outputs
	uniquePaths := s.Topology.FindUniquePathsToOutputs(gate)
	if len(uniquePaths) == 0 {
		s.Logger.Trace("No unique paths found for gate %s", gate.Name)
		return false, nil
	}

	s.Logger.Trace("Found %d unique paths that must be sensitized", len(uniquePaths))

	// Attempt to sensitize each path
	changed, err := s.SensitizePathsToOutputs(gate, uniquePaths)
	if err != nil {
		return false, err
	}

	// If we've made changes, perform implication and update frontiers
	if changed {
		_, err := s.Implication.ImplyValues()
		if err != nil {
			return false, err
		}

		s.Frontier.UpdateDFrontier()
		s.Frontier.UpdateJFrontier()
	}

	return changed, nil
}

// Update the SensitizePathsToOutputs function signature
func (s *Sensitization) SensitizePathsToOutputs(sourceGate *circuit.Gate, paths [][]*circuit.Line) (bool, error) {
	s.Logger.Trace("Sensitizing paths from gate %s to outputs", sourceGate.Name)
	changed := false

	// First, collect all lines that are part of any path
	pathLines := make(map[*circuit.Line]bool)
	for _, path := range paths {
		for _, line := range path {
			pathLines[line] = true
		}
	}

	// For each path
	for _, path := range paths {
		for i := 0; i < len(path); i++ {
			line := path[i]

			// Find gates this line feeds into
			for _, gate := range line.OutputGates {
				// For all gates except the source gate of the fault
				if gate.ID == sourceGate.ID {
					continue
				}

				// Get non-controlling value for this gate
				nonControlVal := gate.GetNonControllingValue()
				if nonControlVal == circuit.X {
					continue // Skip gates with no clear non-controlling value
				}

				// Set all side inputs (non-path inputs) to non-controlling values
				for _, input := range gate.Inputs {
					// Skip if this input is part of any path or already assigned
					if pathLines[input] || input.IsAssigned() {
						continue
					}

					s.Logger.Algorithm("Setting side input %s to non-controlling value %v for gate %s",
						input.Name, nonControlVal, gate.Name)
					input.SetValue(nonControlVal)
					changed = true
				}
			}
		}
	}

	return changed, nil
}

// IdentifySensitizableGates finds gates that can be sensitized to propagate faults
func (s *Sensitization) IdentifySensitizableGates() []*circuit.Gate {
	result := make([]*circuit.Gate, 0)

	// For each gate in D-frontier
	for _, gate := range s.Frontier.DFrontier {
		// Check if there's a path to output
		paths := s.Topology.FindUniquePathsToOutputs(gate)
		if len(paths) > 0 {
			result = append(result, gate)
		}
	}

	return result
}

// Update the TrySensitizePath function
func (s *Sensitization) TrySensitizePath(gate *circuit.Gate) (bool, error) {
	// Find all paths from this gate to primary outputs
	allPaths := s.Topology.FindUniquePathsToOutputs(gate)
	if len(allPaths) == 0 {
		return false, nil
	}

	// Try to sensitize the paths
	return s.SensitizePathsToOutputs(gate, allPaths)
}

// FindCriticalInputs identifies inputs that are critical for fault propagation
// These are inputs that must be set to specific values to propagate the fault
func (s *Sensitization) FindCriticalInputs() []InitialObjective {
	objectives := make([]InitialObjective, 0)

	// For each gate in D-frontier
	for _, gate := range s.Frontier.DFrontier {
		// Find all inputs that need to be set to non-controlling values
		nonControlVal := gate.GetNonControllingValue()
		if nonControlVal == circuit.X {
			continue
		}

		for _, input := range gate.Inputs {
			if !input.IsFaulty() && !input.IsAssigned() {
				objectives = append(objectives, InitialObjective{
					Line:  input,
					Value: nonControlVal,
				})
			}
		}
	}

	return objectives
}

// IsPathSensitized checks if a path from a line to primary outputs is sensitized
func (s *Sensitization) IsPathSensitized(line *circuit.Line) bool {
	// If line is already a primary output, it's sensitized
	if line.Type == circuit.PrimaryOutput {
		return true
	}

	// Check all gates this line feeds into
	for _, gate := range line.OutputGates {
		// Check if this gate is sensitized (all other inputs have non-controlling values)
		isSensitized := true

		for _, input := range gate.Inputs {
			// Skip the current line
			if input.ID == line.ID {
				continue
			}

			// If any other input has a controlling value or is X, gate is not sensitized
			if !input.IsAssigned() || input.Value == gate.GetControllingValue() {
				isSensitized = false
				break
			}
		}

		// If this gate is sensitized, check if there's a sensitized path from its output
		if isSensitized && s.IsPathSensitized(gate.Output) {
			return true
		}
	}

	return false
}

// Fix the GetSensitizationObjectives function
func (s *Sensitization) GetSensitizationObjectives() []InitialObjective {
	objectives := make([]InitialObjective, 0)

	// For each gate in D-frontier
	for _, gate := range s.Frontier.DFrontier {
		// Find paths that must be sensitized
		pathsList := s.Topology.FindUniquePathsToOutputs(gate)

		// For each path, identify gates that need side inputs set
		for _, path := range pathsList {
			for _, line := range path {
				if line.InputGate == nil {
					continue
				}

				// For each gate that this line feeds into
				for _, nextGate := range line.OutputGates {
					nonControlVal := nextGate.GetNonControllingValue()
					if nonControlVal == circuit.X {
						continue
					}

					// Set all other inputs to non-controlling values
					for _, input := range nextGate.Inputs {
						if input.ID != line.ID && !input.IsAssigned() {
							objectives = append(objectives, InitialObjective{
								Line:  input,
								Value: nonControlVal,
							})
						}
					}
				}
			}
		}
	}

	return objectives
}
