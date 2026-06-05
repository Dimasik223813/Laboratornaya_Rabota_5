package middleware

import (
	"log"
	"net/http"

	"goapp/services"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("=== AUTH MIDDLEWARE ===")

		// Получаем токен из cookie
		tokenStr, err := c.Cookie("access_token")
		log.Printf("Cookie error: %v", err)
		log.Printf("Token from cookie: %s", tokenStr[:min(50, len(tokenStr))])

		if err != nil || tokenStr == "" {
			log.Println("ERROR: No token in cookie")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing token",
			})
			return
		}

		// Валидация токена
		token, err := authService.ValidateAccessToken(tokenStr)
		if err != nil {
			log.Printf("ERROR validating token: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "invalid token",
				"details": err.Error(),
			})
			return
		}

		// Получаем user_id из claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			log.Println("ERROR: Invalid claims")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid claims",
			})
			return
		}

		userID, ok := claims["sub"].(string)
		if !ok {
			log.Println("ERROR: No user id in claims")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "no user id",
			})
			return
		}

		log.Printf("✅ User authenticated: %s", userID)
		c.Set("currentUser", userID)
		c.Next()
	}
}
