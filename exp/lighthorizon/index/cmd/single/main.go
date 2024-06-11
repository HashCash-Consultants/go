package main

import (
	"context"
	"flag"
	"runtime"
	"strings"

	"github.com/HashCash-Consultants/go/exp/lightaurora/index"
	"github.com/HashCash-Consultants/go/historyarchive"
	"github.com/HashCash-Consultants/go/network"
	"github.com/HashCash-Consultants/go/support/log"
)

func main() {
	sourceUrl := flag.String("source", "gcs://aurora-archive-poc", "history archive url to read txmeta files")
	targetUrl := flag.String("target", "file://indexes", "where to write indexes")
	networkPassphrase := flag.String("network-passphrase", network.TestNetworkPassphrase, "network passphrase")
	start := flag.Int("start", 2, "ledger to start at (inclusive, default: 2, the earliest)")
	end := flag.Int("end", 0, "ledger to end at (inclusive, default: 0, the latest as of start time)")
	modules := flag.String("modules", "accounts,transactions", "comma-separated list of modules to index (default: all)")
	watch := flag.Bool("watch", false, "whether to watch the `source` for new "+
		"txmeta files and index them (default: false). "+
		"note: `-watch` implies a continuous `-end 0` to get to the latest ledger in txmeta files")
	workerCount := flag.Int("workers", runtime.NumCPU()-1, "number of workers (default: # of CPUs - 1)")

	flag.Parse()
	log.SetLevel(log.InfoLevel)
	// log.SetLevel(log.DebugLevel)

	builder, err := index.BuildIndices(
		context.Background(),
		*sourceUrl,
		*targetUrl,
		*networkPassphrase,
		historyarchive.Range{
			Low:  uint32(max(*start, 2)),
			High: uint32(*end),
		},
		strings.Split(*modules, ","),
		*workerCount,
	)
	if err != nil {
		panic(err)
	}

	if *watch {
		if err := builder.Watch(context.Background()); err != nil {
			panic(err)
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
