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
	// Skip if already bound
	for _, gate := range startLine.OutputGates {
		if !gate.Output.IsBound {
			gate.Output.IsBound = true
			gate.Output.IsFree = false

			// mark recursively
			t.markReachableLines(gate.Output)
		}
	}
}

// IdentifyHeadLines identifies all head lines in the circuit
// (free lines that are adjacent to bound lines)
func (t *Topology) IdentifyHeadLines() {
	t.HeadLinesList = make([]*Line, 0)

	// 创建fanout points的快速查找表
	fanoutMap := make(map[*Line]bool)
	for _, fp := range t.FanoutPoints {
		fanoutMap[fp] = true
	}

	for _, line := range t.Circuit.Lines {
		// 跳过不是free的线路和fanout points
		if !line.IsFree || fanoutMap[line] {
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
	t.ReconvPoints = make(map[*Line]bool)

	// 对每个fanout point，跟踪前向路径以找到重合点
	for _, fanout := range t.FanoutPoints {
		// 使用BFS跟踪来自此fanout的路径
		visited := make(map[*Line]int) // Line -> 到达该线路的路径数
		queue := make([]*Line, 0)

		// 从fanout的所有直接输出开始
		for _, gate := range fanout.OutputGates {
			queue = append(queue, gate.Output)
		}

		// 为直接输出初始化访问计数
		for _, line := range queue {
			visited[line] = 1
		}

		// BFS查找重合点
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			// 处理此线路输入到的每个gate
			for _, gate := range current.OutputGates {
				output := gate.Output

				// 检查是否是PrimaryOutput，如果是且路径数>1则标记为重合点
				if output.Type == PrimaryOutput && visited[current] > 1 {
					t.ReconvPoints[output] = true
				}

				if count, exists := visited[output]; exists {
					// 这条线路之前已被访问，所以它是重合点
					visited[output] = count + 1
					t.ReconvPoints[output] = true
				} else {
					// 首次访问这条线路
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

// FindUniquePathsToOutputs finds paths from a gate to primary outputs that must be sensitized
func (t *Topology) FindUniquePathsToOutputs(gate *Gate) [][]*Line {
	paths := [][]*Line{}

	// Start with the output of the gate
	outputLine := gate.Output

	// Add it to the initial path
	initialPath := []*Line{outputLine}

	// Find all paths from this line to primary outputs
	t.findPathsToPO(outputLine, initialPath, &paths)

	return paths
}

// Helper function to recursively find paths to primary outputs
func (t *Topology) findPathsToPO(line *Line, currentPath []*Line, allPaths *[][]*Line) {
	// If we've reached a primary output, add the path
	if line.Type == PrimaryOutput {
		pathCopy := make([]*Line, len(currentPath))
		copy(pathCopy, currentPath)
		*allPaths = append(*allPaths, pathCopy)
		return
	}

	// For each gate this line feeds into, explore a new path
	for _, gate := range line.OutputGates {
		// Skip already visited lines to prevent cycles
		alreadyInPath := false
		for _, pathLine := range currentPath {
			if gate.Output == pathLine {
				alreadyInPath = true
				break
			}
		}

		if !alreadyInPath {
			// Create a new path with the output added
			newPath := make([]*Line, len(currentPath))
			copy(newPath, currentPath)
			newPath = append(newPath, gate.Output)

			// Explore this path
			t.findPathsToPO(gate.Output, newPath, allPaths)
		}
	}
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
		path := t.FindPathBetween(headLine, targetLine)
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

// FindPathBetween finds a path between two lines if one exists
func (t *Topology) FindPathBetween(start, end *Line) []*Line {
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
