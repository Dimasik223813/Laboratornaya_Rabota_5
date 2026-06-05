package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RefreshToken хранит данные о выданных токенах обновления (Refresh Token)
type RefreshToken struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	TokenHash string    `gorm:"not null"` // хеш реального токена
	ExpiresAt time.Time `gorm:"not null"`
	Revoked   bool      `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"` // soft delete
}
