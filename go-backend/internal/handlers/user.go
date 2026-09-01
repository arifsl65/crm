package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/accountant-crm/go-backend/internal/auth"
	"github.com/accountant-crm/go-backend/internal/database"
	"github.com/accountant-crm/go-backend/internal/middleware"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	db *database.Pool
}

// NewUserHandler creates a new UserHandler instance
func NewUserHandler(db *database.Pool) *UserHandler {
	return &UserHandler{db: db}
}

// User represents a user record
type User struct {
	ID        string     `json:"id"`
	TenantID  *string    `json:"tenant_id,omitempty"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	Status    string     `json:"status"`
	AvatarURL *string    `json:"avatar_url,omitempty"`
	Phone     *string    `json:"phone,omitempty"`
	Specialism *string   `json:"specialism,omitempty"`
	Notes     *string    `json:"notes,omitempty"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	TenantID   string  `json:"tenant_id" binding:"required,uuid"`
	Name       string  `json:"name" binding:"required,min=1,max=255"`
	Email      string  `json:"email" binding:"required,email"`
	Password   *string `json:"password,omitempty"`
	Role       string  `json:"role" binding:"required,oneof=tenant_admin staff client"`
	Phone      *string `json:"phone,omitempty"`
	Specialism *string `json:"specialism,omitempty"`
	Notes      *string `json:"notes,omitempty"`
	SendEmail  *bool   `json:"send_email,omitempty"`
}

// UpdateUserRequest represents the request body for updating a user
type UpdateUserRequest struct {
	Name       *string `json:"name,omitempty"`
	Email      *string `json:"email,omitempty"`
	Password   *string `json:"password,omitempty"`
	Role       *string `json:"role,omitempty"`
	Status     *string `json:"status,omitempty"`
	AvatarURL  *string `json:"avatar_url,omitempty"`
	Phone      *string `json:"phone,omitempty"`
	Specialism *string `json:"specialism,omitempty"`
	Notes      *string `json:"notes,omitempty"`
}

// List returns users based on role permissions
func (h *UserHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)

	users := []User{}

	if role == "super_admin" {
		// Super admins can see all users; use SuperAdminTransaction so RLS
		// policies see app.role='super_admin' and allow cross-tenant reads.
		err := h.db.SuperAdminTransaction(ctx, func(tx pgx.Tx) error {
			rows, err := tx.Query(ctx, `
				SELECT id, tenant_id, name, email, role, status, avatar_url, phone,
				       specialism, notes, last_login_at, created_at, updated_at
				FROM users
				WHERE deleted_at IS NULL
				ORDER BY created_at DESC
			`)
			if err != nil {
				return err
			}
			defer rows.Close()

			for rows.Next() {
				var u User
				if err := rows.Scan(
					&u.ID, &u.TenantID, &u.Name, &u.Email, &u.Role, &u.Status,
					&u.AvatarURL, &u.Phone, &u.Specialism, &u.Notes,
					&u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
				); err != nil {
					return err
				}
				users = append(users, u)
			}
			return rows.Err()
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to fetch users",
			})
			return
		}
	} else {
		// Others only see users in their tenant - use TenantDB for RLS
		tenantDB, ok := middleware.GetTenantDB(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "Database context not available",
			})
			return
		}

		err := tenantDB.Query(c, `
			SELECT id, tenant_id, name, email, role, status, avatar_url, phone,
			       specialism, notes, last_login_at, created_at, updated_at
			FROM users
			WHERE tenant_id = $1 AND deleted_at IS NULL
			ORDER BY created_at DESC
		`, []interface{}{userTenantID}, func(rows pgx.Rows) error {
			for rows.Next() {
				var u User
				if err := rows.Scan(
					&u.ID, &u.TenantID, &u.Name, &u.Email, &u.Role, &u.Status,
					&u.AvatarURL, &u.Phone, &u.Specialism, &u.Notes,
					&u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
				); err != nil {
					return err
				}
				users = append(users, u)
			}
			return nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "database_error",
				"message": "Failed to fetch users",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"count": len(users),
	})
}

