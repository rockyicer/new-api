package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateUserPermissions_AllowsRootSidebarSettings(t *testing.T) {
	permissions := calculateUserPermissions(common.RoleRootUser)

	sidebarSettings, ok := permissions["sidebar_settings"].(bool)
	require.True(t, ok, "sidebar_settings should be a bool")
	assert.True(t, sidebarSettings, "root user should be allowed to manage sidebar settings")
}
