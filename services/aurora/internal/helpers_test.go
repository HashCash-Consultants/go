package aurora

import (
	"log"
	"time"

	"github.com/shantanu-hashcash/throttled"

	"github.com/shantanu-hashcash/go/network"
	"github.com/shantanu-hashcash/go/services/aurora/internal/test"
	supportLog "github.com/shantanu-hashcash/go/support/log"
)

func NewTestApp(dsn string) *App {
	app, err := NewApp(NewTestConfig(dsn))
	if err != nil {
		log.Fatal("cannot create app", err)
	}
	return app
}

func NewTestConfig(dsn string) Config {
	return Config{
		DatabaseURL: dsn,
		RateQuota: &throttled.RateQuota{
			MaxRate:  throttled.PerHour(1000),
			MaxBurst: 100,
		},
		ConnectionTimeout: 55 * time.Second, // Default
		LogLevel:          supportLog.InfoLevel,
		NetworkPassphrase: network.TestNetworkPassphrase,
	}
}

func NewRequestHelper(app *App) test.RequestHelper {
	return test.NewRequestHelper(app.webServer.Router.Mux)
}
