package db

import (
	"fmt"
	"strings"
	"time"
)

const jiandanKeyPrefix = "jiandan:"

// JiandanStore manages visited status of jiandan comments per target (user/group).
type JiandanStore struct {
	redis *Redis
}

func NewJiandanStore(redis *Redis) *JiandanStore {
	return &JiandanStore{redis: redis}
}

func jiandanKey(targetId, commentId string) string {
	return fmt.Sprintf("%s%s:%s", jiandanKeyPrefix, targetId, commentId)
}

// IsVisited checks whether a comment has been visited for the given target.
func (s *JiandanStore) IsVisited(targetId, commentId string) bool {
	key := jiandanKey(targetId, commentId)
	exists, err := s.redis.Exists(key)
	if err != nil {
		return false
	}
	return exists > 0
}

// MarkVisited marks a comment as visited for the given target.
// pics is the comma-joined pic URLs stored as the value.
// commentDate is used to calculate the TTL (15 days from the comment date).
func (s *JiandanStore) MarkVisited(targetId, commentId string, pics []string, commentDate time.Time) {
	key := jiandanKey(targetId, commentId)
	ttl := time.Until(commentDate.AddDate(0, 0, 15))
	if ttl <= 0 {
		ttl = 24 * time.Hour // minimum 1 day TTL for old posts
	}
	s.redis.Set(key, strings.Join(pics, ","), ttl)
}
