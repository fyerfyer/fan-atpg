package test

import "github.com/fyerfyer/fan-atpg/pkg/circuit"

// Helper function to find a line by name (already implemented elsewhere)
func findLine(c *circuit.Circuit, name string) *circuit.Line {
	for _, line := range c.Lines {
		if line.Name == name {
			return line
		}
	}
	return nil
}

// Helper function to find a gate by name
func findGate(c *circuit.Circuit, name string) *circuit.Gate {
	for _, gate := range c.Gates {
		if gate.Name == name {
			return gate
		}
	}
	return nil
}
