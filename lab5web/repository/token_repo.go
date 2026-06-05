package repository

import (
	"goapp/models"

	"gorm.io/gorm"
)

type TokenRepo struct {
	db *gorm.DB
}

func NewTokenRepo(db *gorm.DB) *TokenRepo {
	return &TokenRepo{
		db: db,
	}
}

// Сохранение refresh token
func (r *TokenRepo) Save(token *models.RefreshToken) error {

	return r.db.Create(token).Error
}

// Найти валидный token
func (r *TokenRepo) FindValid(
	hash string,
) (*models.RefreshToken, error) {

	var token models.RefreshToken

	err := r.db.
		Where(
			"token_hash = ? AND revoked = false",
			hash,
		).
		First(&token).Error

	if err != nil {
		return nil, err
	}

	return &token, nil
}

// Обновление token
func (r *TokenRepo) Update(
	token *models.RefreshToken,
) error {

	return r.db.Save(token).Error
}

// Logout all sessions
func (r *TokenRepo) RevokeAllByUser(
	userID string,
) error {

	return r.db.
		Model(&models.RefreshToken{}).
		Where("user_id = ?", userID).
		Update("revoked", true).
		Error
}
