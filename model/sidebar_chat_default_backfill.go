package model

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	sidebarChatHiddenDefaultBackfillStatusKey       = "SidebarChatHiddenDefaultBackfillStatus"
	sidebarChatHiddenDefaultBackfillStatusCompleted = "completed"
	sidebarDrawingLogHiddenDefaultBackfillStatusKey = "SidebarDrawingLogHiddenDefaultBackfillStatus"
	sidebarDrawingLogHiddenDefaultBackfillCompleted = "completed"
)

type sidebarModulesConfig map[string]map[string]bool

func BackfillSidebarChatHiddenDefault() error {
	status, err := getSidebarChatHiddenDefaultBackfillStatus()
	if err != nil {
		return err
	}
	if status == sidebarChatHiddenDefaultBackfillStatusCompleted {
		return nil
	}

	var users []User
	if err = DB.Find(&users).Error; err != nil {
		return err
	}

	updatedUsers := 0
	for i := range users {
		updated, updateErr := backfillUserSidebarChatHiddenDefault(&users[i])
		if updateErr != nil {
			return updateErr
		}
		if updated {
			updatedUsers++
		}
	}

	if err = UpdateOption(sidebarChatHiddenDefaultBackfillStatusKey, sidebarChatHiddenDefaultBackfillStatusCompleted); err != nil {
		return err
	}

	common.SysLog("backfilled sidebar chat hidden default for existing users: " + strconv.Itoa(updatedUsers))
	return nil
}

func backfillUserSidebarChatHiddenDefault(user *User) (bool, error) {
	currentSetting := user.GetSetting()
	sidebarModules, changed, err := ensureSidebarChatHiddenByDefault(user.Role, currentSetting.SidebarModules)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}

	currentSetting.SidebarModules = sidebarModules
	user.SetSetting(currentSetting)
	if err = DB.Model(&User{}).Where("id = ?", user.Id).Update("setting", user.Setting).Error; err != nil {
		return false, err
	}
	if err = updateSidebarBackfillUserCache(*user); err != nil {
		return false, err
	}
	return true, nil
}

func ensureSidebarChatHiddenByDefault(userRole int, raw string) (string, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return generateDefaultSidebarConfigForRole(userRole), true, nil
	}

	var config sidebarModulesConfig
	if err := common.UnmarshalJsonStr(raw, &config); err != nil {
		common.SysLog("failed to parse sidebar modules during chat default backfill, regenerate default config: " + err.Error())
		return generateDefaultSidebarConfigForRole(userRole), true, nil
	}
	if config == nil {
		config = make(sidebarModulesConfig)
	}

	defaultConfig := buildDefaultSidebarConfigForRole(userRole)
	defaultChatConfig := defaultConfig["chat"]

	if config["chat"] == nil {
		config["chat"] = make(map[string]bool)
	}

	changed := false
	for key, value := range defaultChatConfig {
		if _, exists := config["chat"][key]; !exists {
			config["chat"][key] = value
			changed = true
		}
	}

	if config["chat"]["enabled"] {
		config["chat"]["enabled"] = false
		changed = true
	}

	if !changed {
		return raw, false, nil
	}

	encoded, err := common.Marshal(config)
	if err != nil {
		return "", false, err
	}
	return string(encoded), true, nil
}

func getSidebarChatHiddenDefaultBackfillStatus() (string, error) {
	var option Option
	result := DB.Where("key = ?", sidebarChatHiddenDefaultBackfillStatusKey).Limit(1).Find(&option)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", nil
	}
	return option.Value, nil
}

