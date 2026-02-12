package db

import (
	"fmt"
	"sync"
)

const avatarKeyPrefix = "avatar:"

// AvatarStore manages user avatar storage in Redis with an in-memory cache.
type AvatarStore struct {
	redis *Redis
	cache sync.Map
}

func NewAvatarStore(redis *Redis) *AvatarStore {
	return &AvatarStore{redis: redis}
}

func avatarKey(userId string) string {
	return fmt.Sprintf("%s%s", avatarKeyPrefix, userId)
}

// Save stores or updates the avatar base64 content for a given userId.
func (s *AvatarStore) Save(userId string, base64Content string) error {
	key := avatarKey(userId)
	if err := s.redis.Set(key, base64Content, 0); err != nil {
		return err
	}
	s.cache.Store(key, base64Content)
	return nil
}

// Get returns the avatar base64 content for a given userId.
// Returns empty string if not found.
func (s *AvatarStore) Get(userId string) (string, bool) {
	key := avatarKey(userId)

	// Check in-memory cache first
	if val, ok := s.cache.Load(key); ok {
		return val.(string), true
	}

	// Fall back to Redis
	val, err := s.redis.Get(key)
	if err != nil || val == "" {
		return "", false
	}

	s.cache.Store(key, val)
	return val, true
}
