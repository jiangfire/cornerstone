package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPluginService_RestrictsOwnershipAndTableAccess(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewPluginService(db)

	owner := createResourceUser(t, db, "plugin_owner")
	outsider := createResourceUser(t, db, "plugin_outsider")
	viewer := createResourceUser(t, db, "plugin_viewer")
	editor := createResourceUser(t, db, "plugin_editor")

	database := createResourceDatabase(t, db, owner.ID, "PluginPermissionDB")
	grantResourceDatabaseAccess(t, db, database.ID, viewer.ID, "viewer")
	grantResourceDatabaseAccess(t, db, database.ID, editor.ID, "editor")
	table := createResourceTable(t, db, database.ID, "Orders")

	ownedPlugin, err := service.CreatePlugin(CreatePluginRequest{
		Name:      "sync-owner",
		Language:  "bash",
		EntryFile: "main.sh",
		Timeout:   30,
	}, owner.ID)
	require.NoError(t, err)

	viewerPlugin, err := service.CreatePlugin(CreatePluginRequest{
		Name:      "sync-viewer",
		Language:  "bash",
		EntryFile: "viewer.sh",
		Timeout:   30,
	}, viewer.ID)
	require.NoError(t, err)

	editorPlugin, err := service.CreatePlugin(CreatePluginRequest{
		Name:      "sync-editor",
		Language:  "bash",
		EntryFile: "editor.sh",
		Timeout:   30,
	}, editor.ID)
	require.NoError(t, err)

	_, err = service.GetPlugin(ownedPlugin.ID, outsider.ID)
	require.Error(t, err)

	err = service.UpdatePlugin(ownedPlugin.ID, UpdatePluginRequest{
		Name:        "hijack",
		Description: "",
		Timeout:     10,
	}, outsider.ID)
	require.Error(t, err)

	err = service.DeletePlugin(ownedPlugin.ID, outsider.ID)
	require.Error(t, err)

	_, err = service.ListBindings(ownedPlugin.ID, outsider.ID)
	require.Error(t, err)

	err = service.BindPlugin(viewerPlugin.ID, table.ID, "manual", viewer.ID)
	require.Error(t, err)

	err = service.BindPlugin(editorPlugin.ID, table.ID, "manual", editor.ID)
	require.NoError(t, err)

	bindings, err := service.ListBindings(editorPlugin.ID, editor.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 1)
	require.Equal(t, table.ID, bindings[0].TableID)

	err = service.UnbindPlugin(editorPlugin.ID, table.ID, outsider.ID)
	require.Error(t, err)
}

func TestPluginService_ExecutePluginRejectsUnsafeEntryPathWithoutCreatingExecution(t *testing.T) {
	db := setupResourceTestDB(t)
	service := NewPluginService(db)

	owner := createResourceUser(t, db, "plugin_owner_unsafe_entry")
	database := createResourceDatabase(t, db, owner.ID, "PluginUnsafeEntryDB")
	table := createResourceTable(t, db, database.ID, "Orders")

	plugin, err := service.CreatePlugin(CreatePluginRequest{
		Name:      "unsafe-entry",
		Language:  "bash",
		EntryFile: "..\\escape.sh",
		Timeout:   30,
	}, owner.ID)
	require.NoError(t, err)

	require.NoError(t, service.BindPlugin(plugin.ID, table.ID, "manual", owner.ID))

	_, err = service.ExecutePlugin(plugin.ID, owner.ID, ExecutePluginRequest{
		TableID: table.ID,
		Trigger: "manual",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "路径非法")

	executions, err := service.ListExecutions(plugin.ID, owner.ID, 10)
	require.NoError(t, err)
	require.Len(t, executions, 0)
}

func TestResolveScriptPathRejectsUnsafeEntryFileVariants(t *testing.T) {
	testCases := []struct {
		name      string
		workDir   string
		entryFile string
		wantErr   string
	}{
		{
			name:      "empty",
			workDir:   "./plugins",
			entryFile: "",
			wantErr:   "不能为空",
		},
		{
			name:      "traversal",
			workDir:   "./plugins",
			entryFile: "../escape.sh",
			wantErr:   "路径非法",
		},
		{
			name:      "absolute",
			workDir:   "./plugins",
			entryFile: "C:\\temp\\escape.sh",
			wantErr:   "绝对路径",
		},
		{
			name:      "drive relative",
			workDir:   "./plugins",
			entryFile: "C:temp\\escape.sh",
			wantErr:   "绝对路径",
		},
		{
			name:      "unc",
			workDir:   "./plugins",
			entryFile: "\\\\server\\share\\escape.sh",
			wantErr:   "绝对路径",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveScriptPath(tc.workDir, tc.entryFile)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
