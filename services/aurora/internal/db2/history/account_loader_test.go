package history

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/shantanu-hashcash/go/keypair"
	"github.com/shantanu-hashcash/go/services/aurora/internal/test"
)

func TestAccountLoader(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetAuroraDB(t, tt.AuroraDB)
	session := tt.AuroraSession()

	var addresses []string
	for i := 0; i < 100; i++ {
		addresses = append(addresses, keypair.MustRandom().Address())
	}

	loader := NewAccountLoader()
	for _, address := range addresses {
		future := loader.GetFuture(address)
		_, err := future.Value()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid account loader state,`)
		duplicateFuture := loader.GetFuture(address)
		assert.Equal(t, future, duplicateFuture)
	}

	err := loader.Exec(context.Background(), session)
	assert.NoError(t, err)
	assert.Equal(t, LoaderStats{
		Total:    100,
		Inserted: 100,
	}, loader.Stats())
	assert.Panics(t, func() {
		loader.GetFuture(keypair.MustRandom().Address())
	})

	q := &Q{session}
	for _, address := range addresses {
		var internalId int64
		internalId, err = loader.GetNow(address)
		assert.NoError(t, err)
		var account Account
		assert.NoError(t, q.AccountByAddress(context.Background(), &account, address))
		assert.Equal(t, account.ID, internalId)
		assert.Equal(t, account.Address, address)
	}

	_, err = loader.GetNow("not present")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `was not found`)
}
