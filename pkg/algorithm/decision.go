package algorithm

import (
	"fmt"
	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
)

// DecisionNode represents a node in the decision tree
type DecisionNode struct {
	Line        *circuit.Line      // The line where a decision was made
	Value       circuit.LogicValue // The current assigned value
	Tried       bool               // Whether the alternative value has been tried
	Alternative circuit.LogicValue // The alternative value (typically the opposite)
}

// Decision manages the decision tree for the FAN algorithm
type Decision struct {
	Circuit     *circuit.Circuit
	Logger      *utils.Logger
	Topology    *circuit.Topology
	Frontier    *Frontier
	Implication *Implication
	Backtrace   *Backtrace
	Stack       []*DecisionNode // The decision stack (tree nodes in order)
}

// NewDecision creates a new Decision manager
func NewDecision(c *circuit.Circuit, t *circuit.Topology, f *Frontier, i *Implication, b *Backtrace, logger *utils.Logger) *Decision {
	return &Decision{
		Circuit:     c,
		Logger:      logger,
		Topology:    t,
		Frontier:    f,
		Implication: i,
		Backtrace:   b,
		Stack:       make([]*DecisionNode, 0),
	}
}

// MakeDecision makes a new decision for the next step in test generation
func (d *Decision) MakeDecision() (bool, error) {
	d.Logger.Decision("Making a new decision")

	// Get the next objective (what line to set and what value to try)
	line, value, shouldContinue := d.Backtrace.GetNextObjective()
	if !shouldContinue {
		d.Logger.Decision("Backtrace indicates we need to backtrack")
		return d.Backtrack()
	}

	// If no line was selected, check if we've found a test or need to backtrack
	if line == nil {
		if d.Circuit.CheckTestStatus() {
			d.Logger.Decision("Test found! No further decisions needed")
			return true, nil
		}
		d.Logger.Decision("No viable line found for decision, need to backtrack")
		return d.Backtrack()
	}

	// Create a new decision node
	node := &DecisionNode{
		Line:        line,
		Value:       value,
		Tried:       false,
		Alternative: oppositeBinaryValue(value),
	}

	// Try the selected value
	success, err := d.tryValue(line, value)
	if err != nil {
		return false, err
	}

	if success {
		// Value worked, add the node to the stack
		d.Stack = append(d.Stack, node)
		d.Logger.Decision("Decision successful: %s = %v", line.Name, value)
		return true, nil
	}

	// First value didn't work, try the alternative immediately
	d.Logger.Decision("First value %v failed for %s, trying alternative %v",
		value, line.Name, node.Alternative)

	success, err = d.tryValue(line, node.Alternative)
	if err != nil {
		return false, err
	}

	if success {
		// Alternative worked, add the node to the stack with tried=true
		node.Value = node.Alternative
		node.Tried = true
		d.Stack = append(d.Stack, node)
		d.Logger.Decision("Alternative decision successful: %s = %v", line.Name, node.Alternative)
		return true, nil
	}

	// Both values failed, need to backtrack
	d.Logger.Decision("Both values failed for %s, need to backtrack", line.Name)
	return d.Backtrack()
}

// tryValue attempts to set a line to a specific value and check if it leads to a conflict
func (d *Decision) tryValue(line *circuit.Line, value circuit.LogicValue) (bool, error) {
	// Save circuit state before trying
	savedState := d.saveCircuitState()

	// Try setting the value
	line.SetValue(value)
	d.Logger.Trace("Setting %s = %v", line.Name, value)

	// Perform implication
	ok, err := d.Implication.ImplyValues()
	if err != nil || !ok {
		// Restore state and return false
		d.restoreCircuitState(savedState)
		d.Logger.Trace("Value %v on %s leads to conflict", value, line.Name)
		return false, nil
	}

	// Update frontiers after implication
	d.Frontier.UpdateDFrontier()
	d.Frontier.UpdateJFrontier()

	// Check if X-path exists (path from D/D' to output)
	if len(d.Frontier.DFrontier) > 0 && !d.Backtrace.CheckXPath() {
		// No path exists, restore state and return false
		d.restoreCircuitState(savedState)
		d.Logger.Decision("No path exists from fault to output, decision fails")
		return false, nil
	}

	return true, nil
}

