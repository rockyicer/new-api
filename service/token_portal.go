package service

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

var ErrTokenPortalInvalidAPIKey = errors.New("API 密钥无效")
var ErrTokenPortalUserDisabled = errors.New("用户已被封禁")

func NormalizeTokenPortalAPIKey(rawKey string) string {
	rawKey = strings.TrimSpace(rawKey)
	if strings.HasPrefix(rawKey, "Bearer ") || strings.HasPrefix(rawKey, "bearer ") {
		rawKey = strings.TrimSpace(rawKey[7:])
	}
	rawKey = strings.TrimPrefix(rawKey, "sk-")
	parts := strings.Split(rawKey, "-")
	return parts[0]
}

func ResolveTokenPortalAccess(rawKey string) (*model.Token, *model.UserBase, int, error) {
	key := NormalizeTokenPortalAPIKey(rawKey)
	if key == "" {
		return nil, nil, http.StatusUnauthorized, ErrTokenPortalInvalidAPIKey
	}

	token, err := model.GetTokenByKey(key, false)
	if err != nil {
		return nil, nil, http.StatusUnauthorized, ErrTokenPortalInvalidAPIKey
	}

	userCache, err := model.GetUserCache(token.UserId)
	if err != nil {
		return nil, nil, http.StatusInternalServerError, err
	}
	if userCache.Status != common.UserStatusEnabled {
		return nil, nil, http.StatusForbidden, ErrTokenPortalUserDisabled
	}

	return token, userCache, http.StatusOK, nil
}
