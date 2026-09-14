package store

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestSaveAndLoadPushChannels(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "meta.sqlite3"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	meta, err := New(db, "sqlite3")
	require.NoError(t, err)

	_, ok, err := meta.LoadPushChannels()
	require.NoError(t, err)
	require.False(t, ok)

	require.NoError(t, meta.SavePushChannels([]PushChannel{
		{Channel: "dingding", Enabled: true, AccessToken: "dt", SignSecret: "ds"},
		{Channel: "lark", Enabled: true, AccessToken: "https://example.com/hook", SignSecret: "ls"},
		{Channel: "wechatwork", Enabled: false, ExtraKey: "wk"},
	}))

	items, ok, err := meta.LoadPushChannels()
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, items, 3)

	byChannel := map[string]PushChannel{}
	for _, item := range items {
		byChannel[item.Channel] = item
	}
	require.True(t, byChannel["dingding"].Enabled)
	require.Equal(t, "dt", byChannel["dingding"].AccessToken)
	require.Equal(t, "https://example.com/hook", byChannel["lark"].AccessToken)
	require.False(t, byChannel["wechatwork"].Enabled)
	require.Equal(t, "wk", byChannel["wechatwork"].ExtraKey)
}
