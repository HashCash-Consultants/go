package adapters

import (
	"github.com/shantanu-hashcash/go/amount"
	"github.com/shantanu-hashcash/go/exp/lightaurora/common"
	"github.com/shantanu-hashcash/go/protocols/aurora/base"
	"github.com/shantanu-hashcash/go/protocols/aurora/operations"
	"github.com/shantanu-hashcash/go/xdr"
)

func populateLiquidityPoolDepositOperation(op *common.Operation, baseOp operations.Base) (operations.LiquidityPoolDeposit, error) {
	liquidityPoolDeposit := op.Get().Body.MustLiquidityPoolDepositOp()

	return operations.LiquidityPoolDeposit{
		Base: baseOp,
		// TODO: some fields missing because derived from meta
		LiquidityPoolID: xdr.Hash(liquidityPoolDeposit.LiquidityPoolId).HexString(),
		ReservesMax: []base.AssetAmount{
			{Amount: amount.String(liquidityPoolDeposit.MaxAmountA)},
			{Amount: amount.String(liquidityPoolDeposit.MaxAmountB)},
		},
		MinPrice: liquidityPoolDeposit.MinPrice.String(),
		MinPriceR: base.Price{
			N: int32(liquidityPoolDeposit.MinPrice.N),
			D: int32(liquidityPoolDeposit.MinPrice.D),
		},
		MaxPrice: liquidityPoolDeposit.MaxPrice.String(),
		MaxPriceR: base.Price{
			N: int32(liquidityPoolDeposit.MaxPrice.N),
			D: int32(liquidityPoolDeposit.MaxPrice.D),
		},
	}, nil
}
