package adapters

import (
	"github.com/shantanu-hashcash/go/exp/lightaurora/common"
	"github.com/shantanu-hashcash/go/protocols/aurora/base"
	"github.com/shantanu-hashcash/go/protocols/aurora/operations"
	"github.com/shantanu-hashcash/go/support/errors"
	"github.com/shantanu-hashcash/go/xdr"
)

func populateSetTrustLineFlagsOperation(op *common.Operation, baseOp operations.Base) (operations.SetTrustLineFlags, error) {
	setTrustLineFlags := op.Get().Body.MustSetTrustLineFlagsOp()

	var (
		assetType string
		code      string
		issuer    string
	)
	err := setTrustLineFlags.Asset.Extract(&assetType, &code, &issuer)
	if err != nil {
		return operations.SetTrustLineFlags{}, errors.Wrap(err, "xdr.Asset.Extract error")
	}

	var (
		setFlags  []int
		setFlagsS []string

		clearFlags  []int
		clearFlagsS []string
	)

	if setTrustLineFlags.SetFlags > 0 {
		f := xdr.TrustLineFlags(setTrustLineFlags.SetFlags)

		if f.IsAuthorized() {
			setFlags = append(setFlags, int(xdr.TrustLineFlagsAuthorizedFlag))
			setFlagsS = append(setFlagsS, "authorized")
		}

		if f.IsAuthorizedToMaintainLiabilitiesFlag() {
			setFlags = append(setFlags, int(xdr.TrustLineFlagsAuthorizedToMaintainLiabilitiesFlag))
			setFlagsS = append(setFlagsS, "authorized_to_maintain_liabilites")
		}

		if f.IsClawbackEnabledFlag() {
			setFlags = append(setFlags, int(xdr.TrustLineFlagsTrustlineClawbackEnabledFlag))
			setFlagsS = append(setFlagsS, "clawback_enabled")
		}
	}

	if setTrustLineFlags.ClearFlags > 0 {
		f := xdr.TrustLineFlags(setTrustLineFlags.ClearFlags)

		if f.IsAuthorized() {
			clearFlags = append(clearFlags, int(xdr.TrustLineFlagsAuthorizedFlag))
			clearFlagsS = append(clearFlagsS, "authorized")
		}

		if f.IsAuthorizedToMaintainLiabilitiesFlag() {
			clearFlags = append(clearFlags, int(xdr.TrustLineFlagsAuthorizedToMaintainLiabilitiesFlag))
			clearFlagsS = append(clearFlagsS, "authorized_to_maintain_liabilites")
		}

		if f.IsClawbackEnabledFlag() {
			clearFlags = append(clearFlags, int(xdr.TrustLineFlagsTrustlineClawbackEnabledFlag))
			clearFlagsS = append(clearFlagsS, "clawback_enabled")
		}
	}

	return operations.SetTrustLineFlags{
		Base: baseOp,
		Asset: base.Asset{
			Type:   assetType,
			Code:   code,
			Issuer: issuer,
		},
		Trustor:     setTrustLineFlags.Trustor.Address(),
		SetFlags:    setFlags,
		SetFlagsS:   setFlagsS,
		ClearFlags:  clearFlags,
		ClearFlagsS: clearFlagsS,
	}, nil
}
