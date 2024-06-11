package adapters

import (
	"github.com/HashCash-Consultants/go/exp/lightaurora/common"
	"github.com/HashCash-Consultants/go/protocols/aurora/operations"
	"github.com/HashCash-Consultants/go/support/errors"
	"github.com/HashCash-Consultants/go/xdr"
)

func populateClaimClaimableBalanceOperation(op *common.Operation, baseOp operations.Base) (operations.ClaimClaimableBalance, error) {
	claimClaimableBalance := op.Get().Body.MustClaimClaimableBalanceOp()

	balanceID, err := xdr.MarshalHex(claimClaimableBalance.BalanceId)
	if err != nil {
		return operations.ClaimClaimableBalance{}, errors.New("invalid balanceId")
	}

	return operations.ClaimClaimableBalance{
		Base:      baseOp,
		BalanceID: balanceID,
		Claimant:  op.SourceAccount().Address(),
		// TODO
		ClaimantMuxed:   "",
		ClaimantMuxedID: 0,
	}, nil
}
