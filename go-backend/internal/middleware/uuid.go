// Package middleware provides HTTP middleware for the Accountant CRM API.
package middleware

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

// UUID regex pattern (RFC 4122)
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ValidateUUID validates that the specified path parameter is a valid UUID.
// It returns a 400 Bad Request if the parameter is not a valid UUID format.
//
// Usage:
//
//	router.GET("/users/:id", middleware.ValidateUUID("id"), userHandler)
func ValidateUUID(paramName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value := c.Param(paramName)

		if value == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_parameter",
				"message": "Missing required parameter: " + paramName,
			})
			return
		}

		if !uuidRegex.MatchString(value) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_uuid",
				"message": "Invalid UUID format for parameter: " + paramName,
			})
			return
		}

		c.Next()
	}
}

// ValidateUUIDs validates multiple path parameters as UUIDs.
// Useful when a route has multiple UUID parameters.
//
// Usage:
//
//	router.GET("/tenants/:tenant_id/users/:user_id", middleware.ValidateUUIDs("tenant_id", "user_id"), handler)
func ValidateUUIDs(paramNames ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, paramName := range paramNames {
			value := c.Param(paramName)

			if value == "" {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error":   "invalid_parameter",
					"message": "Missing required parameter: " + paramName,
				})
				return
			}

			if !uuidRegex.MatchString(value) {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error":   "invalid_uuid",
					"message": "Invalid UUID format for parameter: " + paramName,
				})
				return
			}
		}

		c.Next()
	}
}

// IsValidUUID checks if a string is a valid UUID format.
// This is a utility function for use in handlers.
func IsValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}
