package adapters

import (
	"github.com/HashCash-Consultants/go/amount"
	"github.com/HashCash-Consultants/go/exp/lightaurora/common"
	"github.com/HashCash-Consultants/go/protocols/aurora"
	"github.com/HashCash-Consultants/go/protocols/aurora/operations"
)

func populateCreateClaimableBalanceOperation(op *common.Operation, baseOp operations.Base) (operations.CreateClaimableBalance, error) {
	createClaimableBalance := op.Get().Body.MustCreateClaimableBalanceOp()

	claimants := make([]aurora.Claimant, len(createClaimableBalance.Claimants))
	for i, claimant := range createClaimableBalance.Claimants {
		claimants[i] = aurora.Claimant{
			Destination: claimant.MustV0().Destination.Address(),
			Predicate:   claimant.MustV0().Predicate,
		}
	}

	return operations.CreateClaimableBalance{
		Base:      baseOp,
		Asset:     createClaimableBalance.Asset.StringCanonical(),
		Amount:    amount.String(createClaimableBalance.Amount),
		Claimants: claimants,
	}, nil
}
