package adapters

import (
	"github.com/HashCash-Consultants/go/amount"
	"github.com/HashCash-Consultants/go/exp/lightaurora/common"
	"github.com/HashCash-Consultants/go/protocols/aurora/base"
	"github.com/HashCash-Consultants/go/protocols/aurora/operations"
	"github.com/HashCash-Consultants/go/xdr"
)

func populateLiquidityPoolWithdrawOperation(op *common.Operation, baseOp operations.Base) (operations.LiquidityPoolWithdraw, error) {
	liquidityPoolWithdraw := op.Get().Body.MustLiquidityPoolWithdrawOp()

	return operations.LiquidityPoolWithdraw{
		Base: baseOp,
		// TODO: some fields missing because derived from meta
		LiquidityPoolID: xdr.Hash(liquidityPoolWithdraw.LiquidityPoolId).HexString(),
		ReservesMin: []base.AssetAmount{
			{Amount: amount.String(liquidityPoolWithdraw.MinAmountA)},
			{Amount: amount.String(liquidityPoolWithdraw.MinAmountB)},
		},
		Shares: amount.String(liquidityPoolWithdraw.Amount),
	}, nil
}
