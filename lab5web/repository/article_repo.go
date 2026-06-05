package repository

import (
	"goapp/models"

	"gorm.io/gorm"
)

type ArticleRepo struct {
	db *gorm.DB
}

func NewArticleRepo(db *gorm.DB) *ArticleRepo {
	return &ArticleRepo{db}
}

// GetDB возвращает экземпляр БД для сложных запросов
func (r *ArticleRepo) GetDB() *gorm.DB {
	return r.db
}

func (r *ArticleRepo) Create(article *models.Article) error {
	return r.db.Create(article).Error
}

func (r *ArticleRepo) FindByID(id string) (*models.Article, error) {
	var art models.Article
	if err := r.db.Preload("User").First(&art, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &art, nil
}

func (r *ArticleRepo) Update(article *models.Article) error {
	return r.db.Save(article).Error
}

func (r *ArticleRepo) Delete(article *models.Article) error {
	return r.db.Delete(article).Error
}

func (r *ArticleRepo) ListByUser(userID string) ([]models.Article, error) {
	var list []models.Article
	if err := r.db.Where("user_id = ?", userID).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}
