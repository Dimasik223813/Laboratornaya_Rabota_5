package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Article представляет основной ресурс предметной области
type Article struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Title     string    `gorm:"not null"`
	Content   string    `gorm:"type:text"`
	Status    string    `gorm:"not null"` // например, "draft", "published"
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	User      User      `gorm:"foreignKey:UserID"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"` // soft delete
}
