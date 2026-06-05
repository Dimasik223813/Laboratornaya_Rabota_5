package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"goapp/config"
	"goapp/models"
	"goapp/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService обрабатывает логику аутентификации/авторизации
type AuthService struct {
	users  *repository.UserRepo
	tokens *repository.TokenRepo
	db     *gorm.DB
	cache  *CacheService
}

func NewAuthService(db *gorm.DB, cache *CacheService) *AuthService {
	return &AuthService{
		users:  repository.NewUserRepo(db),
		tokens: repository.NewTokenRepo(db),
		db:     db,
		cache:  cache,
	}
}

// Проверка Access Token с проверкой в Redis
func (s *AuthService) ValidateAccessToken(tokenStr string) (*jwt.Token, error) {
	log.Println("=========================================")
	log.Println("VALIDATING ACCESS TOKEN")
	log.Println("=========================================")
	log.Printf("Token string (first 50 chars): %s...", tokenStr[:min(50, len(tokenStr))])

	// Парсим токен
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		// Проверяем метод подписи
		log.Printf("Token method: %v", token.Method)
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("ERROR: Unexpected signing method: %v", token.Method)
			return nil, fmt.Errorf("unexpected signing method: %v", token.Method)
		}
		secret := config.JwtAccessSecret()
		log.Printf("Using secret (first 10 chars): %s...", string(secret[:min(10, len(secret))]))
		return secret, nil
	})

	if err != nil {
		log.Printf("ERROR parsing token: %v", err)
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	log.Printf("Token parsed successfully, valid: %v", token.Valid)

	if !token.Valid {
		log.Printf("ERROR: Token is not valid")
		return nil, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		log.Printf("ERROR: Invalid claims type")
		return nil, errors.New("invalid claims")
	}

	log.Printf("Claims: %+v", claims)

	// Проверяем expiration
	if exp, ok := claims["exp"].(float64); ok {
		expTime := time.Unix(int64(exp), 0)
		log.Printf("Token expires at: %v", expTime)
		if time.Now().After(expTime) {
			log.Printf("ERROR: Token has expired")
			return nil, errors.New("token has expired")
		}
	}

	// Получаем JTI
	jti, ok := claims["jti"].(string)
	if !ok {
		log.Printf("ERROR: No jti in claims, type: %T", claims["jti"])
		return nil, errors.New("no jti in token")
	}
	log.Printf("JTI: %s", jti)

	// Получаем User ID
	userID, ok := claims["sub"].(string)
	if !ok {
		log.Printf("ERROR: No sub in claims, type: %T", claims["sub"])
		return nil, errors.New("no user id in token")
	}
	log.Printf("User ID: %s", userID)

	// Проверяем Redis
	if s.cache != nil && s.cache.IsEnabled() {
		redisKey := fmt.Sprintf("wp:auth:user:%s:access:%s", userID, jti)
		log.Printf("Looking for Redis key: %s", redisKey)

		var val interface{}
		err := s.cache.Get(redisKey, &val)
		if err != nil {
			log.Printf("ERROR: Redis get failed: %v", err)
			return nil, errors.New("token revoked or not found")
		}
		log.Printf("Redis value found: %v", val)
	} else {
		log.Printf("WARNING: Cache is disabled")
	}

	log.Println("✅ TOKEN VALIDATION SUCCESSFUL")
	log.Println("=========================================")
	return token, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Регистрация пользователя
func (s *AuthService) Register(email, password string) (*models.User, error) {
	if _, err := s.users.FindByEmail(email); err == nil {
		return nil, errors.New("user already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Email:    email,
		Password: string(hash),
		Salt:     "",
		YandexID: nil, // Явно устанавливаем nil вместо пустой строки
		VKID:     nil,
	}

	if err := s.users.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

// Универсальная генерация токенов с JTI
func (s *AuthService) CreateTokensForUser(user *models.User) (string, string, error) {
	log.Println("=========================================")
	log.Println("CREATING TOKENS FOR USER")
	log.Println("=========================================")
	log.Printf("User ID: %s", user.ID.String())

	// Генерация JTI
	jti := uuid.New().String()
	log.Printf("Generated JTI: %s", jti)

	// Создаем claims
	atClaims := jwt.MapClaims{
		"sub": user.ID.String(),
		"jti": jti,
		"exp": time.Now().Add(config.AccessTokenExpiration()).Unix(),
		"iat": time.Now().Unix(),
	}

	log.Printf("Claims: %+v", atClaims)

	// Создаем токен
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)

	// Подписываем токен
	secret := config.JwtAccessSecret()
	log.Printf("Secret length: %d", len(secret))

	accessToken, err := at.SignedString(secret)
	if err != nil {
		log.Printf("ERROR signing token: %v", err)
		return "", "", err
	}

	log.Printf("Access token created, length: %d", len(accessToken))
	log.Printf("Token preview: %s...", accessToken[:min(50, len(accessToken))])

	// Сохраняем в Redis
	if s.cache != nil && s.cache.IsEnabled() {
		redisKey := fmt.Sprintf("wp:auth:user:%s:access:%s", user.ID.String(), jti)
		log.Printf("Saving to Redis: %s", redisKey)

		err := s.cache.Set(redisKey, "valid", config.AccessTokenExpiration())
		if err != nil {
			log.Printf("WARNING: Failed to store JTI in Redis: %v", err)
		} else {
			log.Printf("✅ Successfully saved to Redis")
		}
	}

	// Создаем refresh token
	refreshPlain := uuid.New().String()
	sha := sha256.New()
	sha.Write([]byte(refreshPlain))
	rtHash := hex.EncodeToString(sha.Sum(nil))

	rt := &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: rtHash,
		ExpiresAt: time.Now().Add(config.RefreshTokenExpiration()),
		Revoked:   false,
	}

	if err := s.tokens.Save(rt); err != nil {
		log.Printf("ERROR saving refresh token: %v", err)
		return "", "", err
	}

	log.Println("✅ TOKENS CREATED SUCCESSFULLY")
	log.Println("=========================================")

	return accessToken, refreshPlain, nil
}

// Логин
func (s *AuthService) Login(email, password string) (string, string, error) {
	user, err := s.users.FindByEmail(email)
	if err != nil {
		return "", "", errors.New("invalid credentials")
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return "", "", errors.New("invalid credentials")
	}

	return s.CreateTokensForUser(user)
}

// Refresh токенов с инвалидацией старого
func (s *AuthService) Refresh(refreshToken string) (string, string, error) {
	hashed := sha256.Sum256([]byte(refreshToken))
	tokenHash := hex.EncodeToString(hashed[:])

	rt, err := s.tokens.FindValid(tokenHash)
	if err != nil {
		return "", "", errors.New("invalid refresh token")
	}

	user, err := s.users.FindByID(rt.UserID.String())
	if err != nil {
		return "", "", err
	}

	// Инвалидация старого Refresh токена в Redis
	if s.cache != nil && s.cache.IsEnabled() {
		oldRedisKey := fmt.Sprintf("wp:auth:user:%s:refresh:%s", user.ID.String(), rt.ID.String())
		s.cache.Delete(oldRedisKey)
	}

	// Revoke old token
	rt.Revoked = true
	_ = s.tokens.Update(rt)

	return s.CreateTokensForUser(user)
}

// Logout текущей сессии (инвалидация токена)
func (s *AuthService) Logout(refreshToken string) error {
	hashed := sha256.Sum256([]byte(refreshToken))
	tokenHash := hex.EncodeToString(hashed[:])

	rt, err := s.tokens.FindValid(tokenHash)
	if err != nil {
		return err
	}

	// Инвалидация Access токена в Redis (если знаем JTI)
	if s.cache != nil && s.cache.IsEnabled() {
		// Удаляем все Access токены пользователя
		pattern := fmt.Sprintf("wp:auth:user:%s:access:*", rt.UserID.String())
		s.cache.DeleteByPattern(pattern)
	}

	rt.Revoked = true
	return s.tokens.Update(rt)
}

// Logout всех сессий пользователя
func (s *AuthService) LogoutAll(userID string) error {
	// Инвалидация всех токенов пользователя в Redis
	if s.cache != nil && s.cache.IsEnabled() {
		pattern := fmt.Sprintf("wp:auth:user:%s:*", userID)
		s.cache.DeleteByPattern(pattern)
	}

	return s.tokens.RevokeAllByUser(userID)
}

// InvalidateUserCache инвалидирует кеш пользователя
func (s *AuthService) InvalidateUserCache(userID string) error {
	if s.cache != nil && s.cache.IsEnabled() {
		patterns := []string{
			fmt.Sprintf("wp:users:profile:%s", userID),
			fmt.Sprintf("wp:items:list:*"),
			fmt.Sprintf("wp:items:user:%s:*", userID),
		}
		for _, pattern := range patterns {
			s.cache.DeleteByPattern(pattern)
		}
	}
	return nil
}
