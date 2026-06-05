package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"goapp/models"
	"goapp/services"
)

// ArticleCreateInput DTO для создания статьи
type ArticleCreateInput struct {
	Title   string `json:"title" binding:"required" example:"Моя первая статья" description:"Заголовок статьи"`
	Content string `json:"content" example:"Текст статьи..." description:"Содержимое статьи"`
	Status  string `json:"status" binding:"required" example:"draft" description:"Статус статьи (draft, published)" enums:"draft,published"`
}

// ArticleUpdateInput DTO для обновления статьи
type ArticleUpdateInput struct {
	Title   string `json:"title" example:"Обновленный заголовок" description:"Заголовок статьи"`
	Content string `json:"content" example:"Обновленный текст..." description:"Содержимое статьи"`
	Status  string `json:"status" example:"published" description:"Статус статьи" enums:"draft,published"`
}

// ArticleResponse DTO для ответа
type ArticleResponse struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000" description:"UUID статьи"`
	Title     string `json:"title" example:"Моя первая статья" description:"Заголовок статьи"`
	Content   string `json:"content" example:"Текст статьи..." description:"Содержимое статьи"`
	Status    string `json:"status" example:"published" description:"Статус статьи"`
	UserID    string `json:"user_id" example:"550e8400-e29b-41d4-a716-446655440001" description:"ID владельца"`
	CreatedAt string `json:"created_at" example:"2024-01-15T12:00:00Z" description:"Дата создания"`
	UpdatedAt string `json:"updated_at" example:"2024-01-15T12:30:00Z" description:"Дата обновления"`
}

// CreateArticleHandler godoc
// @Summary      Создание новой статьи
// @Description  Создает новую статью для авторизованного пользователя
// @Tags         Articles
// @Accept       json
// @Produce      json
// @Param        request body ArticleCreateInput true "Данные для создания статьи"
// @Success      201 {object} ArticleResponse "Статья успешно создана"
// @Failure      400 {object} map[string]interface{} "Неверный формат запроса"
// @Failure      401 {object} map[string]interface{} "Не авторизован"
// @Failure      500 {object} map[string]interface{} "Внутренняя ошибка сервера"
// @Router       /api/articles [post]
// @Security     CookieAuth
func CreateArticleHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	svc := services.NewArticleService(db, cache)

	return func(c *gin.Context) {
		var input ArticleCreateInput

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		userIDRaw, exists := c.Get("currentUser")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userIDStr, ok := userIDRaw.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id type"})
			return
		}

		userUUID, err := uuid.Parse(userIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
			return
		}

		art := &models.Article{
			Title:   input.Title,
			Content: input.Content,
			Status:  input.Status,
			UserID:  userUUID,
		}

		created, err := svc.CreateArticle(art)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot create article"})
			return
		}

		c.JSON(http.StatusCreated, ArticleResponse{
			ID:        created.ID.String(),
			Title:     created.Title,
			Content:   created.Content,
			Status:    created.Status,
			UserID:    created.UserID.String(),
			CreatedAt: created.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: created.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
}

// GetArticlesHandler godoc
// @Summary      Получение списка статей пользователя
// @Description  Возвращает все статьи текущего авторизованного пользователя с пагинацией
// @Tags         Articles
// @Produce      json
// @Param        page query int false "Номер страницы" default(1) example(1)
// @Param        limit query int false "Количество записей на странице" default(10) example(10)
// @Success      200 {object} map[string]interface{} "Список статей"
// @Failure      401 {object} map[string]interface{} "Не авторизован"
// @Failure      500 {object} map[string]interface{} "Внутренняя ошибка сервера"
// @Router       /api/articles [get]
// @Security     CookieAuth
func GetArticlesHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	svc := services.NewArticleService(db, cache)

	return func(c *gin.Context) {
		userIDRaw, exists := c.Get("currentUser")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userIDStr, ok := userIDRaw.(string)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid user id type"})
			return
		}

		// Пагинация
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 10
		}
		if limit > 100 {
			limit = 100
		}

		articles, total, err := svc.GetArticlesByUser(userIDStr, page, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot fetch articles"})
			return
		}

		response := make([]ArticleResponse, len(articles))
		for i, art := range articles {
			response[i] = ArticleResponse{
				ID:        art.ID.String(),
				Title:     art.Title,
				Content:   art.Content,
				Status:    art.Status,
				UserID:    art.UserID.String(),
				CreatedAt: art.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt: art.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"data":  response,
			"total": total,
			"page":  page,
			"limit": limit,
		})
	}
}

