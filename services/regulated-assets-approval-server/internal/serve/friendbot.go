package serve

import (
	"context"
	"fmt"
	"net/http"

	"github.com/HashCash-Consultants/go/clients/auroraclient"
	"github.com/HashCash-Consultants/go/keypair"
	"github.com/HashCash-Consultants/go/services/regulated-assets-approval-server/internal/serve/httperror"
	"github.com/HashCash-Consultants/go/strkey"
	"github.com/HashCash-Consultants/go/support/errors"
	"github.com/HashCash-Consultants/go/support/http/httpdecode"
	"github.com/HashCash-Consultants/go/support/log"
	"github.com/HashCash-Consultants/go/support/render/httpjson"
	"github.com/HashCash-Consultants/go/txnbuild"
)

type friendbotHandler struct {
	issuerAccountSecret string
	assetCode           string
	auroraClient       auroraclient.ClientInterface
	auroraURL          string
	networkPassphrase   string
	paymentAmount       int
}

func (h friendbotHandler) validate() error {
	if h.issuerAccountSecret == "" {
		return errors.New("issuer secret cannot be empty")
	}

	if !strkey.IsValidEd25519SecretSeed(h.issuerAccountSecret) {
		return errors.Errorf("the provided string %q is not a valid Hcnet account seed", h.issuerAccountSecret)
	}

	if h.assetCode == "" {
		return errors.New("asset code cannot be empty")
	}

	if h.auroraClient == nil {
		return errors.New("aurora client cannot be nil")
	}

	if h.auroraURL == "" {
		return errors.New("aurora url cannot be empty")
	}

	if h.networkPassphrase == "" {
		return errors.New("network passphrase cannot be empty")
	}

	if h.paymentAmount == 0 {
		return errors.New("payment amount must be greater than zero")
	}

	return nil
}

type friendbotRequest struct {
	Address string `query:"addr"`
}

func (h friendbotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	in := friendbotRequest{}
	err := httpdecode.Decode(r, &in)
	if err != nil {
		log.Ctx(ctx).Error(errors.Wrap(err, "decoding input parameters"))
		httpErr := httperror.NewHTTPError(http.StatusBadRequest, "Invalid input parameters")
		httpErr.Render(w)
		return
	}

	err = h.topUpAccountWithRegulatedAsset(ctx, in)
	if err != nil {
		httpErr, ok := err.(*httperror.Error)
		if !ok {
			httpErr = httperror.InternalServer
		}
		httpErr.Render(w)
		return
	}

	httpjson.Render(w, httpjson.DefaultResponse, httpjson.JSON)
}

func (h friendbotHandler) topUpAccountWithRegulatedAsset(ctx context.Context, in friendbotRequest) error {
	err := h.validate()
	if err != nil {
		err = errors.Wrap(err, "validating friendbotHandler")
		log.Ctx(ctx).Error(err)
		return err
	}

	if in.Address == "" {
		return httperror.NewHTTPError(http.StatusBadRequest, `Missing query paramater "addr".`)
	}

	if !strkey.IsValidEd25519PublicKey(in.Address) {
		return httperror.NewHTTPError(http.StatusBadRequest, `"addr" is not a valid Hcnet address.`)
	}

	account, err := h.auroraClient.AccountDetail(auroraclient.AccountRequest{AccountID: in.Address})
	if err != nil {
		log.Ctx(ctx).Error(errors.Wrapf(err, "getting detail for account %s", in.Address))
		return httperror.NewHTTPError(http.StatusBadRequest, `Please make sure the provided account address already exists in the network.`)
	}

	kp, err := keypair.ParseFull(h.issuerAccountSecret)
	if err != nil {
		err = errors.Wrap(err, "parsing secret")
		log.Ctx(ctx).Error(err)
		return err
	}

	asset := txnbuild.CreditAsset{
		Code:   h.assetCode,
		Issuer: kp.Address(),
	}

	var accountHasTrustline bool
	for _, b := range account.Balances {
		if b.Asset.Code == asset.Code && b.Asset.Issuer == asset.Issuer {
			accountHasTrustline = true
			break
		}
	}
	if !accountHasTrustline {
		return httperror.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Account with address %s doesn't have a trustline for %s:%s", in.Address, asset.Code, asset.Issuer))
	}

	issuerAcc, err := h.auroraClient.AccountDetail(auroraclient.AccountRequest{AccountID: kp.Address()})
	if err != nil {
		log.Ctx(ctx).Error(errors.Wrapf(err, "getting detail for issuer account %s", kp.Address()))
		return httperror.InternalServer
	}

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &issuerAcc,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{
			&txnbuild.AllowTrust{
				Trustor:   in.Address,
				Type:      asset,
				Authorize: true,
			},
			&txnbuild.Payment{
				Destination: in.Address,
				Amount:      fmt.Sprintf("%d", h.paymentAmount),
				Asset:       asset,
			},
			&txnbuild.AllowTrust{
				Trustor:   in.Address,
				Type:      asset,
				Authorize: false,
			},
		},
		BaseFee:       300,
		Preconditions: txnbuild.Preconditions{TimeBounds: txnbuild.NewTimeout(300)},
	})
	if err != nil {
		err = errors.Wrap(err, "building transaction")
		log.Ctx(ctx).Error(err)
		return err
	}

	tx, err = tx.Sign(h.networkPassphrase, kp)
	if err != nil {
		err = errors.Wrap(err, "signing transaction")
		log.Ctx(ctx).Error(err)
		return err
	}

	_, err = h.auroraClient.SubmitTransaction(tx)
	if err != nil {
		err = httperror.ParseAuroraError(err)
		log.Ctx(ctx).Error(err)
		return err
	}

	return nil
}
