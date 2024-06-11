package adapters

import (
	"github.com/HashCash-Consultants/go/exp/lightaurora/common"
	"github.com/HashCash-Consultants/go/protocols/aurora/operations"
	"github.com/HashCash-Consultants/go/support/errors"
	"github.com/HashCash-Consultants/go/xdr"
)

func populateClawbackClaimableBalanceOperation(op *common.Operation, baseOp operations.Base) (operations.ClawbackClaimableBalance, error) {
	clawbackClaimableBalance := op.Get().Body.MustClawbackClaimableBalanceOp()

	balanceID, err := xdr.MarshalHex(clawbackClaimableBalance.BalanceId)
	if err != nil {
		return operations.ClawbackClaimableBalance{}, errors.Wrap(err, "invalid balanceId")
	}

	return operations.ClawbackClaimableBalance{
		Base:      baseOp,
		BalanceID: balanceID,
	}, nil
}
