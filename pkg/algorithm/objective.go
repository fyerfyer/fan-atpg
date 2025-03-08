package algorithm

import (
	"fmt"
	"github.com/fyerfyer/fan-atpg/pkg/circuit"
	"github.com/fyerfyer/fan-atpg/pkg/utils"
	"sort"
)

// InitialObjective represents an objective to set a specific line to a specific value
type InitialObjective struct {
	Line  *circuit.Line      // The line to be set
	Value circuit.LogicValue // The target value
}

// Objective represents a triplet (s, n₀, n₁) for multiple backtrace
// where s is an objective line, n₀ is the number of times the objective value 0 is required,
// and n₁ is the number of times the objective value 1 is required
type Objective struct {
	Line *circuit.Line // Objective line
	N0   int           // Number of times value 0 is required
	N1   int           // Number of times value 1 is required
}

// String returns a string representation of an objective
func (o *Objective) String() string {
	return fmt.Sprintf("(%s, n₀=%d, n₁=%d)", o.Line.Name, o.N0, o.N1)
}

// GetPreferredValue returns the preferred value (0 or 1) for this objective
func (o *Objective) GetPreferredValue() circuit.LogicValue {
	if o.N0 > o.N1 {
		return circuit.Zero
	} else if o.N1 > o.N0 {
		return circuit.One
	}
	// If equal, prefer 1 (arbitrary choice)
	return circuit.One
}

// MultipleBacktrace manages the multiple backtrace process
type MultipleBacktrace struct {
	Circuit     *circuit.Circuit
	Logger      *utils.Logger
	Topology    *circuit.Topology
	InitialObjs []InitialObjective
	CurrentObjs []*Objective
	FinalObjs   []*Objective
}

// NewMultipleBacktrace creates a new multiple backtrace manager
func NewMultipleBacktrace(c *circuit.Circuit, topo *circuit.Topology, logger *utils.Logger) *MultipleBacktrace {
	return &MultipleBacktrace{
		Circuit:     c,
		Logger:      logger,
		Topology:    topo,
		InitialObjs: make([]InitialObjective, 0),
		CurrentObjs: make([]*Objective, 0),
		FinalObjs:   make([]*Objective, 0),
	}
}

// SetInitialObjectives sets the initial objectives for multiple backtrace
func (mb *MultipleBacktrace) SetInitialObjectives(objs []InitialObjective) {
	mb.InitialObjs = objs
	mb.CurrentObjs = make([]*Objective, 0)
	mb.FinalObjs = make([]*Objective, 0)

	// Convert initial objectives to current objectives
	for _, obj := range objs {
		if obj.Value == circuit.Zero {
			mb.CurrentObjs = append(mb.CurrentObjs, &Objective{
				Line: obj.Line,
				N0:   1,
				N1:   0,
			})
		} else if obj.Value == circuit.One {
			mb.CurrentObjs = append(mb.CurrentObjs, &Objective{
				Line: obj.Line,
				N0:   0,
				N1:   1,
			})
		}
	}
}

// PerformBacktrace performs multiple backtrace from the current objectives
// to the head lines of the circuit
func (mb *MultipleBacktrace) PerformBacktrace() {
	mb.Logger.Algorithm("Starting multiple backtrace with %d objectives", len(mb.CurrentObjs))
	mb.Logger.Indent()
	defer mb.Logger.Outdent()

	// Initialize set of processed lines to avoid cycles
	processed := make(map[*circuit.Line]bool)

	// Process objectives until we reach head lines or primary inputs
	queue := make([]*Objective, len(mb.CurrentObjs))
	copy(queue, mb.CurrentObjs)

	for len(queue) > 0 {
		obj := queue[0]
		queue = queue[1:]

		mb.Logger.Trace("Processing objective %s", obj.String())

		line := obj.Line

		// Skip lines we've already processed
		if processed[line] {
			continue
		}

		// If this is a head line or primary input, add to final objectives
		if line.IsHeadLine || line.Type == circuit.PrimaryInput {
			mb.FinalObjs = append(mb.FinalObjs, obj)
			mb.Logger.Trace("Reached head line or PI: %s", line.Name)
			continue
		}

		// Get the gate that drives this line
		inputGate := line.InputGate
		if inputGate == nil {
			mb.Logger.Warning("Line %s has no driving gate", line.Name)
			continue
		}

		// Create new objectives based on gate type and propagate backward
		newObjs := mb.backtraceGate(inputGate, obj)
		queue = append(queue, newObjs...)

		// Mark this line as processed
		processed[line] = true
	}

	// Sort final objectives by their preference power (n₁ - n₀)
	sort.Slice(mb.FinalObjs, func(i, j int) bool {
		// Higher absolute difference indicates stronger preference
		diffI := mb.FinalObjs[i].N1 - mb.FinalObjs[i].N0
		diffJ := mb.FinalObjs[j].N1 - mb.FinalObjs[j].N0
		if abs(diffI) != abs(diffJ) {
			return abs(diffI) > abs(diffJ)
		}

		// If tied, prefer head lines over primary inputs
		if mb.FinalObjs[i].Line.IsHeadLine != mb.FinalObjs[j].Line.IsHeadLine {
			return mb.FinalObjs[i].Line.IsHeadLine
		}

		// If still tied, use line ID for stability
		return mb.FinalObjs[i].Line.ID < mb.FinalObjs[j].Line.ID
	})

	mb.Logger.Algorithm("Multiple backtrace completed with %d final objectives", len(mb.FinalObjs))
	for _, obj := range mb.FinalObjs {
		mb.Logger.Trace("Final objective: %s, preferred value: %v", obj.String(), obj.GetPreferredValue())
	}
}

