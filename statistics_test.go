package roundrobin_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hqgoroundrobin "github.com/hueristiq/hq-lib-roundrobin-go"
)

func TestStatisticsOf(t *testing.T) {
	t.Parallel()

	rr := hqgoroundrobin.New("a", "b")

	value, ok := rr.NextOK()
	require.True(t, ok)
	require.Equal(t, "a", value)

	aStats, found := rr.StatisticsOf("a")
	require.True(t, found)
	require.NotNil(t, aStats)
	assert.Equal(t, int64(1), aStats.ServesCount())

	aStats.ResetServesCount()
	assert.Equal(t, int64(0), aStats.ServesCount())

	missingStats, found := rr.StatisticsOf("missing")
	assert.False(t, found)
	assert.Nil(t, missingStats)

	require.True(t, rr.Remove("a"))

	// The pointer stays valid after removal, but the counter no longer moves.
	assert.Equal(t, int64(0), aStats.ServesCount())
}