// Backtrack performs backtracking in the decision tree
func (d *Decision) Backtrack() (bool, error) {
	d.Logger.Backtrack("Starting backtracking")

	// If stack is empty, no more backtracking possible
	if len(d.Stack) == 0 {
		d.Logger.Backtrack("Decision stack empty, no more backtracking possible")
		return false, fmt.Errorf("no test possible, decision stack empty")
	}

	// Pop the last decision
	lastIdx := len(d.Stack) - 1
	node := d.Stack[lastIdx]
	d.Stack = d.Stack[:lastIdx]

	// If we haven't tried the alternative, try it now
	if !node.Tried {
		d.Logger.Backtrack("Trying alternative value %v for %s",
			node.Alternative, node.Line.Name)

		// Reset circuit state
		d.Circuit.Reset()

		// If we have a fault site, re-inject fault
		if d.Circuit.FaultSite != nil {
			d.Circuit.InjectFault(d.Circuit.FaultSite, d.Circuit.FaultType)
		}

		// Reapply all decisions up to but not including the current one
		for i := 0; i < len(d.Stack); i++ {
			prevNode := d.Stack[i]
			prevNode.Line.SetValue(prevNode.Value)
		}

		// Perform implication to restore circuit state
		_, err := d.Implication.ImplyValues()
		if err != nil {
			return false, err
		}

		// Now try the alternative value
		success, err := d.tryValue(node.Line, node.Alternative)
		if err != nil {
			return false, err
		}

		if success {
			// Alternative worked, push back onto stack with tried=true
			node.Value = node.Alternative
			node.Tried = true
			d.Stack = append(d.Stack, node)
			d.Logger.Backtrack("Alternative value %v successful for %s",
				node.Alternative, node.Line.Name)
			return true, nil
		}

		d.Logger.Backtrack("Alternative value %v also failed for %s, continuing backtrack",
			node.Alternative, node.Line.Name)
	}

	// If we get here, both values failed or we've already tried the alternative
	// Continue backtracking to previous decision
	return d.Backtrack()
}

// GetTestPattern returns the current test pattern (input assignments)
func (d *Decision) GetTestPattern() map[string]circuit.LogicValue {
	return d.Circuit.GetCurrentTest()
}

// saveCircuitState saves the current state of all lines in the circuit
func (d *Decision) saveCircuitState() map[int]circuit.LogicValue {
	state := make(map[int]circuit.LogicValue)
	for id, line := range d.Circuit.Lines {
		state[id] = line.Value
	}
	return state
}

// restoreCircuitState restores all line values from a saved state
func (d *Decision) restoreCircuitState(state map[int]circuit.LogicValue) {
	for id, value := range state {
		if line, exists := d.Circuit.Lines[id]; exists {
			line.Value = value
		}
	}

	// Update frontiers after restore
	d.Frontier.UpdateDFrontier()
	d.Frontier.UpdateJFrontier()
}

// GetCurrentDecisionDepth returns the current depth in the decision tree
func (d *Decision) GetCurrentDecisionDepth() int {
	return len(d.Stack)
}

// Reset resets the decision tree
func (d *Decision) Reset() {
	d.Stack = make([]*DecisionNode, 0)
}

// IsSatisfiable determines if the current decision state can lead to a solution
func (d *Decision) IsSatisfiable() bool {
	// Check if there's a fault site, it must be activated
	if d.Circuit.FaultSite != nil {
		if d.Circuit.FaultSite.Value != d.Circuit.FaultType &&
			!d.Circuit.FaultSite.IsFaulty() {
			return false
		}
	}

	// If there are faulty signals but no D-frontier and no fault at output,
	// then the fault effect is blocked
	faultySignalExists := false
	faultyOutputExists := false

	for _, line := range d.Circuit.Lines {
		if line.IsFaulty() {
			faultySignalExists = true
			if line.Type == circuit.PrimaryOutput {
				faultyOutputExists = true
			}
		}
	}

	if faultySignalExists && !faultyOutputExists && len(d.Frontier.DFrontier) == 0 {
		return false
	}

	return true
}

// Helper function to get the opposite binary value
func oppositeBinaryValue(value circuit.LogicValue) circuit.LogicValue {
	if value == circuit.Zero {
		return circuit.One
	}
	return circuit.Zero
}
