package circuit

import (
	"sort"
)

// Topology contains information about the circuit structure
type Topology struct {
	Circuit       *Circuit
	LevelMap      map[*Line]int  // Map of lines to their level in the circuit
	MaxLevel      int            // Maximum level in the circuit
	FanoutPoints  []*Line        // Lines that fan out to multiple gates
	ReconvPoints  map[*Line]bool // Lines where reconvergence occurs
	HeadLinesList []*Line        // Pre-computed list of head lines
}

// NewTopology creates a new topology analyzer for the given circuit
func NewTopology(c *Circuit) *Topology {
	return &Topology{
		Circuit:      c,
		LevelMap:     make(map[*Line]int),
		ReconvPoints: make(map[*Line]bool),
	}
}

// Analyze performs a complete topological analysis of the circuit
func (t *Topology) Analyze() {
	// Compute levels
	t.ComputeLevels()

	// Identify fanout points
	t.IdentifyFanoutPoints()

	// Identify free and bound regions
	t.IdentifyFreeAndBoundRegions()

	// Identify reconvergent paths
	t.IdentifyReconvergentPaths()

	// Pre-compute head lines list
	t.IdentifyHeadLines()
}

// ComputeLevels assigns a level to each line in the circuit
// Primary inputs are level 0, and levels increase toward outputs
func (t *Topology) ComputeLevels() {
	// Start with primary inputs at level 0
	for _, input := range t.Circuit.Inputs {
		t.LevelMap[input] = 0
	}

	// Keep processing gates until all lines have levels
	changed := true
	for changed {
		changed = false

		for _, gate := range t.Circuit.Gates {
			// Skip if output already has a level
			if _, hasLevel := t.LevelMap[gate.Output]; hasLevel {
				continue
			}

			// Check if all inputs have levels
			allInputsHaveLevels := true
			maxInputLevel := -1

			for _, input := range gate.Inputs {
				if level, exists := t.LevelMap[input]; exists {
					if level > maxInputLevel {
						maxInputLevel = level
					}
				} else {
					allInputsHaveLevels = false
					break
				}
			}

			// If all inputs have levels, assign level to output
			if allInputsHaveLevels {
				t.LevelMap[gate.Output] = maxInputLevel + 1
				if maxInputLevel+1 > t.MaxLevel {
					t.MaxLevel = maxInputLevel + 1
				}
				changed = true
			}
		}
	}
}

// IdentifyFanoutPoints identifies all fanout points in the circuit
func (t *Topology) IdentifyFanoutPoints() {
	t.FanoutPoints = make([]*Line, 0)

	for _, line := range t.Circuit.Lines {
		if len(line.OutputGates) > 1 {
			t.FanoutPoints = append(t.FanoutPoints, line)
		}
	}
}

// IdentifyFreeAndBoundRegions marks lines as free or bound
func (t *Topology) IdentifyFreeAndBoundRegions() {
	// Reset all lines to free initially
	for _, line := range t.Circuit.Lines {
		line.IsFree = true
		line.IsBound = false
	}

	// Mark all lines reachable from fanout points as bound
	for _, fanout := range t.FanoutPoints {
		t.markReachableLines(fanout)
	}
}

// markReachableLines marks all lines reachable from a starting line as bound
func (t *Topology) markReachableLines(startLine *Line) {
	// Skip if already processed
	if startLine.IsBound {
		return
	}

	startLine.IsBound = true
	startLine.IsFree = false

	// Mark all lines reachable from this line
	for _, gate := range startLine.OutputGates {
		t.markReachableLines(gate.Output)
	}
}

// IdentifyHeadLines identifies all head lines in the circuit
// (free lines that are adjacent to bound lines)
func (t *Topology) IdentifyHeadLines() {
	t.HeadLinesList = make([]*Line, 0)

	for _, line := range t.Circuit.Lines {
		if !line.IsFree {
			continue
		}

		// Check if this free line feeds into any gates that produce bound lines
		for _, gate := range line.OutputGates {
			if gate.Output.IsBound {
				line.IsHeadLine = true
				t.HeadLinesList = append(t.HeadLinesList, line)
				break
			}
		}
	}

	// Sort head lines by level for better decision making
	sort.Slice(t.HeadLinesList, func(i, j int) bool {
		return t.LevelMap[t.HeadLinesList[i]] < t.LevelMap[t.HeadLinesList[j]]
	})
}

