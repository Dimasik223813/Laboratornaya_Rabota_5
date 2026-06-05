package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// CacheService предоставляет методы для работы с Redis
type CacheService struct {
	client *redis.Client
	ctx    context.Context
	ttl    time.Duration
}

// NewCacheService создает новый экземпляр CacheService
func NewCacheService(host, port, password string, db int, defaultTTL time.Duration) *CacheService {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
		DB:       db,
	})

	ctx := context.Background()

	// Проверка подключения
	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v. Cache will be disabled.", err)
		return &CacheService{
			client: nil,
			ctx:    ctx,
			ttl:    defaultTTL,
		}
	}

	log.Println("✅ Redis connected successfully")
	return &CacheService{
		client: client,
		ctx:    ctx,
		ttl:    defaultTTL,
	}
}

// IsEnabled проверяет, доступен ли Redis
func (s *CacheService) IsEnabled() bool {
	return s.client != nil
}

// Get получает значение из кеша по ключу
func (s *CacheService) Get(key string, dest interface{}) error {
	if !s.IsEnabled() {
		return fmt.Errorf("cache disabled")
	}

	data, err := s.client.Get(s.ctx, key).Bytes()
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

// Set сохраняет значение в кеш
func (s *CacheService) Set(key string, value interface{}, ttl ...time.Duration) error {
	if !s.IsEnabled() {
		return nil
	}

	expiration := s.ttl
	if len(ttl) > 0 && ttl[0] > 0 {
		expiration = ttl[0]
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return s.client.Set(s.ctx, key, data, expiration).Err()
}

// Delete удаляет ключ из кеша
func (s *CacheService) Delete(key string) error {
	if !s.IsEnabled() {
		return nil
	}
	return s.client.Del(s.ctx, key).Err()
}

// DeleteByPattern удаляет все ключи, соответствующие паттерну
func (s *CacheService) DeleteByPattern(pattern string) error {
	if !s.IsEnabled() {
		return nil
	}

	keys, err := s.client.Keys(s.ctx, pattern).Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return s.client.Del(s.ctx, keys...).Err()
	}
	return nil
}

// GetTTL возвращает время жизни ключа
func (s *CacheService) GetTTL(key string) (time.Duration, error) {
	if !s.IsEnabled() {
		return 0, fmt.Errorf("cache disabled")
	}
	return s.client.TTL(s.ctx, key).Result()
}

// Close закрывает соединение с Redis
func (s *CacheService) Close() error {
	if s.IsEnabled() {
		return s.client.Close()
	}
	return nil
}
