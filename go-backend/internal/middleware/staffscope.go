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
// Valid roles (from schema): super_admin, tenant_admin, staff, client
// - super_admin: Global admin, bypasses tenant checks (handled separately)
// - tenant_admin: Admin within a specific tenant
// - staff: Staff member within a tenant
// - client: Client portal access within a tenant
//
// This is used for tenant-specific operations like managing clients,
// uploading documents, or viewing service details.
func StaffScope(allowedRoles ...string) gin.HandlerFunc {
	// Build allowed roles map for O(1) lookup
	allowed := make(map[string]bool)
	for _, role := range allowedRoles {
		allowed[role] = true
	}

	// Default allowed roles if none specified (tenant_admin and staff)
	if len(allowed) == 0 {
		allowed["tenant_admin"] = true
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

// TenantAdminOnly restricts to tenant_admin role only.
// Note: super_admin bypasses this check automatically in StaffScope.
func TenantAdminOnly() gin.HandlerFunc {
	return StaffScope("tenant_admin")
}

// TenantAdminOrAbove restricts to tenant_admin or super_admin roles.
// Use this for admin-level operations within a tenant.
func TenantAdminOrAbove() gin.HandlerFunc {
	return StaffScope("tenant_admin")
}

// StaffOrAbove allows staff and tenant_admin roles.
// Note: super_admin bypasses this check automatically in StaffScope.
// Use this for general staff operations (most endpoints).
func StaffOrAbove() gin.HandlerFunc {
	return StaffScope("staff", "tenant_admin")
}

// ClientOrAbove allows client, staff, and tenant_admin roles.
// Use this for read operations that clients can also access.
func ClientOrAbove() gin.HandlerFunc {
	return StaffScope("client", "staff", "tenant_admin")
}

// Deprecated: AdminOnly is deprecated, use TenantAdminOnly instead.
// Kept for backward compatibility during migration.
func AdminOnly() gin.HandlerFunc {
	return TenantAdminOnly()
}

// Deprecated: ManagerOrAbove is deprecated, use TenantAdminOrAbove instead.
// The "manager" role does not exist in the schema.
func ManagerOrAbove() gin.HandlerFunc {
	return TenantAdminOrAbove()
}
