// Package security provides GraphQL query security extensions
package security

import (
	"github.com/99designs/gqlgen/graphql/handler/extension"
)

// MaxComplexity is the maximum allowed query complexity
const MaxComplexity = 1000

// ComplexityLimit returns a complexity extension configured with our limits
func ComplexityLimit() *extension.ComplexityLimit {
	return extension.FixedComplexityLimit(MaxComplexity)
}

// ComplexityFunc calculates complexity for fields based on arguments
// This is used by gqlgen's complexity extension
func ComplexityFunc(childComplexity int, args map[string]interface{}) int {
	// Base complexity is always at least 1
	complexity := 1

	// Add child complexity
	complexity += childComplexity

	// Multiply by limit if present (for pagination)
	if first, ok := args["first"].(int); ok && first > 0 {
		complexity *= first
	}
	if limit, ok := args["limit"].(int); ok && limit > 0 {
		complexity *= limit
	}

	return complexity
}
