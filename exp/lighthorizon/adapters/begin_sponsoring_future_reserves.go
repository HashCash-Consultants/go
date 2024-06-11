package adapters

import (
	"github.com/HashCash-Consultants/go/exp/lightaurora/common"
	"github.com/HashCash-Consultants/go/protocols/aurora/operations"
)

func populateBeginSponsoringFutureReservesOperation(op *common.Operation, baseOp operations.Base) (operations.BeginSponsoringFutureReserves, error) {
	beginSponsoringFutureReserves := op.Get().Body.MustBeginSponsoringFutureReservesOp()

	return operations.BeginSponsoringFutureReserves{
		Base:        baseOp,
		SponsoredID: beginSponsoringFutureReserves.SponsoredId.Address(),
	}, nil
}
