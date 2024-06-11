package adapters

import (
	"encoding/base64"

	"github.com/shantanu-hashcash/go/exp/lightaurora/common"
	"github.com/shantanu-hashcash/go/protocols/aurora/operations"
)

func populateManageDataOperation(op *common.Operation, baseOp operations.Base) (operations.ManageData, error) {
	manageData := op.Get().Body.MustManageDataOp()

	dataValue := ""
	if manageData.DataValue != nil {
		dataValue = base64.StdEncoding.EncodeToString(*manageData.DataValue)
	}

	return operations.ManageData{
		Base:  baseOp,
		Name:  string(manageData.DataName),
		Value: dataValue,
	}, nil
}
