package actions

import "github.com/HashCash-Consultants/go/services/aurora/internal/corestate"

type CoreStateGetter interface {
	GetCoreState() corestate.State
}
