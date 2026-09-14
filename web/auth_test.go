package web

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"lousentry/store"
	_ "modernc.org/sqlite"
)

func TestAuthStoreUsers(t *testing.T) {
	meta := testStore(t)
	auth, err := NewAuthStore(meta, "admin", "admin123")
	require.NoError(t, err)

	token, err := auth.Login("admin", "admin123", "127.0.0.1")
	require.NoError(t, err)
	user, ok := auth.Verify(token)
	require.True(t, ok)
	require.Equal(t, "admin", user)

	require.NoError(t, auth.AddUser("ops", "ops123456", store.RoleReadonly))
	require.Equal(t, 2, len(auth.ListUsers()))
	require.Equal(t, store.RoleReadonly, auth.UserOf("ops").Role)
	require.False(t, auth.IsAdmin("ops"))
	_, err = auth.Login("ops", "ops123456", "127.0.0.1")
	require.NoError(t, err)

	require.Error(t, auth.DeleteUser("admin", "admin"))
	require.NoError(t, auth.DeleteUser("admin", "ops"))
	require.Equal(t, 1, len(auth.ListUsers()))
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "meta.sqlite3"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	meta, err := store.New(db, "sqlite3")
	require.NoError(t, err)
	return meta
}
