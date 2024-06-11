package actions

import "github.com/shantanu-hashcash/go/services/aurora/internal/corestate"

type CoreStateGetter interface {
	GetCoreState() corestate.State
}
