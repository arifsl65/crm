// Package security provides GraphQL query security extensions
package security

import (
	"context"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/rs/zerolog/log"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// QueryTimeout is the maximum time allowed for a query to execute
const QueryTimeout = 5 * time.Second

// TimeoutExtension adds query timeout enforcement
type TimeoutExtension struct{}

var _ graphql.OperationInterceptor = (*TimeoutExtension)(nil)

// ExtensionName returns the extension name
func (t *TimeoutExtension) ExtensionName() string {
	return "QueryTimeout"
}

// Validate validates the schema
func (t *TimeoutExtension) Validate(schema graphql.ExecutableSchema) error {
	return nil
}

// InterceptOperation wraps the operation with a timeout
func (t *TimeoutExtension) InterceptOperation(ctx context.Context, next graphql.OperationHandler) graphql.ResponseHandler {
	oc := graphql.GetOperationContext(ctx)

	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, QueryTimeout)

	// Track if timeout occurred
	done := make(chan struct{})

	go func() {
		select {
		case <-timeoutCtx.Done():
			if timeoutCtx.Err() == context.DeadlineExceeded {
				log.Warn().
					Str("operation", oc.OperationName).
					Dur("timeout", QueryTimeout).
					Msg("GraphQL query timed out")
			}
		case <-done:
			// Query completed normally
		}
	}()

	// Execute with timeout context
	responseHandler := next(timeoutCtx)

	return func(ctx context.Context) *graphql.Response {
		resp := responseHandler(timeoutCtx)

		// Signal completion
		close(done)
		cancel()

		// Add timeout error if applicable
		if timeoutCtx.Err() == context.DeadlineExceeded {
			if resp == nil {
				resp = &graphql.Response{}
			}
			resp.Errors = append(resp.Errors, &gqlerror.Error{
				Message: "query execution timed out",
				Extensions: map[string]interface{}{
					"code":    "QUERY_TIMEOUT",
					"timeout": QueryTimeout.String(),
				},
			})
		}

		return resp
	}
}
