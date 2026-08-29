// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// StaffScope restricts endpoints to only staff roles within a tenant.
// It checks that the user:
// 1. Has a valid tenant_id (not a super_admin operating without tenant context)
// 2. Has one of the allowed roles
//
// This is used for tenant-specific operations like managing clients,
// uploading documents, or viewing service details.
func StaffScope(allowedRoles ...string) gin.HandlerFunc {
	// Build allowed roles map for O(1) lookup
	allowed := make(map[string]bool)
	for _, role := range allowedRoles {
		allowed[role] = true
	}

	// Default allowed roles if none specified
	if len(allowed) == 0 {
		allowed["admin"] = true
		allowed["manager"] = true
		allowed["staff"] = true
	}

	return func(c *gin.Context) {
		// Get role from context (set by JWTAuth)
		role, exists := c.Get(AuthRole)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "access_denied",
				"message": "No role assigned",
			})
			c.Abort()
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "access_denied",
				"message": "Invalid role format",
			})
			c.Abort()
			return
		}

		// Super admins bypass staff scope checks (they have global access)
		if roleStr == "super_admin" {
			c.Next()
			return
		}

		// Verify tenant context exists (staff must operate within a tenant)
		tenantID, _ := GetTenantID(c)
		if tenantID.String() == "00000000-0000-0000-0000-000000000000" {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "tenant_required",
				"message": "Tenant context required for this operation",
			})
			c.Abort()
			return
		}

		// Check if role is allowed
		if !allowed[roleStr] {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "access_denied",
				"message": "Insufficient permissions for this operation",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AdminOnly is a convenience middleware that restricts to admin roles only.
func AdminOnly() gin.HandlerFunc {
	return StaffScope("admin", "super_admin")
}

// ManagerOrAbove restricts to manager, admin, or super_admin roles.
func ManagerOrAbove() gin.HandlerFunc {
	return StaffScope("manager", "admin", "super_admin")
}

// StaffOrAbove allows all staff roles (staff, manager, admin, super_admin).
func StaffOrAbove() gin.HandlerFunc {
	return StaffScope("staff", "manager", "admin", "super_admin")
}
