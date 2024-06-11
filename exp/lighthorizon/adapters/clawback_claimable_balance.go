package adapters

import (
	"github.com/shantanu-hashcash/go/exp/lightaurora/common"
	"github.com/shantanu-hashcash/go/protocols/aurora/operations"
	"github.com/shantanu-hashcash/go/support/errors"
	"github.com/shantanu-hashcash/go/xdr"
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
