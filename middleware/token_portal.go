package middleware

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const tokenPortalSessionTokenIDKey = "token_portal_token_id"
const tokenPortalSessionUserIDKey = "token_portal_user_id"
const tokenPortalSessionTokenNameKey = "token_portal_token_name"

func sessionIntValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func SetTokenPortalSession(session sessions.Session, token *model.Token) error {
	session.Set(tokenPortalSessionTokenIDKey, token.Id)
	session.Set(tokenPortalSessionUserIDKey, token.UserId)
	session.Set(tokenPortalSessionTokenNameKey, token.Name)
	return session.Save()
}

func ClearTokenPortalSession(session sessions.Session) error {
	session.Delete(tokenPortalSessionTokenIDKey)
	session.Delete(tokenPortalSessionUserIDKey)
	session.Delete(tokenPortalSessionTokenNameKey)
	return session.Save()
}

func TokenPortalAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		session := sessions.Default(c)

		tokenID, ok := sessionIntValue(session.Get(tokenPortalSessionTokenIDKey))
		if !ok || tokenID <= 0 {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "查询会话已失效，请重新输入 API 密钥",
			})
			c.Abort()
			return
		}

		userID, ok := sessionIntValue(session.Get(tokenPortalSessionUserIDKey))
		if !ok || userID <= 0 {
			_ = ClearTokenPortalSession(session)
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "查询会话已失效，请重新输入 API 密钥",
			})
			c.Abort()
			return
		}

		token, err := model.GetTokenByIds(tokenID, userID)
		if err != nil {
			_ = ClearTokenPortalSession(session)
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "查询会话已失效，请重新输入 API 密钥",
			})
			c.Abort()
			return
		}

		userCache, err := model.GetUserCache(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": err.Error(),
			})
			c.Abort()
			return
		}
		if userCache.Status != common.UserStatusEnabled {
			_ = ClearTokenPortalSession(session)
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "用户已被封禁",
			})
			c.Abort()
			return
		}

		c.Set("id", token.UserId)
		c.Set("token_id", token.Id)
		c.Set("token_name", token.Name)
		c.Set("token_portal_token", token)
		c.Set("portal_mode", true)
		c.Next()
	}
}