// backtraceGate creates new objectives based on the gate type and current objective
func (mb *MultipleBacktrace) backtraceGate(gate *circuit.Gate, obj *Objective) []*Objective {
	newObjs := make([]*Objective, 0)

	switch gate.Type {
	case circuit.AND, circuit.NAND:
		// For the output of an AND gate:
		// - To justify output=0, we need at least one input=0
		// - To justify output=1, we need all inputs=1

		// If it's NAND, invert the n₀ and n₁ values
		n0, n1 := obj.N0, obj.N1
		if gate.Type == circuit.NAND {
			n0, n1 = obj.N1, obj.N0
		}

		if n0 > 0 {
			// Find the easiest input to control to 0
			easiestInput := mb.findEasiestControlInput(gate, circuit.Zero)
			newObj := &Objective{
				Line: easiestInput,
				N0:   n0,
				N1:   0,
			}
			newObjs = append(newObjs, newObj)
		}

		if n1 > 0 {
			// Need all inputs to be 1
			for _, input := range gate.Inputs {
				newObj := &Objective{
					Line: input,
					N0:   0,
					N1:   n1,
				}
				newObjs = append(newObjs, newObj)
			}
		}

	case circuit.OR, circuit.NOR:
		// For the output of an OR gate:
		// - To justify output=1, we need at least one input=1
		// - To justify output=0, we need all inputs=0

		// If it's NOR, invert the n₀ and n₁ values
		n0, n1 := obj.N0, obj.N1
		if gate.Type == circuit.NOR {
			n0, n1 = obj.N1, obj.N0
		}

		if n1 > 0 {
			// Find the easiest input to control to 1
			easiestInput := mb.findEasiestControlInput(gate, circuit.One)
			newObj := &Objective{
				Line: easiestInput,
				N0:   0,
				N1:   n1,
			}
			newObjs = append(newObjs, newObj)
		}

		if n0 > 0 {
			// Need all inputs to be 0
			for _, input := range gate.Inputs {
				newObj := &Objective{
					Line: input,
					N0:   n0,
					N1:   0,
				}
				newObjs = append(newObjs, newObj)
			}
		}

	case circuit.NOT:
		// For a NOT gate, invert n₀ and n₁
		if len(gate.Inputs) == 1 {
			newObj := &Objective{
				Line: gate.Inputs[0],
				N0:   obj.N1,
				N1:   obj.N0,
			}
			newObjs = append(newObjs, newObj)
		}

	case circuit.BUF:
		// For a buffer, just pass the objective through
		if len(gate.Inputs) == 1 {
			newObj := &Objective{
				Line: gate.Inputs[0],
				N0:   obj.N0,
				N1:   obj.N1,
			}
			newObjs = append(newObjs, newObj)
		}

	case circuit.XOR, circuit.XNOR:
		// XOR and XNOR require more complex handling
		// Simplified approach for now
		for _, input := range gate.Inputs {
			// For XOR output=1: could be (0,1) or (1,0)
			// For XOR output=0: could be (0,0) or (1,1)
			// We'll just assign equal weights to both possibilities
			newObj := &Objective{
				Line: input,
				N0:   obj.N0 + obj.N1, // Equal weight to 0
				N1:   obj.N0 + obj.N1, // Equal weight to 1
			}
			newObjs = append(newObjs, newObj)
		}
	}

	return newObjs
}

// findEasiestControlInput finds the input that is easiest to control to the target value
func (mb *MultipleBacktrace) findEasiestControlInput(gate *circuit.Gate, targetValue circuit.LogicValue) *circuit.Line {
	// This is a simplified implementation
	// In practice, you might use testability measures or controllability metrics

	// If gate has a cached control input ID, use that
	if gate.ControlID >= 0 && gate.ControlID < len(gate.Inputs) {
		return gate.Inputs[gate.ControlID]
	}

	// Otherwise, default to the first input
	if len(gate.Inputs) > 0 {
		return gate.Inputs[0]
	}

	// Should never happen
	mb.Logger.Error("Gate %s has no inputs", gate.Name)
	return nil
}

// GetBestFinalObjective returns the best final objective to try next
func (mb *MultipleBacktrace) GetBestFinalObjective() (*circuit.Line, circuit.LogicValue) {
	if len(mb.FinalObjs) == 0 {
		return nil, circuit.X
	}

	bestObj := mb.FinalObjs[0]
	value := bestObj.GetPreferredValue()

	mb.Logger.Decision("Selected best objective: line %s = %v (n₀=%d, n₁=%d)",
		bestObj.Line.Name, value, bestObj.N0, bestObj.N1)

	return bestObj.Line, value
}

// IsObjectiveEffective checks if the current objectives are still effective
// This depends on the state of the D-frontier after implication
func (mb *MultipleBacktrace) IsObjectiveEffective(oldDFrontier []*circuit.Gate, newDFrontier []*circuit.Gate) bool {
	// If we were propagating D/D' and D-frontier has changed, objectives are no longer effective
	if len(oldDFrontier) != len(newDFrontier) {
		return false
	}

	for i := range oldDFrontier {
		if oldDFrontier[i].ID != newDFrontier[i].ID {
			return false
		}
	}

	return true
}

// Helper function to get absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
