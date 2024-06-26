// Package reap contains the history reaping subsystem for aurora.  This system
// is designed to remove data from the history database such that it does not
// grow indefinitely.  The system can be configured with a number of ledgers to
// maintain at a minimum.
package reap

import (
	"context"

	"github.com/HashCash-Consultants/go/services/aurora/internal/db2/history"
	"github.com/HashCash-Consultants/go/services/aurora/internal/ledger"
	"github.com/HashCash-Consultants/go/support/db"
)

// System represents the history reaping subsystem of aurora.
type System struct {
	HistoryQ       *history.Q
	RetentionCount uint
	RetentionBatch uint

	ledgerState *ledger.State
	ctx         context.Context
	cancel      context.CancelFunc
}

// New initializes the reaper, causing it to begin polling the hcnet-core
// database for now ledgers and ingesting data into the aurora database.
func New(retention, retentionBatchSize uint, dbSession db.SessionInterface, ledgerState *ledger.State) *System {
	ctx, cancel := context.WithCancel(context.Background())

	r := &System{
		HistoryQ:       &history.Q{dbSession.Clone()},
		RetentionCount: retention,
		RetentionBatch: retentionBatchSize,
		ledgerState:    ledgerState,
		ctx:            ctx,
		cancel:         cancel,
	}

	return r
}