// GetArticleByIDHandler godoc
// @Summary      Получение статьи по ID
// @Description  Возвращает статью по указанному ID (только если пользователь является владельцем)
// @Tags         Articles
// @Produce      json
// @Param        id path string true "UUID статьи" example:"550e8400-e29b-41d4-a716-446655440000"
// @Success      200 {object} ArticleResponse "Статья найдена"
// @Failure      401 {object} map[string]interface{} "Не авторизован"
// @Failure      403 {object} map[string]interface{} "Доступ запрещен (не владелец)"
// @Failure      404 {object} map[string]interface{} "Статья не найдена"
// @Failure      500 {object} map[string]interface{} "Внутренняя ошибка сервера"
// @Router       /api/articles/{id} [get]
// @Security     CookieAuth
func GetArticleByIDHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	svc := services.NewArticleService(db, cache)

	return func(c *gin.Context) {
		id := c.Param("id")
		userIDRaw, _ := c.Get("currentUser")
		userIDStr := userIDRaw.(string)

		article, err := svc.GetArticleByID(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
			return
		}

		if article.UserID.String() != userIDStr {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		c.JSON(http.StatusOK, ArticleResponse{
			ID:        article.ID.String(),
			Title:     article.Title,
			Content:   article.Content,
			Status:    article.Status,
			UserID:    article.UserID.String(),
			CreatedAt: article.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: article.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
}

// UpdateArticleHandler godoc
// @Summary      Обновление статьи
// @Description  Обновляет существующую статью (только если пользователь является владельцем)
// @Tags         Articles
// @Accept       json
// @Produce      json
// @Param        id path string true "UUID статьи" example:"550e8400-e29b-41d4-a716-446655440000"
// @Param        request body ArticleUpdateInput true "Данные для обновления"
// @Success      200 {object} ArticleResponse "Статья успешно обновлена"
// @Failure      400 {object} map[string]interface{} "Неверный формат запроса"
// @Failure      401 {object} map[string]interface{} "Не авторизован"
// @Failure      403 {object} map[string]interface{} "Доступ запрещен (не владелец)"
// @Failure      404 {object} map[string]interface{} "Статья не найдена"
// @Failure      500 {object} map[string]interface{} "Внутренняя ошибка сервера"
// @Router       /api/articles/{id} [put]
// @Security     CookieAuth
func UpdateArticleHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	svc := services.NewArticleService(db, cache)

	return func(c *gin.Context) {
		id := c.Param("id")
		userIDRaw, _ := c.Get("currentUser")
		userIDStr := userIDRaw.(string)

		var input ArticleUpdateInput
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		article, err := svc.GetArticleByID(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
			return
		}

		if article.UserID.String() != userIDStr {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		updateData := make(map[string]interface{})
		if input.Title != "" {
			updateData["title"] = input.Title
		}
		if input.Content != "" {
			updateData["content"] = input.Content
		}
		if input.Status != "" {
			updateData["status"] = input.Status
		}

		updated, err := svc.UpdateArticle(article, updateData)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot update article"})
			return
		}

		c.JSON(http.StatusOK, ArticleResponse{
			ID:        updated.ID.String(),
			Title:     updated.Title,
			Content:   updated.Content,
			Status:    updated.Status,
			UserID:    updated.UserID.String(),
			CreatedAt: updated.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: updated.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
}

// DeleteArticleHandler godoc
// @Summary      Удаление статьи
// @Description  Удаляет статью (soft delete) (только если пользователь является владельцем)
// @Tags         Articles
// @Produce      json
// @Param        id path string true "UUID статьи" example:"550e8400-e29b-41d4-a716-446655440000"
// @Success      200 {object} map[string]interface{} "Статья успешно удалена"
// @Failure      401 {object} map[string]interface{} "Не авторизован"
// @Failure      403 {object} map[string]interface{} "Доступ запрещен (не владелец)"
// @Failure      404 {object} map[string]interface{} "Статья не найдена"
// @Failure      500 {object} map[string]interface{} "Внутренняя ошибка сервера"
// @Router       /api/articles/{id} [delete]
// @Security     CookieAuth
func DeleteArticleHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	svc := services.NewArticleService(db, cache)

	return func(c *gin.Context) {
		id := c.Param("id")
		userIDRaw, _ := c.Get("currentUser")
		userIDStr := userIDRaw.(string)

		article, err := svc.GetArticleByID(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Article not found"})
			return
		}

		if article.UserID.String() != userIDStr {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		if err := svc.DeleteArticle(article); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Cannot delete article"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Article deleted successfully"})
	}
}
