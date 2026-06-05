package services

import (
	"fmt"
	"goapp/models"
	"goapp/repository"

	"gorm.io/gorm"
)

// ArticleService обрабатывает логику CRUD для статей
type ArticleService struct {
	repo  *repository.ArticleRepo
	cache *CacheService
}

func NewArticleService(db *gorm.DB, cache *CacheService) *ArticleService {
	return &ArticleService{
		repo:  repository.NewArticleRepo(db),
		cache: cache,
	}
}

// generateListCacheKey генерирует ключ для кеша списка статей
func (s *ArticleService) generateListCacheKey(userID string, page, limit int) string {
	return fmt.Sprintf("wp:items:user:%s:list:page:%d:limit:%d", userID, page, limit)
}

// generateItemCacheKey генерирует ключ для кеша одной статьи
func (s *ArticleService) generateItemCacheKey(id string) string {
	return fmt.Sprintf("wp:items:detail:%s", id)
}

// CreateArticle с инвалидацией кеша
func (s *ArticleService) CreateArticle(input *models.Article) (*models.Article, error) {
	if err := s.repo.Create(input); err != nil {
		return nil, err
	}

	// Инвалидация кеша списков
	if s.cache != nil && s.cache.IsEnabled() {
		pattern := fmt.Sprintf("wp:items:user:%s:list:*", input.UserID.String())
		s.cache.DeleteByPattern(pattern)
	}

	return input, nil
}

// GetArticleByID с кешированием
func (s *ArticleService) GetArticleByID(id string) (*models.Article, error) {
	cacheKey := s.generateItemCacheKey(id)

	// Проверка кеша
	if s.cache != nil && s.cache.IsEnabled() {
		var article models.Article
		if err := s.cache.Get(cacheKey, &article); err == nil {
			return &article, nil
		}
	}

	// Запрос из БД
	article, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}

	// Сохранение в кеш
	if s.cache != nil && s.cache.IsEnabled() {
		s.cache.Set(cacheKey, article)
	}

	return article, nil
}

// GetArticlesByUser с кешированием и пагинацией
func (s *ArticleService) GetArticlesByUser(userID string, page, limit int) ([]models.Article, int64, error) {
	cacheKey := s.generateListCacheKey(userID, page, limit)

	// Проверка кеша
	if s.cache != nil && s.cache.IsEnabled() {
		var cached struct {
			Articles []models.Article `json:"articles"`
			Total    int64            `json:"total"`
		}
		if err := s.cache.Get(cacheKey, &cached); err == nil {
			return cached.Articles, cached.Total, nil
		}
	}

	// Запрос из БД
	offset := (page - 1) * limit
	var articles []models.Article
	var total int64

	db := s.repo.GetDB()
	if err := db.Model(&models.Article{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Where("user_id = ?", userID).Offset(offset).Limit(limit).Find(&articles).Error; err != nil {
		return nil, 0, err
	}

	// Сохранение в кеш
	if s.cache != nil && s.cache.IsEnabled() {
		cachedData := struct {
			Articles []models.Article `json:"articles"`
			Total    int64            `json:"total"`
		}{
			Articles: articles,
			Total:    total,
		}
		s.cache.Set(cacheKey, cachedData)
	}

	return articles, total, nil
}

// UpdateArticle с инвалидацией кеша
func (s *ArticleService) UpdateArticle(article *models.Article, data map[string]interface{}) (*models.Article, error) {
	if title, ok := data["title"].(string); ok {
		article.Title = title
	}
	if content, ok := data["content"].(string); ok {
		article.Content = content
	}
	if status, ok := data["status"].(string); ok {
		article.Status = status
	}

	if err := s.repo.Update(article); err != nil {
		return nil, err
	}

	// Инвалидация кеша
	if s.cache != nil && s.cache.IsEnabled() {
		s.cache.Delete(s.generateItemCacheKey(article.ID.String()))
		pattern := fmt.Sprintf("wp:items:user:%s:list:*", article.UserID.String())
		s.cache.DeleteByPattern(pattern)
	}

	return article, nil
}

// DeleteArticle с инвалидацией кеша
func (s *ArticleService) DeleteArticle(article *models.Article) error {
	if err := s.repo.Delete(article); err != nil {
		return err
	}

	// Инвалидация кеша
	if s.cache != nil && s.cache.IsEnabled() {
		s.cache.Delete(s.generateItemCacheKey(article.ID.String()))
		pattern := fmt.Sprintf("wp:items:user:%s:list:*", article.UserID.String())
		s.cache.DeleteByPattern(pattern)
	}

	return nil
}
