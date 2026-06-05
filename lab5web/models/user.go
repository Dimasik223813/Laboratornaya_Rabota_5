package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User представляет пользователя системы
type User struct {
	ID        uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	Email     string         `gorm:"unique;not null" json:"email"`
	Password  string         `gorm:"not null" json:"-"`
	Salt      string         `gorm:"not null" json:"-"`
	YandexID  *string        `gorm:"uniqueIndex" json:"-"` // Используем указатель, чтобы можно было хранить NULL
	VKID      *string        `json:"-"`                    // Используем указатель
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
