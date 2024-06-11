package adapters

import (
	"github.com/HashCash-Consultants/go/exp/lightaurora/common"
	"github.com/HashCash-Consultants/go/protocols/aurora/operations"
)

func populateInflationOperation(op *common.Operation, baseOp operations.Base) (operations.Inflation, error) {
	return operations.Inflation{
		Base: baseOp,
	}, nil
}
