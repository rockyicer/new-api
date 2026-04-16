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

func TestGenerateDefaultSidebarConfigForRole_UsesVisibleDefaults(t *testing.T) {
	config := decodeSidebarModulesForTest(
		t,
		generateDefaultSidebarConfigForRole(common.RoleCommonUser),
	)

	require.Contains(t, config, "chat")
	assert.True(t, config["chat"]["enabled"])
	assert.True(t, config["chat"]["playground"])
	assert.True(t, config["chat"]["chat"])
	assert.True(t, config["console"]["midjourney"])
	assert.True(t, config["console"]["task"])
}

func TestBackfillSidebarLegacyVisibleDefaults_UpdatesExistingUsersOnce(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))

	legacySidebarConfig := map[string]map[string]bool{
		"chat": {
			"enabled":    false,
			"playground": true,
			"chat":       true,
		},
		"console": {
			"enabled":    true,
			"detail":     true,
			"token":      false,
			"log":        true,
			"midjourney": false,
			"task":       false,
		},
	}

	legacyUser := User{
		Username: "legacy-sidebar-user",
		Password: "secret",
		AffCode:  "legacy-sidebar-aff",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	legacyUser.SetSetting(dto.UserSetting{
		Language:       "zh-CN",
		SidebarModules: encodeSidebarModulesForTest(t, legacySidebarConfig),
	})
	require.NoError(t, DB.Create(&legacyUser).Error)

	missingSidebarUser := User{
		Username: "missing-sidebar-visible-user",
		Password: "secret",
		AffCode:  "missing-sidebar-aff",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	missingSidebarUser.SetSetting(dto.UserSetting{
		Language: "en",
	})
	require.NoError(t, DB.Create(&missingSidebarUser).Error)

	require.NoError(t, BackfillSidebarLegacyVisibleDefaults())

	var reloadedLegacyUser User
	require.NoError(t, DB.First(&reloadedLegacyUser, legacyUser.Id).Error)
	legacySetting := reloadedLegacyUser.GetSetting()
	assert.Equal(t, "zh-CN", legacySetting.Language)
	legacySidebar := decodeSidebarModulesForTest(t, legacySetting.SidebarModules)
	assert.True(t, legacySidebar["chat"]["enabled"])
	assert.True(t, legacySidebar["chat"]["playground"])
	assert.True(t, legacySidebar["chat"]["chat"])
	assert.False(t, legacySidebar["console"]["token"])
	assert.True(t, legacySidebar["console"]["midjourney"])
	assert.True(t, legacySidebar["console"]["task"])

	var reloadedMissingSidebarUser User
	require.NoError(t, DB.First(&reloadedMissingSidebarUser, missingSidebarUser.Id).Error)
	missingSidebarSetting := reloadedMissingSidebarUser.GetSetting()
	assert.Equal(t, "en", missingSidebarSetting.Language)
	missingSidebar := decodeSidebarModulesForTest(t, missingSidebarSetting.SidebarModules)
	assert.True(t, missingSidebar["chat"]["enabled"])
	assert.True(t, missingSidebar["console"]["midjourney"])
	assert.True(t, missingSidebar["console"]["task"])

	legacySidebar["chat"]["enabled"] = false
	legacySidebar["console"]["task"] = false
	legacySetting.SidebarModules = encodeSidebarModulesForTest(t, legacySidebar)
	reloadedLegacyUser.SetSetting(legacySetting)
	require.NoError(t, reloadedLegacyUser.Update(false))

	require.NoError(t, BackfillSidebarLegacyVisibleDefaults())

	var reopenedLegacyUser User
	require.NoError(t, DB.First(&reopenedLegacyUser, legacyUser.Id).Error)
	reopenedSetting := reopenedLegacyUser.GetSetting()
	reopenedSidebar := decodeSidebarModulesForTest(t, reopenedSetting.SidebarModules)
	assert.False(t, reopenedSidebar["chat"]["enabled"])
	assert.False(t, reopenedSidebar["console"]["task"])
	assert.Equal(
		t,
		sidebarLegacyVisibleDefaultsBackfillStatusCompleted,
		getOptionValueForTest(t, sidebarLegacyVisibleDefaultsBackfillStatusKey),
	)
}
