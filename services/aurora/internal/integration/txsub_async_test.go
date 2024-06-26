package integration

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/HashCash-Consultants/go/clients/auroraclient"
	"github.com/HashCash-Consultants/go/protocols/aurora"
	"github.com/HashCash-Consultants/go/services/aurora/internal/test/integration"
	"github.com/HashCash-Consultants/go/support/errors"
	"github.com/HashCash-Consultants/go/txnbuild"
)

func getTransaction(client *auroraclient.Client, hash string) error {
	for i := 0; i < 60; i++ {
		_, err := client.TransactionDetail(hash)
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		return nil
	}
	return errors.New("transaction not found")
}

func TestAsyncTxSub_SuccessfulSubmission(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{})
	master := itest.Master()
	masterAccount := itest.MasterAccount()

	txParams := txnbuild.TransactionParams{
		BaseFee:              txnbuild.MinBaseFee,
		SourceAccount:        masterAccount,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{
			&txnbuild.Payment{
				Destination: master.Address(),
				Amount:      "10",
				Asset:       txnbuild.NativeAsset{},
			},
		},
		Preconditions: txnbuild.Preconditions{
			TimeBounds:   txnbuild.NewInfiniteTimeout(),
			LedgerBounds: &txnbuild.LedgerBounds{MinLedger: 0, MaxLedger: 100},
		},
	}

	txResp, err := itest.AsyncSubmitTransaction(master, txParams)
	assert.NoError(t, err)
	assert.Equal(t, txResp, aurora.AsyncTransactionSubmissionResponse{
		TxStatus: "PENDING",
		Hash:     "6cbb7f714bd08cea7c30cab7818a35c510cbbfc0a6aa06172a1e94146ecf0165",
	})

	err = getTransaction(itest.Client(), txResp.Hash)
	assert.NoError(t, err)
}

func TestAsyncTxSub_SubmissionError(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{})
	master := itest.Master()
	masterAccount := itest.MasterAccount()

	txParams := txnbuild.TransactionParams{
		BaseFee:              txnbuild.MinBaseFee,
		SourceAccount:        masterAccount,
		IncrementSequenceNum: false,
		Operations: []txnbuild.Operation{
			&txnbuild.Payment{
				Destination: master.Address(),
				Amount:      "10",
				Asset:       txnbuild.NativeAsset{},
			},
		},
		Preconditions: txnbuild.Preconditions{
			TimeBounds:   txnbuild.NewInfiniteTimeout(),
			LedgerBounds: &txnbuild.LedgerBounds{MinLedger: 0, MaxLedger: 100},
		},
	}

	txResp, err := itest.AsyncSubmitTransaction(master, txParams)
	assert.NoError(t, err)
	assert.Equal(t, txResp, aurora.AsyncTransactionSubmissionResponse{
		ErrorResultXDR: "AAAAAAAAAGT////7AAAAAA==",
		TxStatus:       "ERROR",
		Hash:           "0684df00f20efd5876f1b8d17bc6d3a68d8b85c06bb41e448815ecaa6307a251",
	})
}

func TestAsyncTxSub_SubmissionTryAgainLater(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{})
	master := itest.Master()
	masterAccount := itest.MasterAccount()

	txParams := txnbuild.TransactionParams{
		BaseFee:              txnbuild.MinBaseFee,
		SourceAccount:        masterAccount,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{
			&txnbuild.Payment{
				Destination: master.Address(),
				Amount:      "10",
				Asset:       txnbuild.NativeAsset{},
			},
		},
		Preconditions: txnbuild.Preconditions{
			TimeBounds:   txnbuild.NewInfiniteTimeout(),
			LedgerBounds: &txnbuild.LedgerBounds{MinLedger: 0, MaxLedger: 100},
		},
	}

	txResp, err := itest.AsyncSubmitTransaction(master, txParams)
	assert.NoError(t, err)
	assert.Equal(t, txResp, aurora.AsyncTransactionSubmissionResponse{
		ErrorResultXDR: "",
		TxStatus:       "PENDING",
		Hash:           "6cbb7f714bd08cea7c30cab7818a35c510cbbfc0a6aa06172a1e94146ecf0165",
	})

	txResp, err = itest.AsyncSubmitTransaction(master, txParams)
	assert.NoError(t, err)
	assert.Equal(t, txResp, aurora.AsyncTransactionSubmissionResponse{
		ErrorResultXDR: "",
		TxStatus:       "TRY_AGAIN_LATER",
		Hash:           "d5eb72a4c1832b89965850fff0bd9bba4b6ca102e7c89099dcaba5e7d7d2e049",
	})
}

func TestAsyncTxSub_GetOpenAPISpecResponse(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{})
	res, err := http.Get(itest.AsyncTxSubOpenAPISpecURL())
	assert.NoError(t, err)
	assert.Equal(t, res.StatusCode, 200)

	bytes, err := io.ReadAll(res.Body)
	res.Body.Close()
	assert.NoError(t, err)

	openAPISpec := string(bytes)
	assert.Contains(t, openAPISpec, "openapi: 3.0.0")
}
