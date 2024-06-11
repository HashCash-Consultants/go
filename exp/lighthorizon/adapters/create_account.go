package adapters

import (
	"github.com/shantanu-hashcash/go/amount"
	"github.com/shantanu-hashcash/go/exp/lightaurora/common"
	"github.com/shantanu-hashcash/go/protocols/aurora/operations"
)

func populateCreateAccountOperation(op *common.Operation, baseOp operations.Base) (operations.CreateAccount, error) {
	createAccount := op.Get().Body.MustCreateAccountOp()

	return operations.CreateAccount{
		Base:            baseOp,
		StartingBalance: amount.String(createAccount.StartingBalance),
		Funder:          op.SourceAccount().Address(),
		Account:         createAccount.Destination.Address(),
	}, nil
}
