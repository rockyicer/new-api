package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type TokenPortalLoginRequest struct {
	APIKey string `json:"api_key"`
}

type TokenPortalSessionResponse struct {
	TokenID   int    `json:"token_id"`
	UserID    int    `json:"user_id"`
	TokenName string `json:"token_name"`
	MaskedKey string `json:"masked_key"`
}

func buildTokenPortalSessionResponse(token *model.Token) TokenPortalSessionResponse {
	return TokenPortalSessionResponse{
		TokenID:   token.Id,
		UserID:    token.UserId,
		TokenName: token.Name,
		MaskedKey: token.GetMaskedKey(),
	}
}

func TokenPortalLogin(c *gin.Context) {
	var request TokenPortalLoginRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "参数错误",
		})
		return
	}
	if strings.TrimSpace(request.APIKey) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请输入 API 密钥",
		})
		return
	}

	token, _, statusCode, err := service.ResolveTokenPortalAccess(request.APIKey)
	if err != nil {
		c.JSON(statusCode, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	session := sessions.Default(c)
	if err := middleware.SetTokenPortalSession(session, token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "保存查询会话失败",
		})
		return
	}

	common.ApiSuccess(c, buildTokenPortalSessionResponse(token))
}

func GetTokenPortalSession(c *gin.Context) {
	token, ok := c.Get("token_portal_token")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "查询会话已失效，请重新输入 API 密钥",
		})
		return
	}

	tokenModel, ok := token.(*model.Token)
	if !ok || tokenModel == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "查询会话已失效，请重新输入 API 密钥",
		})
		return
	}

	common.ApiSuccess(c, buildTokenPortalSessionResponse(tokenModel))
}

func LogoutTokenPortal(c *gin.Context) {
	session := sessions.Default(c)
	if err := middleware.ClearTokenPortalSession(session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "退出查询失败",
		})
		return
	}

	common.ApiSuccess(c, nil)
}
