package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"goapp/config"
	"goapp/models"
	"goapp/repository"
	"goapp/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func generateState() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// RegisterInput DTO для регистрации
type RegisterInput struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com" description:"Email пользователя"`
	Password string `json:"password" binding:"required,min=6" example:"password123" description:"Пароль (минимум 6 символов)"`
}

// LoginInput DTO для входа
type LoginInput struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com" description:"Email пользователя"`
	Password string `json:"password" binding:"required" example:"password123" description:"Пароль"`
}

// UserResponse DTO для ответа
type UserResponse struct {
	ID    string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000" description:"UUID пользователя"`
	Email string `json:"email" example:"user@example.com" description:"Email пользователя"`
}

// RegisterHandler godoc
// @Summary      Регистрация нового пользователя
// @Description  Создает нового пользователя с указанным email и паролем
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body RegisterInput true "Данные для регистрации"
// @Success      201 {object} UserResponse "Пользователь успешно создан"
// @Failure      400 {object} map[string]interface{} "Неверный формат запроса или пользователь уже существует"
// @Failure      500 {object} map[string]interface{} "Внутренняя ошибка сервера"
// @Router       /auth/register [post]
func RegisterHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	auth := services.NewAuthService(db, cache)

	return func(c *gin.Context) {
		var input RegisterInput

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		user, err := auth.Register(input.Email, input.Password)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, UserResponse{
			ID:    user.ID.String(),
			Email: user.Email,
		})
	}
}

// LoginHandler godoc
// @Summary      Вход в систему
// @Description  Аутентификация пользователя. Устанавливает access_token и refresh_token в cookies.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body LoginInput true "Учетные данные"
// @Success      200 {object} map[string]interface{} "Успешный вход"
// @Failure      400 {object} map[string]interface{} "Неверный формат запроса"
// @Failure      401 {object} map[string]interface{} "Неверные учетные данные"
// @Failure      500 {object} map[string]interface{} "Внутренняя ошибка сервера"
// @Router       /auth/login [post]
func LoginHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	auth := services.NewAuthService(db, cache)

	return func(c *gin.Context) {
		var input LoginInput

		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		accessToken, refreshToken, err := auth.Login(input.Email, input.Password)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid credentials",
			})
			return
		}

		c.SetSameSite(http.SameSiteLaxMode)

		c.SetCookie(
			"access_token",
			accessToken,
			int(config.AccessTokenExpiration().Seconds()),
			"/",
			"",
			false,
			true,
		)

		c.SetCookie(
			"refresh_token",
			refreshToken,
			int(config.RefreshTokenExpiration().Seconds()),
			"/",
			"",
			false,
			true,
		)

		c.JSON(http.StatusOK, gin.H{
			"message": "logged in",
		})
	}
}

// RefreshHandler godoc
// @Summary      Обновление токенов
// @Description  Обновляет access_token и refresh_token с использованием refresh_token из cookie
// @Tags         Auth
// @Produce      json
// @Success      200 {object} map[string]interface{} "Токены успешно обновлены"
// @Failure      400 {object} map[string]interface{} "Нет refresh token"
// @Failure      401 {object} map[string]interface{} "Неверный refresh token"
// @Router       /auth/refresh [post]
func RefreshHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	auth := services.NewAuthService(db, cache)

	return func(c *gin.Context) {
		refreshToken, err := c.Cookie("refresh_token")

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "no refresh token",
			})
			return
		}

		newAccess, newRefresh, err := auth.Refresh(refreshToken)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid refresh token",
			})
			return
		}

		c.SetSameSite(http.SameSiteLaxMode)

		c.SetCookie(
			"access_token",
			newAccess,
			int(config.AccessTokenExpiration().Seconds()),
			"/",
			"",
			false,
			true,
		)

		c.SetCookie(
			"refresh_token",
			newRefresh,
			int(config.RefreshTokenExpiration().Seconds()),
			"/",
			"",
			false,
			true,
		)

		c.JSON(http.StatusOK, gin.H{
			"message": "tokens refreshed",
		})
	}
}

