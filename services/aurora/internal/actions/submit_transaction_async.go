package actions

import (
	"context"
	"net/http"

	"github.com/shantanu-hashcash/go/protocols/aurora"
	proto "github.com/shantanu-hashcash/go/protocols/hcnetcore"
	hProblem "github.com/shantanu-hashcash/go/services/aurora/internal/render/problem"
	"github.com/shantanu-hashcash/go/support/errors"
	"github.com/shantanu-hashcash/go/support/log"
	"github.com/shantanu-hashcash/go/support/render/problem"
)

type coreClient interface {
	SubmitTx(ctx context.Context, rawTx string) (resp *proto.TXResponse, err error)
}

type AsyncSubmitTransactionHandler struct {
	NetworkPassphrase string
	DisableTxSub      bool
	ClientWithMetrics coreClient
	CoreStateGetter
}

func (handler AsyncSubmitTransactionHandler) GetResource(_ HeaderWriter, r *http.Request) (interface{}, error) {
	// TODO: Move the problem responses to a separate file as constants or a function.
	logger := log.Ctx(r.Context())

	if err := validateBodyType(r); err != nil {
		return nil, err
	}

	raw, err := getString(r, "tx")
	if err != nil {
		return nil, err
	}

	if handler.DisableTxSub {
		return nil, &problem.P{
			Type:   "transaction_submission_disabled",
			Title:  "Transaction Submission Disabled",
			Status: http.StatusForbidden,
			Detail: "Transaction submission has been disabled for Aurora. " +
				"To enable it again, remove env variable DISABLE_TX_SUB.",
			Extras: map[string]interface{}{
				"envelope_xdr": raw,
			},
		}
	}

	info, err := extractEnvelopeInfo(raw, handler.NetworkPassphrase)
	if err != nil {
		return nil, &problem.P{
			Type:   "transaction_malformed",
			Title:  "Transaction Malformed",
			Status: http.StatusBadRequest,
			Detail: "Aurora could not decode the transaction envelope in this " +
				"request. A transaction should be an XDR TransactionEnvelope struct " +
				"encoded using base64.  The envelope read from this request is " +
				"echoed in the `extras.envelope_xdr` field of this response for your " +
				"convenience.",
			Extras: map[string]interface{}{
				"envelope_xdr": raw,
				"error":        err,
			},
		}
	}

	coreState := handler.GetCoreState()
	if !coreState.Synced {
		return nil, hProblem.StaleHistory
	}

	resp, err := handler.ClientWithMetrics.SubmitTx(r.Context(), raw)
	if err != nil {
		return nil, &problem.P{
			Type:   "transaction_submission_failed",
			Title:  "Transaction Submission Failed",
			Status: http.StatusInternalServerError,
			Detail: "Could not submit transaction to hcnet-core. " +
				"The `extras.error` field on this response contains further " +
				"details.  Descriptions of each code can be found at: " +
				"https://developers.hcnet.org/api/errors/http-status-codes/aurora-specific/transaction-submission-async/transaction_submission_failed",
			Extras: map[string]interface{}{
				"envelope_xdr": raw,
				"error":        err,
			},
		}
	}

	if resp.IsException() {
		logger.WithField("envelope_xdr", raw).WithError(errors.Errorf(resp.Exception)).Error("Transaction submission exception from hcnet-core")
		return nil, &problem.P{
			Type:   "transaction_submission_exception",
			Title:  "Transaction Submission Exception",
			Status: http.StatusInternalServerError,
			Detail: "Received exception from hcnet-core." +
				"The `extras.error` field on this response contains further " +
				"details.  Descriptions of each code can be found at: " +
				"https://developers.hcnet.org/api/errors/http-status-codes/aurora-specific/transaction-submission-async/transaction_submission_exception",
			Extras: map[string]interface{}{
				"envelope_xdr": raw,
				"error":        resp.Exception,
			},
		}
	}

	switch resp.Status {
	case proto.TXStatusError, proto.TXStatusPending, proto.TXStatusDuplicate, proto.TXStatusTryAgainLater:
		response := aurora.AsyncTransactionSubmissionResponse{
			TxStatus: resp.Status,
			Hash:     info.hash,
		}

		if resp.Status == proto.TXStatusError {
			response.ErrorResultXDR = resp.Error
		}

		return response, nil
	default:
		logger.WithField("envelope_xdr", raw).WithError(errors.Errorf(resp.Error)).Error("Received invalid submission status from hcnet-core")
		return nil, &problem.P{
			Type:   "transaction_submission_invalid_status",
			Title:  "Transaction Submission Invalid Status",
			Status: http.StatusInternalServerError,
			Detail: "Received invalid status from hcnet-core." +
				"The `extras.error` field on this response contains further " +
				"details.  Descriptions of each code can be found at: " +
				"https://developers.hcnet.org/api/errors/http-status-codes/aurora-specific/transaction-submission-async/transaction_submission_invalid_status",
			Extras: map[string]interface{}{
				"envelope_xdr": raw,
				"error":        resp.Error,
			},
		}
	}

}