package algorithm

import (
	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
)

// Backtrace manages the backtracing process for the FAN algorithm
type Backtrace struct {
	Circuit     *circuit.Circuit
	Logger      *utils.Logger
	Topology    *circuit.Topology
	Frontier    *Frontier
	Implication *Implication
	MBT         *MultipleBacktrace
}

// NewBacktrace creates a new Backtrace manager
func NewBacktrace(
	c *circuit.Circuit,
	t *circuit.Topology,
	f *Frontier,
	i *Implication,
	logger *utils.Logger,
) *Backtrace {
	mbt := NewMultipleBacktrace(c, t, logger)

	return &Backtrace{
		Circuit:     c,
		Logger:      logger,
		Topology:    t,
		Frontier:    f,
		Implication: i,
		MBT:         mbt,
	}
}

// BacktraceFromDFrontier performs backtrace from D-frontier to determine objectives
// for fault propagation and returns (line, value) to try next
func (b *Backtrace) BacktraceFromDFrontier() (*circuit.Line, circuit.LogicValue) {
	b.Logger.Algorithm("Starting backtrace from D-frontier")

	// Get initial objectives for propagating fault through D-frontier
	initialObjs := b.Frontier.GetObjectivesFromDFrontier()
	if len(initialObjs) == 0 {
		b.Logger.Algorithm("No objectives found for D-frontier propagation")
		return nil, circuit.X
	}

	// Save current D-frontier for checking effectiveness later
	oldDFrontier := make([]*circuit.Gate, len(b.Frontier.DFrontier))
	copy(oldDFrontier, b.Frontier.DFrontier)

	// Perform multiple backtrace
	b.MBT.SetInitialObjectives(initialObjs)
	b.MBT.PerformBacktrace()

	// Get best final objective
	line, value := b.MBT.GetBestFinalObjective()
	if line == nil {
		b.Logger.Algorithm("Backtrace could not find suitable assignment")
		return nil, circuit.X
	}

	b.Logger.Algorithm("Backtrace from D-frontier selected line %s with value %v",
		line.Name, value)

	return line, value
}

// BacktraceFromJFrontier performs backtrace from J-frontier to determine objectives
// for line justification and returns (line, value) to try next
func (b *Backtrace) BacktraceFromJFrontier() (*circuit.Line, circuit.LogicValue) {
	b.Logger.Algorithm("Starting backtrace from J-frontier")

	// Get initial objectives for justifying lines in J-frontier
	initialObjs := b.Frontier.GetObjectivesFromJFrontier()
	if len(initialObjs) == 0 {
		b.Logger.Algorithm("No objectives found for J-frontier justification")
		return nil, circuit.X
	}

	// Perform multiple backtrace
	b.MBT.SetInitialObjectives(initialObjs)
	b.MBT.PerformBacktrace()

	// Get best final objective
	line, value := b.MBT.GetBestFinalObjective()
	if line == nil {
		b.Logger.Algorithm("Backtrace could not find suitable assignment")
		return nil, circuit.X
	}

	b.Logger.Algorithm("Backtrace from J-frontier selected line %s with value %v",
		line.Name, value)

	return line, value
}

// DirectBacktrace performs backtrace directly from a specific line and value
// Used for special cases like initial fault activation
func (b *Backtrace) DirectBacktrace(targetLine *circuit.Line, targetValue circuit.LogicValue) (*circuit.Line, circuit.LogicValue) {
	b.Logger.Algorithm("Starting direct backtrace for line %s = %v",
		targetLine.Name, targetValue)

	// Create a single objective
	initialObjs := []InitialObjective{
		{
			Line:  targetLine,
			Value: targetValue,
		},
	}

	// Perform multiple backtrace
	b.MBT.SetInitialObjectives(initialObjs)
	b.MBT.PerformBacktrace()

	// Get best final objective
	line, value := b.MBT.GetBestFinalObjective()
	if line == nil {
		b.Logger.Algorithm("Direct backtrace could not find suitable assignment")
		return nil, circuit.X
	}

	b.Logger.Algorithm("Direct backtrace selected line %s with value %v",
		line.Name, value)

	return line, value
}

