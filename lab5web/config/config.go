package config

import (
	"fmt"
	"os"
	"time"
)

// Возвращает DSN для подключения к Postgres из окружения
func GetPostgresDSN() string {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		sslmode = "disable"
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)
}

func JwtAccessSecret() []byte {
	return []byte(os.Getenv("JWT_ACCESS_SECRET"))
}
func JwtRefreshSecret() []byte {
	return []byte(os.Getenv("JWT_REFRESH_SECRET"))
}

func AccessTokenExpiration() time.Duration {
	d, _ := time.ParseDuration(os.Getenv("JWT_ACCESS_EXPIRATION"))
	return d
}
func RefreshTokenExpiration() time.Duration {
	d, _ := time.ParseDuration(os.Getenv("JWT_REFRESH_EXPIRATION"))
	return d
}
