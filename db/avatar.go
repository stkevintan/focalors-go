package db

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"
	"sync"

	_ "image/gif"
	_ "image/jpeg"

	"golang.org/x/image/draw"
)

const avatarKeyPrefix = "avatar:"
const avatarSize = 128

// AvatarCallback is invoked when an avatar is saved successfully.
type AvatarCallback func(userId string, base64Content string)

// AvatarStore manages user avatar storage in Redis with an in-memory cache.
type AvatarStore struct {
	redis     *Redis
	cache     sync.Map
	watcherMu sync.RWMutex
	watchers  []AvatarCallback
}

func NewAvatarStore(redis *Redis) *AvatarStore {
	return &AvatarStore{redis: redis}
}

func avatarKey(userId string) string {
	return fmt.Sprintf("%s%s", avatarKeyPrefix, userId)
}

// Save stores or updates the avatar for a given userId.
// The input base64Content is decoded, resized to 64x64 PNG, and stored.
func (s *AvatarStore) Save(userId string, base64Content string) error {
	resized, err := resizeAvatar(base64Content)
	if err != nil {
		return fmt.Errorf("resize avatar: %w", err)
	}
	key := avatarKey(userId)
	if err := s.redis.Set(key, resized, 0); err != nil {
		return err
	}
	s.cache.Store(key, resized)
	// Copy watchers slice to avoid holding the lock while callbacks are executed
	s.watcherMu.RLock()
	watchersCopy := make([]AvatarCallback, len(s.watchers))
	copy(watchersCopy, s.watchers)
	s.watcherMu.RUnlock()
	
	for _, cb := range watchersCopy {
		go cb(userId, resized)
	}
	return nil
}

// resizeAvatar decodes a base64 image, resizes it to avatarSize x avatarSize,
// and returns the result as a base64-encoded PNG string.
func resizeAvatar(base64Content string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	dst := image.NewRGBA(image.Rect(0, 0, avatarSize, avatarSize))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return "", fmt.Errorf("encode png: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// Watch registers a callback that is called whenever an avatar is saved successfully.
func (s *AvatarStore) Watch(cb AvatarCallback) {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()
	s.watchers = append(s.watchers, cb)
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

// Has returns whether the given userId has a saved avatar.
func (s *AvatarStore) Has(userId string) bool {
	key := avatarKey(userId)
	if _, ok := s.cache.Load(key); ok {
		return true
	}
	exists, _ := s.redis.Exists(key)
	return exists > 0
}

// List returns all saved avatars as a map of userId to base64 content.
func (s *AvatarStore) List() (map[string]string, error) {
	keys, err := s.redis.Keys(avatarKeyPrefix + "*")
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		userId := strings.TrimPrefix(key, avatarKeyPrefix)
		val, err := s.redis.Get(key)
		if err != nil || val == "" {
			continue
		}
		result[userId] = val
	}
	return result, nil
}
