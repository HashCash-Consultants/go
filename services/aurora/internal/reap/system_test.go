package reap

import (
	"testing"

	"github.com/HashCash-Consultants/go/services/aurora/internal/ledger"
	"github.com/HashCash-Consultants/go/services/aurora/internal/test"
)

func TestDeleteUnretainedHistory(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	ledgerState := &ledger.State{}
	ledgerState.SetStatus(tt.Scenario("kahuna"))

	db := tt.AuroraSession()

	sys := New(0, 50, db, ledgerState)

	// Disable sleeps for this.
	sleep = 0

	var (
		prev int
		cur  int
	)
	err := db.GetRaw(tt.Ctx, &prev, `SELECT COUNT(*) FROM history_ledgers`)
	tt.Require.NoError(err)

	err = sys.DeleteUnretainedHistory(tt.Ctx)
	if tt.Assert.NoError(err) {
		err = db.GetRaw(tt.Ctx, &cur, `SELECT COUNT(*) FROM history_ledgers`)
		tt.Require.NoError(err)
		tt.Assert.Equal(prev, cur, "Ledgers deleted when RetentionCount == 0")
	}

	ledgerState.SetStatus(tt.LoadLedgerStatus())
	sys.RetentionCount = 10
	err = sys.DeleteUnretainedHistory(tt.Ctx)
	if tt.Assert.NoError(err) {
		err = db.GetRaw(tt.Ctx, &cur, `SELECT COUNT(*) FROM history_ledgers`)
		tt.Require.NoError(err)
		tt.Assert.Equal(10, cur)
	}

	ledgerState.SetStatus(tt.LoadLedgerStatus())
	sys.RetentionCount = 1
	err = sys.DeleteUnretainedHistory(tt.Ctx)
	if tt.Assert.NoError(err) {
		err = db.GetRaw(tt.Ctx, &cur, `SELECT COUNT(*) FROM history_ledgers`)
		tt.Require.NoError(err)
		tt.Assert.Equal(1, cur)
	}
}
