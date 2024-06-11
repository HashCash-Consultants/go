package ticker

import (
	"github.com/shantanu-hashcash/go/services/ticker/internal/gql"
	"github.com/shantanu-hashcash/go/services/ticker/internal/tickerdb"
	hlog "github.com/shantanu-hashcash/go/support/log"
)

func StartGraphQLServer(s *tickerdb.TickerSession, l *hlog.Entry, port string) {
	graphql := gql.New(s, l)

	graphql.Serve(port)
}
