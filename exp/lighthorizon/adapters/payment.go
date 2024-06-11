package adapters

import (
	"github.com/HashCash-Consultants/go/amount"
	"github.com/HashCash-Consultants/go/exp/lightaurora/common"
	"github.com/HashCash-Consultants/go/protocols/aurora/base"
	"github.com/HashCash-Consultants/go/protocols/aurora/operations"
	"github.com/HashCash-Consultants/go/support/errors"
)

func populatePaymentOperation(op *common.Operation, baseOp operations.Base) (operations.Payment, error) {
	payment := op.Get().Body.MustPaymentOp()

	var (
		assetType string
		code      string
		issuer    string
	)
	err := payment.Asset.Extract(&assetType, &code, &issuer)
	if err != nil {
		return operations.Payment{}, errors.Wrap(err, "xdr.Asset.Extract error")
	}

	return operations.Payment{
		Base: baseOp,
		To:   payment.Destination.Address(),
		From: op.SourceAccount().Address(),
		Asset: base.Asset{
			Type:   assetType,
			Code:   code,
			Issuer: issuer,
		},
		Amount: amount.StringFromInt64(int64(payment.Amount)),
	}, nil
}
