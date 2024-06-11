package history

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/shantanu-hashcash/go/services/aurora/internal/test"
	"github.com/shantanu-hashcash/go/xdr"
)

func TestLiquidityPoolLoader(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetAuroraDB(t, tt.AuroraDB)
	session := tt.AuroraSession()

	var ids []string
	for i := 0; i < 100; i++ {
		poolID := xdr.PoolId{byte(i)}
		id, err := xdr.MarshalHex(poolID)
		tt.Assert.NoError(err)
		ids = append(ids, id)
	}

	loader := NewLiquidityPoolLoader()
	for _, id := range ids {
		future := loader.GetFuture(id)
		_, err := future.Value()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid liquidity pool loader state,`)
		duplicateFuture := loader.GetFuture(id)
		assert.Equal(t, future, duplicateFuture)
	}

	err := loader.Exec(context.Background(), session)
	assert.NoError(t, err)
	assert.Equal(t, LoaderStats{
		Total:    100,
		Inserted: 100,
	}, loader.Stats())
	assert.Panics(t, func() {
		loader.GetFuture("not-present")
	})

	q := &Q{session}
	for _, id := range ids {
		var internalID int64
		internalID, err = loader.GetNow(id)
		assert.NoError(t, err)
		var lp HistoryLiquidityPool
		lp, err = q.LiquidityPoolByID(context.Background(), id)
		assert.NoError(t, err)
		assert.Equal(t, lp.PoolID, id)
		assert.Equal(t, lp.InternalID, internalID)
	}

	_, err = loader.GetNow("not present")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `was not found`)
}
