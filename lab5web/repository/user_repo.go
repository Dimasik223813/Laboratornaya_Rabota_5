package repository

import (
	"goapp/models"

	"gorm.io/gorm"
)

type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db}
}

// Создание нового пользователя
func (r *UserRepo) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// Найти по email
func (r *UserRepo) FindByEmail(email string) (*models.User, error) {
	var user models.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Найти по ID
func (r *UserRepo) FindByID(id string) (*models.User, error) {
	var user models.User
	if err := r.db.First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Найти по YandexID
// FindByYandexID находит пользователя по YandexID
func (r *UserRepo) FindByYandexID(yandexID string) (*models.User, error) {
	var user models.User

	// Ищем только если yandexID не пустой
	if yandexID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	if err := r.db.Where("yandex_id = ?", yandexID).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}
