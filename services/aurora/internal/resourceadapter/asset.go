package resourceadapter

import (
	"context"

	protocol "github.com/HashCash-Consultants/go/protocols/aurora"
	"github.com/HashCash-Consultants/go/xdr"
)

func PopulateAsset(ctx context.Context, dest *protocol.Asset, asset xdr.Asset) error {
	return asset.Extract(&dest.Type, &dest.Code, &dest.Issuer)
}
