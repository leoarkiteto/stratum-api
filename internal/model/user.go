// Package model holds the domain types shared across modules.
package model

import "time"

// Role is a user's access role in the platform (RBAC).
type Role string

const (
	// RoleSyndic is the condo manager role (admin-level access).
	RoleSyndic Role = "syndic"
	// RoleOwner is a resident who owns a unit.
	RoleOwner Role = "owner"
	// RoleTenant is a resident who rents a unit. Same as owner, minus
	// running for syndic elections.
	RoleTenant Role = "tenant"
)

// IsValid reports whether r is one of the known roles.
func (r Role) IsValid() bool {
	switch r {
	case RoleSyndic, RoleOwner, RoleTenant:
		return true
	default:
		return false
	}
}

// User is an authenticated platform user.
type User struct {
	ID           int64
	Email        string
	Name         string
	Role         Role
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
