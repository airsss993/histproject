package admin

import "time"

// Admin — модель администратора в БД.
type Admin struct {
	ID           int       `db:"id"`
	Login        string    `db:"login"`
	PasswordHash string    `db:"password_hash"`
	Role         string    `db:"role"`
	CreatedAt    time.Time `db:"created_at"`
	Email        string    `db:"email"`
}

// AdminListItem — представление администратора в списке (без пароля).
type AdminListItem struct {
	ID        int       `db:"id"         json:"id"`
	Login     string    `db:"login"       json:"login"`
	Email     string    `db:"email"       json:"email"`
	Role      string    `db:"role"        json:"role"`
	CreatedAt time.Time `db:"created_at"  json:"created_at"`
}

// LoginRequest — тело запроса на авторизацию.
type LoginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// TokenResponse — пара токенов (используется внутри сервиса).
type TokenResponse struct {
	AccessToken  string
	RefreshToken string
}

// CreateAdminRequest — тело запроса на создание админа.
type CreateAdminRequest struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role"  binding:"required,oneof=admin super_admin"`
}

// CreateAdminResponse — ответ с данными созданного админа.
type CreateAdminResponse struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// UpdateRoleRequest — тело запроса на смену роли администратора.
type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=admin super_admin"`
}

// AdminSession — сессия администратора в БД.
type AdminSession struct {
	ID           int       `db:"id"`
	AdminID      int       `db:"admin_id"`
	RefreshToken string    `db:"refresh_token"`
	ExpiresAt    time.Time `db:"expires_at"`
	CreatedAt    time.Time `db:"created_at"`
}
