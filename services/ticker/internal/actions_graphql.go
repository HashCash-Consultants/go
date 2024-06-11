package ticker

import (
	"github.com/HashCash-Consultants/go/services/ticker/internal/gql"
	"github.com/HashCash-Consultants/go/services/ticker/internal/tickerdb"
	hlog "github.com/HashCash-Consultants/go/support/log"
)

func StartGraphQLServer(s *tickerdb.TickerSession, l *hlog.Entry, port string) {
	graphql := gql.New(s, l)

	graphql.Serve(port)
}