// WhoAmIHandler godoc
// @Summary      Информация о текущем пользователе
// @Description  Возвращает ID текущего авторизованного пользователя
// @Tags         Auth
// @Produce      json
// @Success      200 {object} map[string]interface{} "ID пользователя"
// @Failure      401 {object} map[string]interface{} "Не авторизован"
// @Router       /auth/whoami [get]
// @Security     CookieAuth
func WhoAmIHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("currentUser")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userIDStr := userID.(string)
		cacheKey := fmt.Sprintf("wp:users:profile:%s", userIDStr)

		// Проверка кеша
		if cache != nil && cache.IsEnabled() {
			var cachedUser map[string]interface{}
			if err := cache.Get(cacheKey, &cachedUser); err == nil {
				c.JSON(http.StatusOK, cachedUser)
				return
			}
		}

		// Получение из БД
		userRepo := repository.NewUserRepo(db)
		user, err := userRepo.FindByID(userIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
			return
		}

		response := gin.H{
			"id":    user.ID,
			"email": user.Email,
		}

		// Сохранение в кеш
		if cache != nil && cache.IsEnabled() {
			cache.Set(cacheKey, response)
		}

		c.JSON(http.StatusOK, response)
	}
}

// LogoutHandler godoc
// @Summary      Выход из системы
// @Description  Отзывает refresh token и очищает cookies
// @Tags         Auth
// @Produce      json
// @Success      200 {object} map[string]interface{} "Успешный выход"
// @Failure      400 {object} map[string]interface{} "Нет refresh token"
// @Router       /auth/logout [post]
// @Security     CookieAuth
func LogoutHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	auth := services.NewAuthService(db, cache)

	return func(c *gin.Context) {
		refreshToken, err := c.Cookie("refresh_token")

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "no refresh token",
			})
			return
		}

		_ = auth.Logout(refreshToken)

		c.SetCookie(
			"access_token",
			"",
			-1,
			"/",
			"",
			false,
			true,
		)

		c.SetCookie(
			"refresh_token",
			"",
			-1,
			"/",
			"",
			false,
			true,
		)

		c.JSON(http.StatusOK, gin.H{
			"message": "logged out",
		})
	}
}

// LogoutAllHandler godoc
// @Summary      Выход из всех сессий
// @Description  Отзывает все refresh токены текущего пользователя
// @Tags         Auth
// @Produce      json
// @Success      200 {object} map[string]interface{} "Успешный выход из всех сессий"
// @Failure      401 {object} map[string]interface{} "Не авторизован"
// @Router       /auth/logout-all [post]
// @Security     CookieAuth
func LogoutAllHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	auth := services.NewAuthService(db, cache)

	return func(c *gin.Context) {
		userId, _ := c.Get("currentUser")

		auth.LogoutAll(userId.(string))

		c.JSON(http.StatusOK, gin.H{
			"message": "logged out from all sessions",
		})
	}
}

// OAuthStartHandler godoc
// @Summary      Начало OAuth2 аутентификации
// @Description  Перенаправляет пользователя на страницу авторизации OAuth2 провайдера (Yandex)
// @Tags         Auth
// @Param        provider path string true "Провайдер OAuth (yandex)" example:"yandex"
// @Success      302 "Перенаправление на OAuth2 провайдера"
// @Failure      400 {object} map[string]interface{} "Неподдерживаемый провайдер"
// @Router       /auth/oauth/{provider} [get]
func OAuthStartHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := c.Param("provider")

		if provider != "yandex" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "unsupported provider",
			})
			return
		}

		state := generateState()

		c.SetSameSite(http.SameSiteLaxMode)

		c.SetCookie(
			"oauth_state",
			state,
			600,
			"/",
			"",
			false,
			true,
		)

		clientID := os.Getenv("CLIENT_ID")
		callback := os.Getenv("CALLBACK_URL")

		authURL := fmt.Sprintf(
			"https://oauth.yandex.ru/authorize?response_type=code&client_id=%s&redirect_uri=%s&state=%s",
			url.QueryEscape(clientID),
			url.QueryEscape(callback),
			url.QueryEscape(state),
		)

		c.Redirect(http.StatusFound, authURL)
	}
}