func buildDefaultSidebarConfigForRole(userRole int) sidebarModulesConfig {
	defaultConfig := sidebarModulesConfig{}

	defaultConfig["chat"] = map[string]bool{
		"enabled":    false,
		"playground": true,
		"chat":       true,
	}

	defaultConfig["console"] = map[string]bool{
		"enabled":    true,
		"detail":     true,
		"token":      true,
		"log":        true,
		"midjourney": false,
		"task":       true,
	}

	defaultConfig["personal"] = map[string]bool{
		"enabled":  true,
		"topup":    true,
		"personal": true,
	}

	if userRole == common.RoleAdminUser {
		defaultConfig["admin"] = map[string]bool{
			"enabled":    true,
			"channel":    true,
			"models":     true,
			"redemption": true,
			"user":       true,
			"setting":    false,
		}
	} else if userRole == common.RoleRootUser {
		defaultConfig["admin"] = map[string]bool{
			"enabled":    true,
			"channel":    true,
			"models":     true,
			"redemption": true,
			"user":       true,
			"setting":    true,
		}
	}

	return defaultConfig
}

func BackfillSidebarDrawingLogHiddenDefault() error {
	status, err := getSidebarDrawingLogHiddenDefaultBackfillStatus()
	if err != nil {
		return err
	}
	if status == sidebarDrawingLogHiddenDefaultBackfillCompleted {
		return nil
	}

	var users []User
	if err = DB.Find(&users).Error; err != nil {
		return err
	}

	updatedUsers := 0
	for i := range users {
		updated, updateErr := backfillUserSidebarDrawingLogHiddenDefault(&users[i])
		if updateErr != nil {
			return updateErr
		}
		if updated {
			updatedUsers++
		}
	}

	if err = UpdateOption(sidebarDrawingLogHiddenDefaultBackfillStatusKey, sidebarDrawingLogHiddenDefaultBackfillCompleted); err != nil {
		return err
	}

	common.SysLog("backfilled sidebar drawing log hidden default for existing users: " + strconv.Itoa(updatedUsers))
	return nil
}

func backfillUserSidebarDrawingLogHiddenDefault(user *User) (bool, error) {
	currentSetting := user.GetSetting()
	sidebarModules, changed, err := ensureSidebarDrawingLogHiddenByDefault(user.Role, currentSetting.SidebarModules)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}

	currentSetting.SidebarModules = sidebarModules
	user.SetSetting(currentSetting)
	if err = DB.Model(&User{}).Where("id = ?", user.Id).Update("setting", user.Setting).Error; err != nil {
		return false, err
	}
	if err = updateSidebarBackfillUserCache(*user); err != nil {
		return false, err
	}
	return true, nil
}

func ensureSidebarDrawingLogHiddenByDefault(userRole int, raw string) (string, bool, error) {
	if strings.TrimSpace(raw) == "" {
		return generateDefaultSidebarConfigForRole(userRole), true, nil
	}

	var config sidebarModulesConfig
	if err := common.UnmarshalJsonStr(raw, &config); err != nil {
		common.SysLog("failed to parse sidebar modules during drawing log default backfill, regenerate default config: " + err.Error())
		return generateDefaultSidebarConfigForRole(userRole), true, nil
	}
	if config == nil {
		config = make(sidebarModulesConfig)
	}

	defaultConfig := buildDefaultSidebarConfigForRole(userRole)
	defaultConsoleConfig := defaultConfig["console"]

	if config["console"] == nil {
		config["console"] = make(map[string]bool)
	}

	changed := false
	for key, value := range defaultConsoleConfig {
		if _, exists := config["console"][key]; !exists {
			config["console"][key] = value
			changed = true
		}
	}

	if config["console"]["midjourney"] {
		config["console"]["midjourney"] = false
		changed = true
	}

	if !changed {
		return raw, false, nil
	}

	encoded, err := common.Marshal(config)
	if err != nil {
		return "", false, err
	}
	return string(encoded), true, nil
}

func getSidebarDrawingLogHiddenDefaultBackfillStatus() (string, error) {
	var option Option
	result := DB.Where("key = ?", sidebarDrawingLogHiddenDefaultBackfillStatusKey).Limit(1).Find(&option)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", nil
	}
	return option.Value, nil
}

func updateSidebarBackfillUserCache(user User) error {
	if !common.RedisEnabled || common.RDB == nil {
		return nil
	}
	return updateUserCache(user)
}
