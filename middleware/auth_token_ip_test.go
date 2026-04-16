package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type tokenAuthErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

func setupTokenAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db

	if err := db.AutoMigrate(&model.User{}, &model.Token{}); err != nil {
		t.Fatalf("failed to migrate auth test tables: %v", err)
	}
	ensureTokenAuthDenyIPColumn(t, db)

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func ensureTokenAuthDenyIPColumn(t *testing.T, db *gorm.DB) {
	t.Helper()

	type tableInfoRow struct {
		Name string `gorm:"column:name"`
	}

	var columns []tableInfoRow
	if err := db.Raw("PRAGMA table_info(tokens)").Scan(&columns).Error; err != nil {
		t.Fatalf("failed to inspect token columns: %v", err)
	}

	for _, column := range columns {
		if column.Name == "deny_ips" {
			return
		}
	}

	if err := db.Exec("ALTER TABLE tokens ADD COLUMN deny_ips TEXT DEFAULT ''").Error; err != nil {
		t.Fatalf("failed to add token deny_ips column: %v", err)
	}
}

func seedTokenAuthUser(t *testing.T, db *gorm.DB, userID int) {
	t.Helper()

	user := &model.User{
		Id:       userID,
		Username: fmt.Sprintf("user-%d", userID),
		Password: "password123",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Email:    fmt.Sprintf("user-%d@example.com", userID),
		Group:    "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create auth test user: %v", err)
	}
}

func seedTokenAuthToken(t *testing.T, db *gorm.DB, userID int, rawKey string, allowIPs string, denyIPs string) {
	t.Helper()

	token := &model.Token{
		UserId:         userID,
		Name:           "auth-ip-token",
		Key:            rawKey,
		Status:         common.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    -1,
		RemainQuota:    100,
		UnlimitedQuota: true,
		Group:          "",
	}
	if err := db.Create(token).Error; err != nil {
		t.Fatalf("failed to create auth test token: %v", err)
	}
	if err := db.Exec("UPDATE tokens SET allow_ips = ?, deny_ips = ? WHERE id = ?", allowIPs, denyIPs, token.Id).Error; err != nil {
		t.Fatalf("failed to set auth test token ip rules: %v", err)
	}
}

func performTokenAuthRequest(t *testing.T, rawKey string, clientIP string) *httptest.ResponseRecorder {
	t.Helper()

	router := gin.New()
	router.GET("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-"+rawKey)
	request.RemoteAddr = clientIP + ":12345"

	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeTokenAuthError(t *testing.T, recorder *httptest.ResponseRecorder) tokenAuthErrorResponse {
	t.Helper()

	var response tokenAuthErrorResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode token auth error response: %v", err)
	}
	return response
}

func TestTokenAuthRejectsDeniedIP(t *testing.T) {
	db := setupTokenAuthTestDB(t)
	seedTokenAuthUser(t, db, 1)
	seedTokenAuthToken(t, db, 1, "denyiptoken", "", "203.0.113.7")

	recorder := performTokenAuthRequest(t, "denyiptoken", "203.0.113.7")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected denied ip to be rejected with 403, got %d", recorder.Code)
	}

	response := decodeTokenAuthError(t, recorder)
	if response.Error.Code != string(types.ErrorCodeAccessDenied) {
		t.Fatalf("expected access_denied code, got %q", response.Error.Code)
	}
}

func TestTokenAuthAllowsWhitelistedIPWhenNotDenied(t *testing.T) {
	db := setupTokenAuthTestDB(t)
	seedTokenAuthUser(t, db, 1)
	seedTokenAuthToken(t, db, 1, "allowiptoken", "198.51.100.8", "203.0.113.0/24")

	recorder := performTokenAuthRequest(t, "allowiptoken", "198.51.100.8")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected allowed ip to pass, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestTokenAuthPrefersDenyListOverAllowList(t *testing.T) {
	db := setupTokenAuthTestDB(t)
	seedTokenAuthUser(t, db, 1)
	seedTokenAuthToken(t, db, 1, "conflictiptoken", "198.51.100.0/24", "198.51.100.8")

	recorder := performTokenAuthRequest(t, "conflictiptoken", "198.51.100.8")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected deny list to override allow list with 403, got %d", recorder.Code)
	}

	response := decodeTokenAuthError(t, recorder)
	if response.Error.Code != string(types.ErrorCodeAccessDenied) {
		t.Fatalf("expected access_denied code, got %q", response.Error.Code)
	}
}

func TestTokenAuthKeepsAllowListBehaviorWithoutDenyList(t *testing.T) {
	db := setupTokenAuthTestDB(t)
	seedTokenAuthUser(t, db, 1)
	seedTokenAuthToken(t, db, 1, "allowonlytoken", "198.51.100.0/24", "")

	recorder := performTokenAuthRequest(t, "allowonlytoken", "203.0.113.9")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected allow list mismatch to be rejected with 403, got %d", recorder.Code)
	}

	response := decodeTokenAuthError(t, recorder)
	if response.Error.Code != string(types.ErrorCodeAccessDenied) {
		t.Fatalf("expected access_denied code, got %q", response.Error.Code)
	}
}