// OAuthCallbackHandler godoc
// @Summary      Callback OAuth2 аутентификации
// @Description  Обработка callback от OAuth2 провайдера. Выполняет вход или регистрацию пользователя.
// @Tags         Auth
// @Param        code query string true "Authorization code"
// @Param        state query string true "State parameter"
// @Success      200 {object} map[string]interface{} "Успешная OAuth2 аутентификация"
// @Failure      400 {object} map[string]interface{} "Неверный запрос"
// @Failure      401 {object} map[string]interface{} "Неверный state"
// @Failure      500 {object} map[string]interface{} "Внутренняя ошибка сервера"
// @Router       /auth/oauth/{provider}/callback [get]
func OAuthCallbackHandler(db *gorm.DB, cache *services.CacheService) gin.HandlerFunc {
	authService := services.NewAuthService(db, cache)

	return func(c *gin.Context) {
		code := c.Query("code")
		state := c.Query("state")

		savedState, err := c.Cookie("oauth_state")

		if err != nil || savedState != state {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid oauth state",
			})
			return
		}

		tokenResp, err := http.PostForm(
			"https://oauth.yandex.ru/token",
			url.Values{
				"grant_type":    {"authorization_code"},
				"code":          {code},
				"client_id":     {os.Getenv("CLIENT_ID")},
				"client_secret": {os.Getenv("CLIENT_SECRET")},
			},
		)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		defer tokenResp.Body.Close()

		body, _ := io.ReadAll(tokenResp.Body)

		var tokenData struct {
			AccessToken string `json:"access_token"`
		}

		if err := json.Unmarshal(body, &tokenData); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "cannot parse yandex token",
			})
			return
		}

		req, _ := http.NewRequest(
			"GET",
			"https://login.yandex.ru/info",
			nil,
		)

		req.Header.Set(
			"Authorization",
			"OAuth "+tokenData.AccessToken,
		)

		client := &http.Client{}
		resp, err := client.Do(req)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		defer resp.Body.Close()

		profileBody, _ := io.ReadAll(resp.Body)

		var profile struct {
			ID           string `json:"id"`
			DefaultEmail string `json:"default_email"`
			Login        string `json:"login"`
		}

		if err := json.Unmarshal(profileBody, &profile); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "cannot parse yandex profile",
			})
			return
		}

		userRepo := repository.NewUserRepo(db)

		user, err := userRepo.FindByYandexID(profile.ID)

		if err != nil {
			// Пользователь не найден, создаем нового
			yandexID := profile.ID // Это строка, не пустая
			user = &models.User{
				Email:    profile.DefaultEmail,
				YandexID: &yandexID, // Используем указатель
				Password: "",
				Salt:     "",
				VKID:     nil,
			}

			if createErr := userRepo.Create(user); createErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": createErr.Error(),
				})
				return
			}
		}

		accessToken, refreshToken, err := authService.CreateTokensForUser(user)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.SetSameSite(http.SameSiteLaxMode)

		c.SetCookie(
			"access_token",
			accessToken,
			int(config.AccessTokenExpiration().Seconds()),
			"/",
			"",
			false,
			true,
		)

		c.SetCookie(
			"refresh_token",
			refreshToken,
			int(config.RefreshTokenExpiration().Seconds()),
			"/",
			"",
			false,
			true,
		)

		c.JSON(http.StatusOK, gin.H{
			"message": "yandex oauth success",
			"user": gin.H{
				"id":    user.ID,
				"email": user.Email,
			},
		})
	}
}
