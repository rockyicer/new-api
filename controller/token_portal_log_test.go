package controller

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tokenPortalLogPageResponse struct {
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Total    int         `json:"total"`
	Items    []model.Log `json:"items"`
}

type tokenPortalLogListAPIResponse struct {
	Success bool                       `json:"success"`
	Message string                     `json:"message"`
	Data    tokenPortalLogPageResponse `json:"data"`
}

type tokenPortalStatResponse struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

type tokenPortalStatAPIResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message"`
	Data    tokenPortalStatResponse `json:"data"`
}

func newTokenPortalLogRouter() *gin.Engine {
	router := gin.New()
	store := cookie.NewStore([]byte("token-portal-log-test"))
	router.Use(sessions.Sessions("new-api-test", store))

	portalRoute := router.Group("/api/token-portal")
	{
		portalRoute.POST("/login", TokenPortalLogin)
		protected := portalRoute.Group("/")
		protected.Use(middleware.TokenPortalAuth())
		{
			protected.GET("/me", GetTokenPortalSession)
			protected.POST("/logout", LogoutTokenPortal)
			protected.GET("/log", GetTokenPortalLogs)
			protected.GET("/log/stat", GetTokenPortalLogsStat)
			protected.GET("/log/export", ExportTokenPortalLogsCSV)
			protected.GET("/usage-records", GetTokenPortalLogs)
			protected.GET("/usage-summary", GetTokenPortalLogsStat)
			protected.GET("/usage-records/export", ExportTokenPortalLogsCSV)
		}
	}

	return router
}

func decodeTokenPortalLogListResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenPortalLogListAPIResponse {
	t.Helper()

	var response tokenPortalLogListAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func decodeTokenPortalStatResponse(t *testing.T, recorder *httptest.ResponseRecorder) tokenPortalStatAPIResponse {
	t.Helper()

	var response tokenPortalStatAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func TestTokenPortalLogs_ReturnsOnlyBoundTokenLogs(t *testing.T) {
	db := setupTokenPortalTestDB(t)
	router := newTokenPortalLogRouter()

	user := seedTokenPortalUser(t, db, 1, "student-owner", common.UserStatusEnabled)
	otherToken := seedTokenPortalToken(t, db, user.Id, "stu_shared", "portaltoken101")
	targetToken := seedTokenPortalToken(t, db, user.Id, "stu_shared", "portaltoken202")

	seedExportLog(t, db, &model.Log{
		UserId:      user.Id,
		Username:    user.Username,
		TokenId:     otherToken.Id,
		TokenName:   "stu_shared",
		CreatedAt:   1_700_000_100,
		Type:        model.LogTypeConsume,
		ModelName:   "gpt-4o-mini",
		RequestId:   "req-token-101",
		RequestPath: "/v1/chat/completions",
		Quota:       11,
	})
	seedExportLog(t, db, &model.Log{
		UserId:      user.Id,
		Username:    user.Username,
		TokenId:     targetToken.Id,
		TokenName:   "stu_shared",
		CreatedAt:   1_700_000_101,
		Type:        model.LogTypeConsume,
		ModelName:   "gpt-4o-mini",
		RequestId:   "req-token-202",
		RequestPath: "/v1/chat/completions",
		Quota:       22,
	})

	loginRecorder := performTokenPortalRequest(t, router, http.MethodPost, "/api/token-portal/login", map[string]any{
		"api_key": "sk-" + targetToken.Key,
	}, nil)

	listRecorder := performTokenPortalRequest(t, router, http.MethodGet, "/api/token-portal/log?p=1&page_size=10&request_path=/v1/chat/completions", nil, loginRecorder.Result().Cookies())

	require.Equal(t, http.StatusOK, listRecorder.Code)
	response := decodeTokenPortalLogListResponse(t, listRecorder)
	require.True(t, response.Success)
	require.Len(t, response.Data.Items, 1)
	assert.Equal(t, targetToken.Id, response.Data.Items[0].TokenId)
	assert.Equal(t, "req-token-202", response.Data.Items[0].RequestId)
}

func TestTokenPortalLogsStat_UsesOnlyCurrentToken(t *testing.T) {
	db := setupTokenPortalTestDB(t)
	router := newTokenPortalLogRouter()

	user := seedTokenPortalUser(t, db, 1, "student-owner", common.UserStatusEnabled)
	otherToken := seedTokenPortalToken(t, db, user.Id, "stu_shared", "portalstat101")
	targetToken := seedTokenPortalToken(t, db, user.Id, "stu_shared", "portalstat202")
	now := time.Now().Unix()

	seedExportLog(t, db, &model.Log{
		UserId:           user.Id,
		Username:         user.Username,
		TokenId:          otherToken.Id,
		TokenName:        "stu_shared",
		CreatedAt:        now,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o-mini",
		Quota:            99,
		PromptTokens:     100,
		CompletionTokens: 30,
	})
	seedExportLog(t, db, &model.Log{
		UserId:           user.Id,
		Username:         user.Username,
		TokenId:          targetToken.Id,
		TokenName:        "stu_shared",
		CreatedAt:        now,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o-mini",
		Quota:            25,
		PromptTokens:     10,
		CompletionTokens: 15,
	})
	seedExportLog(t, db, &model.Log{
		UserId:           user.Id,
		Username:         user.Username,
		TokenId:          targetToken.Id,
		TokenName:        "stu_shared",
		CreatedAt:        now - 3600,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o-mini",
		Quota:            45,
		PromptTokens:     7,
		CompletionTokens: 8,
	})

	loginRecorder := performTokenPortalRequest(t, router, http.MethodPost, "/api/token-portal/login", map[string]any{
		"api_key": "sk-" + targetToken.Key,
	}, nil)

	statRecorder := performTokenPortalRequest(t, router, http.MethodGet, "/api/token-portal/log/stat?start_timestamp=1&end_timestamp=9999999999", nil, loginRecorder.Result().Cookies())

	require.Equal(t, http.StatusOK, statRecorder.Code)
	response := decodeTokenPortalStatResponse(t, statRecorder)
	require.True(t, response.Success)
	assert.Equal(t, 70, response.Data.Quota)
	assert.Equal(t, 1, response.Data.Rpm)
	assert.Equal(t, 25, response.Data.Tpm)
}

func TestTokenPortalLogsExport_ExportsOnlyCurrentTokenLogs(t *testing.T) {
	db := setupTokenPortalTestDB(t)
	router := newTokenPortalLogRouter()

	user := seedTokenPortalUser(t, db, 1, "student-owner", common.UserStatusEnabled)
	otherToken := seedTokenPortalToken(t, db, user.Id, "stu_shared", "portalexport101")
	targetToken := seedTokenPortalToken(t, db, user.Id, "stu_shared", "portalexport202")

	seedExportLog(t, db, &model.Log{
		UserId:      user.Id,
		Username:    user.Username,
		TokenId:     otherToken.Id,
		TokenName:   "stu_shared",
		CreatedAt:   1_700_000_100,
		Type:        model.LogTypeConsume,
		ModelName:   "gpt-4o-mini",
		RequestId:   "req-export-101",
		RequestPath: "/v1/chat/completions",
		Quota:       11,
	})
	seedExportLog(t, db, &model.Log{
		UserId:      user.Id,
		Username:    user.Username,
		TokenId:     targetToken.Id,
		TokenName:   "stu_shared",
		CreatedAt:   1_700_000_101,
		Type:        model.LogTypeConsume,
		ModelName:   "gpt-4o-mini",
		RequestId:   "req-export-202",
		RequestPath: "/v1/responses",
		Quota:       22,
	})

	loginRecorder := performTokenPortalRequest(t, router, http.MethodPost, "/api/token-portal/login", map[string]any{
		"api_key": "sk-" + targetToken.Key,
	}, nil)

	exportRecorder := performTokenPortalRequest(t, router, http.MethodGet, "/api/token-portal/log/export?start_timestamp=1&end_timestamp=9999999999", nil, loginRecorder.Result().Cookies())

	require.Equal(t, http.StatusOK, exportRecorder.Code)
	assert.Contains(t, exportRecorder.Header().Get("Content-Type"), "text/csv")

	reader := csv.NewReader(strings.NewReader(exportRecorder.Body.String()))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, []string{
		"used_at",
		"username",
		"token_name",
		"model_name",
		"request_path",
		"quota",
		"prompt_tokens",
		"completion_tokens",
		"ip",
		"request_id",
		"group",
		"log_type",
	}, normalizeCSVHeader(records[0]))
	assert.Equal(t, "req-export-202", records[1][9])
	assert.Equal(t, "/v1/responses", records[1][4])
}

func TestTokenPortalUsageAliasEndpoints_ReturnBoundTokenData(t *testing.T) {
	db := setupTokenPortalTestDB(t)
	router := newTokenPortalLogRouter()

	user := seedTokenPortalUser(t, db, 1, "student-owner", common.UserStatusEnabled)
	otherToken := seedTokenPortalToken(t, db, user.Id, "stu_shared", "portalalias101")
	targetToken := seedTokenPortalToken(t, db, user.Id, "stu_shared", "portalalias202")
	now := time.Now().Unix()

	seedExportLog(t, db, &model.Log{
		UserId:           user.Id,
		Username:         user.Username,
		TokenId:          otherToken.Id,
		TokenName:        "stu_shared",
		CreatedAt:        now,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o-mini",
		RequestId:        "req-alias-101",
		RequestPath:      "/v1/chat/completions",
		Quota:            91,
		PromptTokens:     9,
		CompletionTokens: 1,
	})
	seedExportLog(t, db, &model.Log{
		UserId:           user.Id,
		Username:         user.Username,
		TokenId:          targetToken.Id,
		TokenName:        "stu_shared",
		CreatedAt:        now,
		Type:             model.LogTypeConsume,
		ModelName:        "gpt-4o-mini",
		RequestId:        "req-alias-202",
		RequestPath:      "/v1/chat/completions",
		Quota:            32,
		PromptTokens:     12,
		CompletionTokens: 20,
	})

	loginRecorder := performTokenPortalRequest(t, router, http.MethodPost, "/api/token-portal/login", map[string]any{
		"api_key": "sk-" + targetToken.Key,
	}, nil)
	loginCookies := loginRecorder.Result().Cookies()

	listRecorder := performTokenPortalRequest(t, router, http.MethodGet, "/api/token-portal/usage-records?p=1&page_size=10&request_path=/v1/chat/completions", nil, loginCookies)
	require.Equal(t, http.StatusOK, listRecorder.Code)
	listResponse := decodeTokenPortalLogListResponse(t, listRecorder)
	require.True(t, listResponse.Success)
	require.Len(t, listResponse.Data.Items, 1)
	assert.Equal(t, targetToken.Id, listResponse.Data.Items[0].TokenId)
	assert.Equal(t, "req-alias-202", listResponse.Data.Items[0].RequestId)

	statRecorder := performTokenPortalRequest(t, router, http.MethodGet, "/api/token-portal/usage-summary?start_timestamp=1&end_timestamp=9999999999", nil, loginCookies)
	require.Equal(t, http.StatusOK, statRecorder.Code)
	statResponse := decodeTokenPortalStatResponse(t, statRecorder)
	require.True(t, statResponse.Success)
	assert.Equal(t, 32, statResponse.Data.Quota)
	assert.Equal(t, 1, statResponse.Data.Rpm)
	assert.Equal(t, 32, statResponse.Data.Tpm)

	exportRecorder := performTokenPortalRequest(t, router, http.MethodGet, "/api/token-portal/usage-records/export?start_timestamp=1&end_timestamp=9999999999", nil, loginCookies)
	require.Equal(t, http.StatusOK, exportRecorder.Code)
	assert.Contains(t, exportRecorder.Header().Get("Content-Type"), "text/csv")

	reader := csv.NewReader(strings.NewReader(exportRecorder.Body.String()))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, "req-alias-202", records[1][9])
}
