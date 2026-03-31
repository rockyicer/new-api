package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodeSidebarModulesForTest(t *testing.T, raw string) map[string]map[string]bool {
	t.Helper()

	var config map[string]map[string]bool
	require.NoError(t, common.UnmarshalJsonStr(raw, &config))
	return config
}

func encodeSidebarModulesForTest(t *testing.T, config map[string]map[string]bool) string {
	t.Helper()

	encoded, err := common.Marshal(config)
	require.NoError(t, err)
	return string(encoded)
}

func TestGenerateDefaultSidebarConfigForRole_HidesChatSectionByDefault(t *testing.T) {
	config := decodeSidebarModulesForTest(t, generateDefaultSidebarConfigForRole(common.RoleCommonUser))

	require.Contains(t, config, "chat")
	assert.False(t, config["chat"]["enabled"])
	assert.True(t, config["chat"]["playground"])
	assert.True(t, config["chat"]["chat"])
	assert.False(t, config["console"]["midjourney"])
}

func TestBackfillSidebarChatHiddenDefault_UpdatesExistingUsersOnce(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))

	legacySidebarConfig := map[string]map[string]bool{
		"chat": {
			"enabled":    true,
			"playground": true,
			"chat":       false,
		},
		"console": {
			"enabled": true,
			"detail":  true,
			"token":   false,
		},
	}

	legacyUser := User{
		Username: "legacy-chat-user",
		Password: "secret",
		AffCode:  "legacy-aff-code",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	legacyUser.SetSetting(dto.UserSetting{
		Language:       "zh-CN",
		SidebarModules: encodeSidebarModulesForTest(t, legacySidebarConfig),
	})
	require.NoError(t, DB.Create(&legacyUser).Error)

	missingSidebarUser := User{
		Username: "missing-sidebar-user",
		Password: "secret",
		AffCode:  "missing-aff-code",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	missingSidebarUser.SetSetting(dto.UserSetting{
		Language: "en",
	})
	require.NoError(t, DB.Create(&missingSidebarUser).Error)

	require.NoError(t, BackfillSidebarChatHiddenDefault())

	var reloadedLegacyUser User
	require.NoError(t, DB.First(&reloadedLegacyUser, legacyUser.Id).Error)
	legacySetting := reloadedLegacyUser.GetSetting()
	assert.Equal(t, "zh-CN", legacySetting.Language)
	legacySidebar := decodeSidebarModulesForTest(t, legacySetting.SidebarModules)
	assert.False(t, legacySidebar["chat"]["enabled"])
	assert.True(t, legacySidebar["chat"]["playground"])
	assert.False(t, legacySidebar["chat"]["chat"])
	assert.False(t, legacySidebar["console"]["token"])

	var reloadedMissingSidebarUser User
	require.NoError(t, DB.First(&reloadedMissingSidebarUser, missingSidebarUser.Id).Error)
	missingSidebarSetting := reloadedMissingSidebarUser.GetSetting()
	assert.Equal(t, "en", missingSidebarSetting.Language)
	missingSidebarConfig := decodeSidebarModulesForTest(t, missingSidebarSetting.SidebarModules)
	assert.False(t, missingSidebarConfig["chat"]["enabled"])
	assert.True(t, missingSidebarConfig["chat"]["playground"])
	assert.True(t, missingSidebarConfig["chat"]["chat"])

	legacySidebar["chat"]["enabled"] = true
	legacySetting.SidebarModules = encodeSidebarModulesForTest(t, legacySidebar)
	reloadedLegacyUser.SetSetting(legacySetting)
	require.NoError(t, reloadedLegacyUser.Update(false))

	require.NoError(t, BackfillSidebarChatHiddenDefault())

	var reopenedLegacyUser User
	require.NoError(t, DB.First(&reopenedLegacyUser, legacyUser.Id).Error)
	reopenedLegacySetting := reopenedLegacyUser.GetSetting()
	reopenedLegacySidebar := decodeSidebarModulesForTest(t, reopenedLegacySetting.SidebarModules)
	assert.True(t, reopenedLegacySidebar["chat"]["enabled"])
	assert.Equal(t, sidebarChatHiddenDefaultBackfillStatusCompleted, getOptionValueForTest(t, sidebarChatHiddenDefaultBackfillStatusKey))
}

func TestBackfillSidebarDrawingLogHiddenDefault_UpdatesExistingUsersOnce(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))

	userSidebarConfig := map[string]map[string]bool{
		"chat": {
			"enabled":    true,
			"playground": true,
			"chat":       true,
		},
		"console": {
			"enabled":    true,
			"detail":     true,
			"token":      false,
			"log":        true,
			"midjourney": true,
			"task":       true,
		},
	}

	user := User{
		Username: "drawing-log-user",
		Password: "secret",
		AffCode:  "drawing-log-aff-code",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	user.SetSetting(dto.UserSetting{
		Language:       "zh-CN",
		SidebarModules: encodeSidebarModulesForTest(t, userSidebarConfig),
	})
	require.NoError(t, DB.Create(&user).Error)

	require.NoError(t, BackfillSidebarDrawingLogHiddenDefault())

	var reloadedUser User
	require.NoError(t, DB.First(&reloadedUser, user.Id).Error)
	userSetting := reloadedUser.GetSetting()
	sidebarConfig := decodeSidebarModulesForTest(t, userSetting.SidebarModules)
	assert.True(t, sidebarConfig["chat"]["enabled"])
	assert.False(t, sidebarConfig["console"]["midjourney"])
	assert.False(t, sidebarConfig["console"]["token"])

	sidebarConfig["console"]["midjourney"] = true
	userSetting.SidebarModules = encodeSidebarModulesForTest(t, sidebarConfig)
	reloadedUser.SetSetting(userSetting)
	require.NoError(t, reloadedUser.Update(false))

	require.NoError(t, BackfillSidebarDrawingLogHiddenDefault())

	var reopenedUser User
	require.NoError(t, DB.First(&reopenedUser, user.Id).Error)
	reopenedSetting := reopenedUser.GetSetting()
	reopenedConfig := decodeSidebarModulesForTest(t, reopenedSetting.SidebarModules)
	assert.True(t, reopenedConfig["console"]["midjourney"])
	assert.Equal(t, sidebarDrawingLogHiddenDefaultBackfillCompleted, getOptionValueForTest(t, sidebarDrawingLogHiddenDefaultBackfillStatusKey))
}
