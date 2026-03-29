package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type tokenPortalAPIResponse struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Data    tokenPortalUserDTO `json:"data"`
}

type tokenPortalUserDTO struct {
	TokenID   int    `json:"token_id"`
	UserID    int    `json:"user_id"`
	TokenName string `json:"token_name"`
	MaskedKey string `json:"masked_key"`
}

type tokenPortalContextResponse struct {
	Success bool                  `json:"success"`
	Message string                `json:"message"`
	Data    tokenPortalContextDTO `json:"data"`
}

type tokenPortalContextDTO struct {
	UserID     int  `json:"user_id"`
	TokenID    int  `json:"token_id"`
	PortalMode bool `json:"portal_mode"`
}

type tokenPortalSessionStateResponse struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message"`
	Data    tokenPortalSessionStateDTO `json:"data"`
}

type tokenPortalSessionStateDTO struct {
	DashboardUserID   int    `json:"dashboard_user_id"`
	DashboardUsername string `json:"dashboard_username"`
	DashboardRole     int    `json:"dashboard_role"`
	DashboardStatus   int    `json:"dashboard_status"`
	HasPortalTokenID  bool   `json:"has_portal_token_id"`
	HasPortalUserID   bool   `json:"has_portal_user_id"`
	PortalTokenName   string `json:"portal_token_name"`
}

func sessionValueInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func setupTokenPortalTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	model.DB = db
	model.LOG_DB = db

	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func seedTokenPortalUser(t *testing.T, db *gorm.DB, id int, username string, status int) *model.User {
	t.Helper()

	user := &model.User{
		Id:       id,
		Username: username,
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   status,
		Group:    "default",
		AffCode:  fmt.Sprintf("aff-%d", id),
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func seedTokenPortalToken(t *testing.T, db *gorm.DB, userID int, name string, key string) *model.Token {
	t.Helper()

	token := &model.Token{
		UserId:         userID,
		Name:           name,
		Key:            key,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "default",
	}
	require.NoError(t, db.Create(token).Error)
	return token
}

func newTokenPortalRouter() *gin.Engine {
	router := gin.New()
	store := cookie.NewStore([]byte("token-portal-test"))
	router.Use(sessions.Sessions("new-api-test", store))

	router.POST("/test/session/dashboard-login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("username", "dashboard-user")
		session.Set("role", common.RoleCommonUser)
		session.Set("id", 777)
		session.Set("status", common.UserStatusEnabled)
		session.Set("group", "default")
		if err := session.Save(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		common.ApiSuccess(c, nil)
	})

	router.GET("/test/session/state", func(c *gin.Context) {
		session := sessions.Default(c)
		username, _ := session.Get("username").(string)
		role := sessionValueInt(session.Get("role"))
		userID := sessionValueInt(session.Get("id"))
		status := sessionValueInt(session.Get("status"))
		portalTokenName, _ := session.Get("token_portal_token_name").(string)

		common.ApiSuccess(c, gin.H{
			"dashboard_user_id":   userID,
			"dashboard_username":  username,
			"dashboard_role":      role,
			"dashboard_status":    status,
			"has_portal_token_id": session.Get("token_portal_token_id") != nil,
			"has_portal_user_id":  session.Get("token_portal_user_id") != nil,
			"portal_token_name":   portalTokenName,
		})
	})

	portalRoute := router.Group("/api/token-portal")
	{
		portalRoute.POST("/login", TokenPortalLogin)
		protected := portalRoute.Group("/")
		protected.Use(middleware.TokenPortalAuth())
		{
			protected.GET("/me", GetTokenPortalSession)
			protected.POST("/logout", LogoutTokenPortal)
			protected.GET("/debug-context", func(c *gin.Context) {
				common.ApiSuccess(c, gin.H{
					"user_id":     c.GetInt("id"),
					"token_id":    c.GetInt("token_id"),
					"portal_mode": c.GetBool("portal_mode"),
				})
			})
		}
	}

	return router
}

func performTokenPortalRequest(t *testing.T, router *gin.Engine, method string, target string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody *bytes.Reader
	if body != nil {
		payload, err := common.Marshal(body)
		require.NoError(t, err)
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, target, requestBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeTokenPortalAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenPortalAPIResponse {
	t.Helper()

	var response tokenPortalAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func decodeTokenPortalContextResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenPortalContextResponse {
	t.Helper()

	var response tokenPortalContextResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func decodeTokenPortalSessionStateResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenPortalSessionStateResponse {
	t.Helper()

	var response tokenPortalSessionStateResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestTokenPortalLogin_CreatesPortalSessionAndReturnsMaskedToken(t *testing.T) {
	db := setupTokenPortalTestDB(t)
	router := newTokenPortalRouter()

	user := seedTokenPortalUser(t, db, 1, "student-owner", common.UserStatusEnabled)
	token := seedTokenPortalToken(t, db, user.Id, "stu_adam", "portal1234token5678")

	loginRecorder := performTokenPortalRequest(t, router, http.MethodPost, "/api/token-portal/login", map[string]any{
		"api_key": "sk-" + token.Key,
	}, nil)

	require.Equal(t, http.StatusOK, loginRecorder.Code)
	loginResponse := decodeTokenPortalAPIResponse(t, loginRecorder)
	require.True(t, loginResponse.Success)
	assert.Equal(t, token.Id, loginResponse.Data.TokenID)
	assert.Equal(t, user.Id, loginResponse.Data.UserID)
	assert.Equal(t, token.Name, loginResponse.Data.TokenName)
	assert.Equal(t, token.GetMaskedKey(), loginResponse.Data.MaskedKey)
	assert.NotContains(t, loginRecorder.Body.String(), token.Key)

	meRecorder := performTokenPortalRequest(t, router, http.MethodGet, "/api/token-portal/me", nil, loginRecorder.Result().Cookies())

	require.Equal(t, http.StatusOK, meRecorder.Code)
	meResponse := decodeTokenPortalAPIResponse(t, meRecorder)
	require.True(t, meResponse.Success)
	assert.Equal(t, token.Id, meResponse.Data.TokenID)
	assert.Equal(t, user.Id, meResponse.Data.UserID)
	assert.Equal(t, token.Name, meResponse.Data.TokenName)
	assert.Equal(t, token.GetMaskedKey(), meResponse.Data.MaskedKey)
}

func TestTokenPortalLogin_AllowsExpiredDisabledExhaustedTokenForReadOnlyAccess(t *testing.T) {
	db := setupTokenPortalTestDB(t)
	router := newTokenPortalRouter()

	user := seedTokenPortalUser(t, db, 1, "student-owner", common.UserStatusEnabled)
	token := seedTokenPortalToken(t, db, user.Id, "stu_disabled", "portaldisabledtoken1234")
	token.Status = common.TokenStatusDisabled
	token.ExpiredTime = common.GetTimestamp() - 3600
	token.UnlimitedQuota = false
	token.RemainQuota = 0
	require.NoError(t, db.Save(token).Error)

	recorder := performTokenPortalRequest(t, router, http.MethodPost, "/api/token-portal/login", map[string]any{
		"api_key": "Bearer sk-" + token.Key,
	}, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := decodeTokenPortalAPIResponse(t, recorder)
	require.True(t, response.Success)
	assert.Equal(t, token.Id, response.Data.TokenID)
}

func TestTokenPortalLogin_RejectsDisabledOwner(t *testing.T) {
	db := setupTokenPortalTestDB(t)
	router := newTokenPortalRouter()

	user := seedTokenPortalUser(t, db, 1, "student-owner", common.UserStatusDisabled)
	token := seedTokenPortalToken(t, db, user.Id, "stu_blocked", "portalblockedtoken1234")

	recorder := performTokenPortalRequest(t, router, http.MethodPost, "/api/token-portal/login", map[string]any{
		"api_key": "sk-" + token.Key,
	}, nil)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	response := decodeTokenPortalAPIResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "封禁")
}

func TestTokenPortalMe_RejectsMissingSession(t *testing.T) {
	setupTokenPortalTestDB(t)
	router := newTokenPortalRouter()

	recorder := performTokenPortalRequest(t, router, http.MethodGet, "/api/token-portal/me", nil, nil)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	response := decodeTokenPortalAPIResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "查询会话")
}

func TestTokenPortalLogout_ClearsPortalSession(t *testing.T) {
	db := setupTokenPortalTestDB(t)
	router := newTokenPortalRouter()

	user := seedTokenPortalUser(t, db, 1, "student-owner", common.UserStatusEnabled)
	token := seedTokenPortalToken(t, db, user.Id, "stu_adam", "portallogouttoken1234")

	loginRecorder := performTokenPortalRequest(t, router, http.MethodPost, "/api/token-portal/login", map[string]any{
		"api_key": "sk-" + token.Key,
	}, nil)
	loginCookies := loginRecorder.Result().Cookies()

	logoutRecorder := performTokenPortalRequest(t, router, http.MethodPost, "/api/token-portal/logout", nil, loginCookies)

	require.Equal(t, http.StatusOK, logoutRecorder.Code)
	logoutResponse := decodeTokenPortalAPIResponse(t, logoutRecorder)
	require.True(t, logoutResponse.Success)

	meRecorder := performTokenPortalRequest(t, router, http.MethodGet, "/api/token-portal/me", nil, logoutRecorder.Result().Cookies())

	require.Equal(t, http.StatusUnauthorized, meRecorder.Code)
	meResponse := decodeTokenPortalAPIResponse(t, meRecorder)
	assert.False(t, meResponse.Success)
}

func TestTokenPortalAuth_SetsPortalContext(t *testing.T) {
	db := setupTokenPortalTestDB(t)
	router := newTokenPortalRouter()

	user := seedTokenPortalUser(t, db, 1, "student-owner", common.UserStatusEnabled)
	token := seedTokenPortalToken(t, db, user.Id, "stu_adam", "portalcontexttoken1234")

	loginRecorder := performTokenPortalRequest(t, router, http.MethodPost, "/api/token-portal/login", map[string]any{
		"api_key": "sk-" + token.Key,
	}, nil)

	contextRecorder := performTokenPortalRequest(t, router, http.MethodGet, "/api/token-portal/debug-context", nil, loginRecorder.Result().Cookies())

	require.Equal(t, http.StatusOK, contextRecorder.Code)
	response := decodeTokenPortalContextResponse(t, contextRecorder)
	require.True(t, response.Success)
	assert.Equal(t, user.Id, response.Data.UserID)
	assert.Equal(t, token.Id, response.Data.TokenID)
	assert.True(t, response.Data.PortalMode)
}

func TestTokenPortalLogout_PreservesDashboardSession(t *testing.T) {
	db := setupTokenPortalTestDB(t)
	router := newTokenPortalRouter()

	user := seedTokenPortalUser(t, db, 1, "student-owner", common.UserStatusEnabled)
	token := seedTokenPortalToken(t, db, user.Id, "stu_adam", "portalcoexisttoken1234")

	dashboardLoginRecorder := performTokenPortalRequest(
		t,
		router,
		http.MethodPost,
		"/test/session/dashboard-login",
		nil,
		nil,
	)
	require.Equal(t, http.StatusOK, dashboardLoginRecorder.Code)

	portalLoginRecorder := performTokenPortalRequest(t, router, http.MethodPost, "/api/token-portal/login", map[string]any{
		"api_key": "sk-" + token.Key,
	}, dashboardLoginRecorder.Result().Cookies())
	require.Equal(t, http.StatusOK, portalLoginRecorder.Code)

	stateBeforeLogoutRecorder := performTokenPortalRequest(
		t,
		router,
		http.MethodGet,
		"/test/session/state",
		nil,
		portalLoginRecorder.Result().Cookies(),
	)
	require.Equal(t, http.StatusOK, stateBeforeLogoutRecorder.Code)
	stateBeforeLogout := decodeTokenPortalSessionStateResponse(t, stateBeforeLogoutRecorder)
	require.True(t, stateBeforeLogout.Success)
	assert.Equal(t, 777, stateBeforeLogout.Data.DashboardUserID)
	assert.Equal(t, "dashboard-user", stateBeforeLogout.Data.DashboardUsername)
	assert.Equal(t, common.RoleCommonUser, stateBeforeLogout.Data.DashboardRole)
	assert.Equal(t, common.UserStatusEnabled, stateBeforeLogout.Data.DashboardStatus)
	assert.True(t, stateBeforeLogout.Data.HasPortalTokenID)
	assert.True(t, stateBeforeLogout.Data.HasPortalUserID)
	assert.Equal(t, token.Name, stateBeforeLogout.Data.PortalTokenName)

	logoutRecorder := performTokenPortalRequest(
		t,
		router,
		http.MethodPost,
		"/api/token-portal/logout",
		nil,
		portalLoginRecorder.Result().Cookies(),
	)
	require.Equal(t, http.StatusOK, logoutRecorder.Code)

	stateAfterLogoutRecorder := performTokenPortalRequest(
		t,
		router,
		http.MethodGet,
		"/test/session/state",
		nil,
		logoutRecorder.Result().Cookies(),
	)
	require.Equal(t, http.StatusOK, stateAfterLogoutRecorder.Code)
	stateAfterLogout := decodeTokenPortalSessionStateResponse(t, stateAfterLogoutRecorder)
	require.True(t, stateAfterLogout.Success)
	assert.Equal(t, 777, stateAfterLogout.Data.DashboardUserID)
	assert.Equal(t, "dashboard-user", stateAfterLogout.Data.DashboardUsername)
	assert.Equal(t, common.RoleCommonUser, stateAfterLogout.Data.DashboardRole)
	assert.Equal(t, common.UserStatusEnabled, stateAfterLogout.Data.DashboardStatus)
	assert.False(t, stateAfterLogout.Data.HasPortalTokenID)
	assert.False(t, stateAfterLogout.Data.HasPortalUserID)
	assert.Empty(t, stateAfterLogout.Data.PortalTokenName)
}