// GetNextObjective determines what to target next in the FAN algorithm
// Returns the line to assign, value to try, and boolean indicating if the algorithm should continue
func (b *Backtrace) GetNextObjective() (*circuit.Line, circuit.LogicValue, bool) {
	// First, check if we need to activate the fault (fault excitation)
	if b.Circuit.FaultSite != nil && !b.Circuit.FaultSite.IsAssigned() {
		// For stuck-at-0 fault, we need to set the line to 1 (and vice versa)
		targetValue := circuit.One
		if b.Circuit.FaultType == circuit.One {
			targetValue = circuit.Zero
		}

		b.Logger.Algorithm("Need to activate fault at %s (stuck-at-%v) with value %v",
			b.Circuit.FaultSite.Name, b.Circuit.FaultType, targetValue)

		// If fault site is a head line or PI, assign it directly
		if b.Circuit.FaultSite.IsHeadLine || b.Circuit.FaultSite.Type == circuit.PrimaryInput {
			return b.Circuit.FaultSite, targetValue, true
		}

		// Otherwise, backtrace to control it
		//return b.DirectBacktrace(b.Circuit.FaultSite, targetValue)
		//return
		line, value := b.DirectBacktrace(b.Circuit.FaultSite, targetValue)
		return line, value, false
	}

	// Check if there's a D-frontier to propagate
	if len(b.Frontier.DFrontier) > 0 {
		b.Logger.Algorithm("D-frontier exists with %d gates, trying to propagate fault",
			len(b.Frontier.DFrontier))
		line, value := b.BacktraceFromDFrontier()
		if line != nil {
			return line, value, true
		}
	}

	// Check if there's a J-frontier to justify
	if len(b.Frontier.JFrontier) > 0 {
		b.Logger.Algorithm("J-frontier exists with %d gates, trying to justify values",
			len(b.Frontier.JFrontier))
		line, value := b.BacktraceFromJFrontier()
		if line != nil {
			return line, value, true
		}
	}

	// Check if test is already complete
	if b.Circuit.CheckTestStatus() {
		b.Logger.Algorithm("Test is complete, no further objectives needed")
		return nil, circuit.X, true
	}

	// If we get here, we can't make progress
	b.Logger.Algorithm("No viable objectives found, backtracking needed")
	return nil, circuit.X, false
}

// CheckXPath checks if there's a potential path to propagate D/D' to outputs
func (b *Backtrace) CheckXPath() bool {
	return b.Implication.CheckIfXPathExists()
}

// FindObjectiveForLineJustification finds an objective to justify a line
// when doing direct line justification
func (b *Backtrace) FindObjectiveForLineJustification(line *circuit.Line, value circuit.LogicValue) (*circuit.Line, circuit.LogicValue) {
	if line.InputGate == nil || line.IsHeadLine || line.Type == circuit.PrimaryInput {
		// This is a head line or PI, just assign it
		return line, value
	}

	// Create an objective and backtrace
	initialObjs := []InitialObjective{
		{
			Line:  line,
			Value: value,
		},
	}

	// Perform multiple backtrace
	b.MBT.SetInitialObjectives(initialObjs)
	b.MBT.PerformBacktrace()

	return b.MBT.GetBestFinalObjective()
}

// IsBacktraceNeeded determines if backtrace is needed for a next objective
func (b *Backtrace) IsBacktraceNeeded() bool {
	// If there's a fault site that's not set, we need to activate the fault
	if b.Circuit.FaultSite != nil && !b.Circuit.FaultSite.IsAssigned() {
		return true
	}

	// If there's a D-frontier, we need to propagate
	if len(b.Frontier.DFrontier) > 0 {
		return true
	}

	// If there's a J-frontier, we need to justify
	if len(b.Frontier.JFrontier) > 0 {
		return true
	}

	return false
}

// AreObjectivesEffective checks if current objectives are still effective
// after an implication operation
func (b *Backtrace) AreObjectivesEffective(oldDFrontier []*circuit.Gate) bool {
	// If D-frontier has changed, objectives are no longer effective
	return b.MBT.IsObjectiveEffective(oldDFrontier, b.Frontier.DFrontier)
}
