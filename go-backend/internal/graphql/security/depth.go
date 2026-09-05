// Package security provides GraphQL query security extensions
package security

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/rs/zerolog/log"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// MaxDepth is the maximum allowed query depth
const MaxDepth = 10

// DepthLimiter implements the graphql.OperationParameterMutator interface
// to reject queries that exceed the maximum depth
type DepthLimiter struct{}

var _ graphql.OperationParameterMutator = (*DepthLimiter)(nil)

// ExtensionName returns the extension name
func (d *DepthLimiter) ExtensionName() string {
	return "DepthLimiter"
}

// Validate is called to validate the schema
func (d *DepthLimiter) Validate(schema graphql.ExecutableSchema) error {
	return nil
}

// MutateOperationParameters checks query depth and rejects if too deep
func (d *DepthLimiter) MutateOperationParameters(ctx context.Context, req *graphql.RawParams) *gqlerror.Error {
	// Get the operation context to access the parsed document
	oc := graphql.GetOperationContext(ctx)
	if oc == nil || oc.Doc == nil {
		// Can't validate without parsed document, let it through
		return nil
	}

	// Find the operation being executed
	var operation *ast.OperationDefinition
	for _, op := range oc.Doc.Operations {
		if op.Name == req.OperationName || (req.OperationName == "" && len(oc.Doc.Operations) == 1) {
			operation = op
			break
		}
	}

	if operation == nil {
		return nil
	}

	// Calculate depth
	depth := calculateDepth(operation.SelectionSet, oc.Doc.Fragments, 0)

	if depth > MaxDepth {
		log.Warn().
			Int("depth", depth).
			Int("max_depth", MaxDepth).
			Str("operation", req.OperationName).
			Msg("GraphQL query rejected: depth limit exceeded")

		return &gqlerror.Error{
			Message: "query depth exceeds maximum allowed depth",
			Extensions: map[string]interface{}{
				"code":      "DEPTH_LIMIT_EXCEEDED",
				"depth":     depth,
				"max_depth": MaxDepth,
			},
		}
	}

	log.Debug().
		Int("depth", depth).
		Str("operation", req.OperationName).
		Msg("GraphQL query depth validated")

	return nil
}

// calculateDepth recursively calculates the depth of a selection set
func calculateDepth(selSet ast.SelectionSet, fragments ast.FragmentDefinitionList, currentDepth int) int {
	if len(selSet) == 0 {
		return currentDepth
	}

	maxChildDepth := currentDepth

	for _, sel := range selSet {
		var childDepth int

		switch s := sel.(type) {
		case *ast.Field:
			// Skip introspection fields
			if s.Name == "__typename" || s.Name == "__schema" || s.Name == "__type" {
				continue
			}

			if len(s.SelectionSet) > 0 {
				childDepth = calculateDepth(s.SelectionSet, fragments, currentDepth+1)
			} else {
				childDepth = currentDepth + 1
			}

		case *ast.InlineFragment:
			childDepth = calculateDepth(s.SelectionSet, fragments, currentDepth)

		case *ast.FragmentSpread:
			// Find the fragment definition
			frag := fragments.ForName(s.Name)
			if frag != nil {
				childDepth = calculateDepth(frag.SelectionSet, fragments, currentDepth)
			}
		}

		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}

	return maxChildDepth
}
