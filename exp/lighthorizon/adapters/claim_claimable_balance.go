package adapters

import (
	"github.com/shantanu-hashcash/go/exp/lightaurora/common"
	"github.com/shantanu-hashcash/go/protocols/aurora/operations"
	"github.com/shantanu-hashcash/go/support/errors"
	"github.com/shantanu-hashcash/go/xdr"
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
