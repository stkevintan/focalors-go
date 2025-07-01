package service

import (
	"focalors-go/db"
	"focalors-go/wechat"
	"strconv"
	"strings"
)

type Perm int

const (
	GPTPerm = 1 << iota
)

var PermNameDict = map[string]Perm{
	"gpt": GPTPerm,
}

// String returns the string representation of the permission
func (p Perm) String() string {
	if p == 0 {
		return "no"
	}

	// Handle multiple permissions
	var perms []string
	for name, perm := range PermNameDict {
		if p&perm != 0 {
			perms = append(perms, name)
		}
	}

	if len(perms) > 0 {
		return strings.Join(perms, "|")
	}
	return "unknown"
}

func NewPerm(permType string) Perm {
	// Handle multiple permissions separated by |
	if strings.Contains(permType, "|") {
		var result Perm
		parts := strings.Split(permType, "|")
		for _, part := range parts {
			result |= NewPerm(strings.TrimSpace(part))
		}
		return result
	}

	// Single permission conversion
	normalized := strings.ToLower(strings.TrimSpace(permType))

	// Handle special cases
	if normalized == "" || normalized == "no" || normalized == "none" {
		return 0
	}

	// Look up in the dictionary
	if perm, exists := PermNameDict[normalized]; exists {
		return perm
	}

	return 0
}

type AccessService struct {
	w     *wechat.WechatClient
	redis *db.Redis
	admin string
}

func NewAccessService(w *wechat.WechatClient, redis *db.Redis, admin string) *AccessService {
	return &AccessService{
		w:     w,
		redis: redis,
		admin: admin,
	}
}

func getKey(target string) string {
	return "access:" + target
}

type TargetAndPerm struct {
	Target string
	Perm   Perm
}

func (a *AccessService) ListTargetAndPerm() ([]TargetAndPerm, error) {
	keys, err := a.redis.Keys("access:*")
	if err != nil {
		return nil, err
	}
	var results []TargetAndPerm
	for _, key := range keys {
		target := strings.TrimPrefix(key, "access:")
		perm, err := a.GetPerm(wechat.NewTarget(target))
		if err != nil {
			return nil, err
		}
		results = append(results, TargetAndPerm{
			Target: target,
			Perm:   perm,
		})
	}
	return results, nil
}

func (a *AccessService) GetPerm(target wechat.WechatTarget) (Perm, error) {
	key := getKey(target.GetTarget())
	stored, err := a.redis.Get(key)
	if err != nil {
		return 0, err
	}
	mask, err := strconv.Atoi(stored)
	return Perm(mask), err
}

func (a *AccessService) SetPerm(target wechat.WechatTarget, perm Perm) error {
	if a.IsAdmin(target) {
		return nil
	}
	key := getKey(target.GetTarget())
	return a.redis.Set(key, strconv.Itoa(int(perm)), 0)
}

func (a *AccessService) AddPerm(target wechat.WechatTarget, perm Perm) error {
	if a.IsAdmin(target) {
		return nil
	}
	currentPerm, err := a.GetPerm(target)
	if err != nil {
		return err
	}
	return a.SetPerm(target, currentPerm|perm)
}

func (a *AccessService) RemovePerm(target wechat.WechatTarget, perm Perm) error {
	if a.IsAdmin(target) {
		return nil
	}
	currentPerm, err := a.GetPerm(target)
	if err != nil {
		return err
	}
	return a.SetPerm(target, currentPerm&^perm)
}

func (a *AccessService) HasPerm(target wechat.WechatTarget, perm Perm) (bool, error) {
	if a.IsAdmin(target) {
		return true, nil
	}
	currentPerm, err := a.GetPerm(target)
	if err != nil {
		return false, err
	}
	return currentPerm&perm != 0, nil
}

func (a *AccessService) IsAdmin(target wechat.WechatTarget) bool {
	return target.GetTarget() == a.admin
}
