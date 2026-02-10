package service

import (
	"focalors-go/db"
	"slices"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

type Access int

const (
	GPTAccess = 1 << iota
)

var AccessNameDict = map[string]Access{
	"gpt": GPTAccess,
}

// String returns the string representation of the permission
func (p Access) String() string {
	if p == 0 {
		return "no"
	}

	// Handle multiple accesses
	var accesses []string
	for name, access := range AccessNameDict {
		if p&access != 0 {
			accesses = append(accesses, name)
		}
	}

	if len(accesses) > 0 {
		return strings.Join(accesses, "|")
	}
	return "unknown"
}

func NewAccess(accessType string) Access {
	// Handle multiple permissions separated by |
	if strings.Contains(accessType, "|") {
		var result Access
		parts := strings.Split(accessType, "|")
		for _, part := range parts {
			result |= NewAccess(strings.TrimSpace(part))
		}
		return result
	}

	// Single permission conversion
	normalized := strings.ToLower(strings.TrimSpace(accessType))

	// Handle special cases
	if normalized == "" || normalized == "no" || normalized == "none" {
		return 0
	}

	// Look up in the dictionary
	if perm, exists := AccessNameDict[normalized]; exists {
		return perm
	}

	return 0
}

type AccessService struct {
	redis *db.Redis
	admin []string
}

func NewAccessService(redis *db.Redis, admin []string) *AccessService {
	return &AccessService{
		redis: redis,
		admin: admin,
	}
}

func getKey(target string) string {
	return "access:" + target
}

type AccessItem struct {
	Target string
	Perm   Access
}

func (a *AccessService) ListAll() ([]AccessItem, error) {
	keys, err := a.redis.Keys("access:*")
	if err != nil {
		return nil, err
	}
	var results []AccessItem
	for _, key := range keys {
		target := strings.TrimPrefix(key, "access:")
		perm, err := a.GetAccess(target)
		if err != nil {
			return nil, err
		}
		results = append(results, AccessItem{
			Target: target,
			Perm:   perm,
		})
	}
	return results, nil
}

func (a *AccessService) GetAccess(user string) (Access, error) {
	key := getKey(user)
	stored, err := a.redis.Get(key)
	// redis.Nil represents a missing key
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	mask, err := strconv.Atoi(stored)
	return Access(mask), err
}

func (a *AccessService) SetAccess(user string, access Access) error {
	if a.IsAdmin(user) {
		return nil
	}
	key := getKey(user)
	return a.redis.Set(key, strconv.Itoa(int(access)), 0)
}

func (a *AccessService) AddAccess(user string, access Access) error {
	if a.IsAdmin(user) {
		return nil
	}
	currentAccess, err := a.GetAccess(user)
	if err != nil {
		return err
	}
	return a.SetAccess(user, currentAccess|access)
}

func (a *AccessService) DelAccess(user string, access Access) error {
	if a.IsAdmin(user) {
		return nil
	}
	currentAccess, err := a.GetAccess(user)
	if err != nil {
		return err
	}
	return a.SetAccess(user, currentAccess&^access)
}

func (a *AccessService) HasAccess(user string, access Access) (bool, error) {
	if a.IsAdmin(user) {
		return true, nil
	}
	currentAccess, err := a.GetAccess(user)
	if err != nil {
		return false, err
	}
	return currentAccess&access != 0, nil
}

func (a *AccessService) IsAdmin(user string) bool {
	return slices.Contains(a.admin, user)
}
