package admins

import "time"

type Role string

const (
	RoleAdmin      Role = "admin"
	RoleSuperAdmin Role = "super_admin"
)

func (role Role) Valid() bool {
	return role == RoleAdmin || role == RoleSuperAdmin
}

type Admin struct {
	ID        int64     `json:"id"`
	Login     string    `json:"login"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}
