package integration

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/shantanu-hashcash/go/clients/auroraclient"
	"github.com/shantanu-hashcash/go/protocols/aurora/operations"
	"github.com/shantanu-hashcash/go/services/aurora/internal/test/integration"
	"github.com/shantanu-hashcash/go/txnbuild"
)

func TestExtendFootprintTtl(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableSorobanRPC: true,
	})

	// establish which account will be contract owner, and load it's current seq
	sourceAccount, err := itest.Client().AccountDetail(auroraclient.AccountRequest{
		AccountID: itest.Master().Address(),
	})
	require.NoError(t, err)

	installContractOp := assembleInstallContractCodeOp(t, itest.Master().Address(), add_u64_contract)
	preFlightOp, minFee := itest.PreflightHostFunctions(&sourceAccount, *installContractOp)
	tx := itest.MustSubmitOperationsWithFee(&sourceAccount, itest.Master(), minFee+txnbuild.MinBaseFee, &preFlightOp)

	_, err = itest.Client().TransactionDetail(tx.Hash)
	require.NoError(t, err)

	sourceAccount, bumpFootPrint, minFee := itest.PreflightExtendExpiration(
		itest.Master().Address(),
		preFlightOp.Ext.SorobanData.Resources.Footprint.ReadWrite,
		10000,
	)
	tx = itest.MustSubmitOperationsWithFee(&sourceAccount, itest.Master(), minFee+txnbuild.MinBaseFee, &bumpFootPrint)

	ops, err := itest.Client().Operations(auroraclient.OperationRequest{ForTransaction: tx.Hash})
	require.NoError(t, err)
	require.Len(t, ops.Embedded.Records, 1)

	op := ops.Embedded.Records[0].(operations.ExtendFootprintTtl)
	require.Equal(t, uint32(10000), op.ExtendTo)
}
