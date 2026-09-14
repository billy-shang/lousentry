package ctrl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSourceFromURL(t *testing.T) {
	assert := require.New(t)
	assert.Equal("avd", SourceFromURL("https://avd.aliyun.com/high-risk/list").ID)
	assert.Equal("ti", SourceFromURL("https://ti.qianxin.com/vulnerability/detail/1").ID)
	assert.Equal("chaitin", SourceFromURL("https://stack.chaitin.com/vuldb/detail/1").ID)
	assert.Equal("ti", NormalizeSource("nox"))
}
