package history

import (
	"testing"

	"github.com/HashCash-Consultants/go/services/aurora/internal/test"
	"github.com/stretchr/testify/assert"
)

func TestAssetFilterConfig(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetAuroraDB(t, tt.AuroraDB)
	q := &Q{tt.AuroraSession()}

	fc1Result, err := q.GetAssetFilterConfig(tt.Ctx)
	assert.NoError(t, err)
	tt.Assert.Equal(fc1Result.Enabled, false)
	tt.Assert.Len(fc1Result.Whitelist, 0)

	fc1Result.Enabled = true
	fc1Result.Whitelist = append(fc1Result.Whitelist, "1", "2")
	fc1Result, err = q.UpdateAssetFilterConfig(tt.Ctx, fc1Result)
	assert.NoError(t, err)
	tt.Assert.Equal(fc1Result.Enabled, true)
	tt.Assert.ElementsMatch(fc1Result.Whitelist, []string{"1", "2"})
}

func TestAccountFilterConfig(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetAuroraDB(t, tt.AuroraDB)
	q := &Q{tt.AuroraSession()}

	fc1Result, err := q.GetAccountFilterConfig(tt.Ctx)
	assert.NoError(t, err)
	tt.Assert.Equal(fc1Result.Enabled, false)
	tt.Assert.Len(fc1Result.Whitelist, 0)

	fc1Result.Enabled = true
	fc1Result.Whitelist = append(fc1Result.Whitelist, "1", "2")
	fc1Result, err = q.UpdateAccountFilterConfig(tt.Ctx, fc1Result)
	tt.Assert.Equal(fc1Result.Enabled, true)
	tt.Assert.ElementsMatch(fc1Result.Whitelist, []string{"1", "2"})
}