// IdentifyReconvergentPaths identifies reconvergent paths in the circuit
func (t *Topology) IdentifyReconvergentPaths() {
	// For each fanout point, trace forward to find reconvergence points
	for _, fanout := range t.FanoutPoints {
		// Track paths from this fanout using BFS
		visited := make(map[*Line]int) // Line -> count of paths reaching it
		queue := make([]*Line, 0)

		// Start with all immediate outputs of the fanout
		for _, gate := range fanout.OutputGates {
			queue = append(queue, gate.Output)
			visited[gate.Output] = 1
		}

		// BFS to find reconvergent points
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			// Process each gate this line feeds into
			for _, gate := range current.OutputGates {
				output := gate.Output

				if count, exists := visited[output]; exists {
					// This line has been reached before, so it's a reconvergence point
					visited[output] = count + 1
					t.ReconvPoints[output] = true
				} else {
					// First time visiting this line
					visited[output] = 1
					queue = append(queue, output)
				}
			}
		}
	}
}

// GetHeadLines returns a list of head lines sorted by level
func (t *Topology) GetHeadLines() []*Line {
	return t.HeadLinesList
}

// FindUniquePathsToOutputs finds paths that all signals from a gate must traverse
// to reach primary outputs (used for unique sensitization)
func (t *Topology) FindUniquePathsToOutputs(gate *Gate) []*Line {
	uniquePaths := make([]*Line, 0)

	// Find all paths from gate output to primary outputs
	paths := t.findAllPathsToOutputs(gate.Output)

	// Find lines that appear in all paths
	if len(paths) > 0 {
		// Start with all lines from the first path
		commonLines := make(map[*Line]bool)
		for _, line := range paths[0] {
			commonLines[line] = true
		}

		// Keep only lines that appear in all paths
		for i := 1; i < len(paths); i++ {
			currentPathLines := make(map[*Line]bool)
			for _, line := range paths[i] {
				currentPathLines[line] = true
			}

			// Remove lines not in current path
			for line := range commonLines {
				if !currentPathLines[line] {
					delete(commonLines, line)
				}
			}
		}

		// Convert to slice
		for line := range commonLines {
			uniquePaths = append(uniquePaths, line)
		}

		// Sort by level
		sort.Slice(uniquePaths, func(i, j int) bool {
			return t.LevelMap[uniquePaths[i]] < t.LevelMap[uniquePaths[j]]
		})
	}

	return uniquePaths
}

// findAllPathsToOutputs finds all paths from a line to primary outputs
func (t *Topology) findAllPathsToOutputs(startLine *Line) [][]*Line {
	var paths [][]*Line

	// Recursive helper function
	var findPaths func(line *Line, currentPath []*Line)
	findPaths = func(line *Line, currentPath []*Line) {
		// Add current line to path
		currentPath = append(currentPath, line)

		// If this is a primary output, we found a path
		if line.Type == PrimaryOutput {
			// Make a copy of the path
			pathCopy := make([]*Line, len(currentPath))
			copy(pathCopy, currentPath)
			paths = append(paths, pathCopy)
			return
		}

		// Continue to each output gate
		for _, gate := range line.OutputGates {
			findPaths(gate.Output, currentPath)
		}
	}

	// Start the search
	findPaths(startLine, make([]*Line, 0))
	return paths
}

// GetControlLineFor returns the best head line to control a target line
func (t *Topology) GetControlLineFor(targetLine *Line) *Line {
	// Find path to targetLine containing only free lines
	var bestHeadLine *Line
	bestDistance := -1

	for _, headLine := range t.HeadLinesList {
		// Check if a path exists from this head line to target line
		path := t.findPathBetween(headLine, targetLine)
		if path != nil {
			distance := len(path)
			if bestDistance == -1 || distance < bestDistance {
				bestDistance = distance
				bestHeadLine = headLine
			}
		}
	}

	return bestHeadLine
}

// findPathBetween finds a path between two lines if one exists
func (t *Topology) findPathBetween(start, end *Line) []*Line {
	// Simple BFS to find path
	visited := make(map[*Line]bool)
	queue := [][]*Line{{start}}

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		current := path[len(path)-1]
		if current == end {
			return path
		}

		if visited[current] {
			continue
		}

		visited[current] = true

		// Add all gates this line feeds into
		for _, gate := range current.OutputGates {
			output := gate.Output
			if !visited[output] {
				newPath := make([]*Line, len(path))
				copy(newPath, path)
				newPath = append(newPath, output)
				queue = append(queue, newPath)
			}
		}
	}

	return nil
}
