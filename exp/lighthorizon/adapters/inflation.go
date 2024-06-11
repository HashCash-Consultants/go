package adapters

import (
	"github.com/shantanu-hashcash/go/exp/lightaurora/common"
	"github.com/shantanu-hashcash/go/protocols/aurora/operations"
)

func populateInflationOperation(op *common.Operation, baseOp operations.Base) (operations.Inflation, error) {
	return operations.Inflation{
		Base: baseOp,
	}, nil
}