// Create creates a new user (super_admin or tenant_admin)
func (h *UserHandler) Create(c *gin.Context) {
	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	targetTenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_tenant_id",
			"message": "Invalid tenant ID format",
		})
		return
	}

	// Authorization: tenant_admins can only create users in their tenant
	if role == "tenant_admin" && userTenantID != targetTenantID {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "You can only create users in your own tenant",
		})
		return
	}

	// Tenant admins cannot create other tenant_admins (only super_admin can)
	if role == "tenant_admin" && req.Role == "tenant_admin" {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "forbidden",
			"message": "Only super_admin can create tenant_admin users",
		})
		return
	}

	// Get TenantDB for RLS-protected queries
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Database context not available",
		})
		return
	}

	// Validate tenant exists (uses TenantDB so RLS context is set)
	var tenantExists bool
	err = tenantDB.QueryRowScan(c, []interface{}{&tenantExists}, `
		SELECT EXISTS(SELECT 1 FROM tenants WHERE id = $1 AND deleted_at IS NULL)
	`, targetTenantID)
	if err != nil || !tenantExists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_tenant",
			"message": "Tenant does not exist",
		})
		return
	}

	// Check if email already exists in that tenant
	var emailExists bool
	err = tenantDB.QueryRowScan(c, []interface{}{&emailExists}, `
		SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND tenant_id = $2 AND deleted_at IS NULL)
	`, req.Email, targetTenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to check email uniqueness",
		})
		return
	}
	if emailExists {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "email_exists",
			"message": "A user with this email already exists in this tenant",
		})
		return
	}

	// Determine creation mode: direct (password provided) or invitation (no password)
	var hashedPassword *string
	var inviteToken *string
	var inviteExpires *time.Time
	status := "active"

	if req.Password != nil && *req.Password != "" {
		hp, err := auth.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "hash_error",
				"message": "Failed to hash password",
			})
			return
		}
		hashedPassword = &hp
	} else {
		// Invitation mode
		status = "pending"
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "internal_error",
				"message": "Failed to generate invite token",
			})
			return
		}
		t := hex.EncodeToString(tokenBytes)
		inviteToken = &t
		exp := time.Now().Add(7 * 24 * time.Hour)
		inviteExpires = &exp
	}

	// Create user
	id := uuid.New()
	_, err = tenantDB.Exec(c, `
		INSERT INTO users (id, tenant_id, name, email, password, role, status,
		                   phone, specialism, notes, invite_token, invite_expires,
		                   created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW())
	`, id, targetTenantID, req.Name, req.Email, hashedPassword, req.Role, status,
		req.Phone, req.Specialism, req.Notes, inviteToken, inviteExpires)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "create_error",
			"message": "Failed to create user",
		})
		return
	}

	// Fetch the created user
	user, err := h.getUserByID(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "fetch_error",
			"message": "User created but failed to fetch",
		})
		return
	}

	response := gin.H{
		"message": "User created successfully",
		"user":    user,
	}
	if inviteToken != nil {
		response["invite_token"] = *inviteToken
		response["invite_expires_at"] = inviteExpires
		response["invite_url"] = "/invite/" + *inviteToken
		log.Warn().
			Str("email", req.Email).
			Str("invite_token", *inviteToken).
			Msg("User created via invitation; email client not configured, returning token in response")
	}

	c.JSON(http.StatusCreated, response)
}

// Get retrieves a user by ID
func (h *UserHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_id",
			"message": "Invalid user ID format",
		})
		return
	}

	// Authorization check
	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)
	currentUserID, _ := middleware.GetUserID(c)

	user, err := h.getUserByID(c, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "User not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to fetch user",
		})
		return
	}

	// Authorization: super_admin can see all, others can see users in their tenant or themselves
	if role != "super_admin" {
		userTenantIDStr := userTenantID.String()
		if user.TenantID == nil || *user.TenantID != userTenantIDStr {
			// Not in same tenant - check if viewing self
			if id != currentUserID {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "forbidden",
					"message": "You can only view users in your own tenant",
				})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}

