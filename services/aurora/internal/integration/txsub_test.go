package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/HashCash-Consultants/go/clients/auroraclient"
	"github.com/HashCash-Consultants/go/services/aurora/internal/test/integration"
	"github.com/HashCash-Consultants/go/txnbuild"
)

func TestTxSub(t *testing.T) {
	tt := assert.New(t)

	t.Run("transaction submission is successful when DISABLE_TX_SUB=false", func(t *testing.T) {
		itest := integration.NewTest(t, integration.Config{})
		master := itest.Master()

		op := txnbuild.Payment{
			Destination: master.Address(),
			Amount:      "10",
			Asset:       txnbuild.NativeAsset{},
		}

		txResp, err := itest.SubmitOperations(itest.MasterAccount(), master, &op)
		assert.NoError(t, err)

		var seq int64
		tt.Equal(itest.MasterAccount().GetAccountID(), txResp.Account)
		seq, err = itest.MasterAccount().GetSequenceNumber()
		assert.NoError(t, err)
		tt.Equal(seq, txResp.AccountSequence)
		t.Logf("Done")
	})

	t.Run("transaction submission is not successful when DISABLE_TX_SUB=true", func(t *testing.T) {
		itest := integration.NewTest(t, integration.Config{
			AuroraEnvironment: map[string]string{
				"DISABLE_TX_SUB": "true",
			},
		})
		master := itest.Master()

		op := txnbuild.Payment{
			Destination: master.Address(),
			Amount:      "10",
			Asset:       txnbuild.NativeAsset{},
		}

		_, err := itest.SubmitOperations(itest.MasterAccount(), master, &op)
		assert.Error(t, err)
	})
}

func TestTxSubLimitsBodySize(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		EnableSorobanRPC: true,
		AuroraEnvironment: map[string]string{
			"MAX_HTTP_REQUEST_SIZE": "1800",
		},
	})

	// establish which account will be contract owner, and load it's current seq
	sourceAccount, err := itest.Client().AccountDetail(auroraclient.AccountRequest{
		AccountID: itest.Master().Address(),
	})
	require.NoError(t, err)

	installContractOp := assembleInstallContractCodeOp(t, itest.Master().Address(), "soroban_sac_test.wasm")
	preFlightOp, minFee := itest.PreflightHostFunctions(&sourceAccount, *installContractOp)
	_, err = itest.SubmitOperationsWithFee(&sourceAccount, itest.Master(), minFee+txnbuild.MinBaseFee, &preFlightOp)
	assert.EqualError(
		t, err,
		"aurora error: \"Transaction Malformed\" - check aurora.Error.Problem for more information",
	)

	sourceAccount, err = itest.Client().AccountDetail(auroraclient.AccountRequest{
		AccountID: itest.Master().Address(),
	})
	require.NoError(t, err)

	installContractOp = assembleInstallContractCodeOp(t, itest.Master().Address(), "soroban_add_u64.wasm")
	preFlightOp, minFee = itest.PreflightHostFunctions(&sourceAccount, *installContractOp)
	tx, err := itest.SubmitOperationsWithFee(&sourceAccount, itest.Master(), minFee+txnbuild.MinBaseFee, &preFlightOp)
	require.NoError(t, err)
	require.True(t, tx.Successful)
}