// Update updates a user
func (h *UserHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_id",
			"message": "Invalid user ID format",
		})
		return
	}

	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)
	currentUserID, _ := middleware.GetUserID(c)

	// Fetch target user to check permissions
	targetUser, err := h.getUserByID(c, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "User not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to fetch user",
		})
		return
	}

	// Authorization checks
	isOwnProfile := id == currentUserID
	isSameTenant := targetUser.TenantID != nil && *targetUser.TenantID == userTenantID.String()

	if role != "super_admin" {
		if role == "tenant_admin" {
			// Tenant admins can update users in their tenant
			if !isSameTenant && !isOwnProfile {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "forbidden",
					"message": "You can only update users in your own tenant",
				})
				return
			}
		} else {
			// Staff/clients can only update themselves
			if !isOwnProfile {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "forbidden",
					"message": "You can only update your own profile",
				})
				return
			}
		}
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "validation_error",
			"message": err.Error(),
		})
		return
	}

	// Role change restrictions
	if req.Role != nil {
		// Only super_admin can change roles
		if role != "super_admin" && role != "tenant_admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "You cannot change user roles",
			})
			return
		}
		// Tenant admin cannot promote to tenant_admin
		if role == "tenant_admin" && *req.Role == "tenant_admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "Only super_admin can create tenant_admin users",
			})
			return
		}
		// Validate role value
		validRoles := map[string]bool{"tenant_admin": true, "staff": true, "client": true}
		if !validRoles[*req.Role] {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_role",
				"message": "Role must be tenant_admin, staff, or client",
			})
			return
		}
	}

	// Status change restrictions
	if req.Status != nil {
		validStatuses := map[string]bool{"pending": true, "active": true, "inactive": true}
		if !validStatuses[*req.Status] {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "invalid_status",
				"message": "Status must be pending, active, or inactive",
			})
			return
		}
	}

	// Hash password if provided
	var hashedPassword *string
	if req.Password != nil && len(*req.Password) >= 8 {
		hashed, err := auth.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "hash_error",
				"message": "Failed to hash password",
			})
			return
		}
		hashedPassword = &hashed
	}

	// Get TenantDB for RLS-protected queries
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Database context not available",
		})
		return
	}

	// Update user
	_, err = tenantDB.Exec(c, `
		UPDATE users SET
			name = COALESCE($2, name),
			email = COALESCE($3, email),
			password = COALESCE($4, password),
			role = COALESCE($5, role),
			status = COALESCE($6, status),
			avatar_url = COALESCE($7, avatar_url),
			phone = COALESCE($8, phone),
			specialism = COALESCE($9, specialism),
			notes = COALESCE($10, notes),
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id, req.Name, req.Email, hashedPassword, req.Role, req.Status,
		req.AvatarURL, req.Phone, req.Specialism, req.Notes)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "update_error",
			"message": "Failed to update user",
		})
		return
	}

	// Fetch the updated user
	user, err := h.getUserByID(c, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "fetch_error",
			"message": "User updated but failed to fetch",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User updated successfully",
		"user":    user,
	})
}

// Delete soft-deletes a user (super_admin or tenant_admin)
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_id",
			"message": "Invalid user ID format",
		})
		return
	}

	role, _ := middleware.GetRole(c)
	userTenantID, _ := middleware.GetTenantID(c)
	currentUserID, _ := middleware.GetUserID(c)

	// Cannot delete yourself
	if id == currentUserID {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "cannot_delete_self",
			"message": "You cannot delete your own account",
		})
		return
	}

	// Fetch target user
	targetUser, err := h.getUserByID(c, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "not_found",
				"message": "User not found",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "database_error",
			"message": "Failed to fetch user",
		})
		return
	}

	// Authorization checks
	if role != "super_admin" {
		if role == "tenant_admin" {
			// Tenant admins can only delete users in their tenant
			if targetUser.TenantID == nil || *targetUser.TenantID != userTenantID.String() {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "forbidden",
					"message": "You can only delete users in your own tenant",
				})
				return
			}
			// Cannot delete other tenant_admins
			if targetUser.Role == "tenant_admin" {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "forbidden",
					"message": "Only super_admin can delete tenant_admin users",
				})
				return
			}
		} else {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "You don't have permission to delete users",
			})
			return
		}
	}

	// Get TenantDB for RLS-protected queries
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "internal_error",
			"message": "Database context not available",
		})
		return
	}

	// Soft delete the user
	_, err = tenantDB.Exec(c, `
		UPDATE users SET deleted_at = NOW(), status = 'inactive', updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "delete_error",
			"message": "Failed to delete user",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User deleted successfully",
	})
}

// getUserByID is a helper function to fetch a user by ID using TenantDB for RLS
func (h *UserHandler) getUserByID(c *gin.Context, id uuid.UUID) (*User, error) {
	tenantDB, ok := middleware.GetTenantDB(c)
	if !ok {
		return nil, pgx.ErrNoRows // Return error if TenantDB not available
	}

	var u User
	err := tenantDB.QueryRowScan(c, []interface{}{
		&u.ID, &u.TenantID, &u.Name, &u.Email, &u.Role, &u.Status,
		&u.AvatarURL, &u.Phone, &u.Specialism, &u.Notes,
		&u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt,
	}, `
		SELECT id, tenant_id, name, email, role, status, avatar_url, phone,
		       specialism, notes, last_login_at, created_at, updated_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
