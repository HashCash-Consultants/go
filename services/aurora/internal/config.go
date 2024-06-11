package aurora

import (
	"net/url"
	"time"

	"github.com/shantanu-hashcash/go/ingest/ledgerbackend"

	"github.com/sirupsen/logrus"
	"github.com/stellar/throttled"
)

// Config is the configuration for aurora.  It gets populated by the
// app's main function and is provided to NewApp.
type Config struct {
	DatabaseURL        string
	RoDatabaseURL      string
	HistoryArchiveURLs []string
	Port               uint
	AdminPort          uint

	CaptiveCoreBinaryPath       string
	CaptiveCoreConfigPath       string
	CaptiveCoreTomlParams       ledgerbackend.CaptiveCoreTomlParams
	CaptiveCoreToml             *ledgerbackend.CaptiveCoreToml
	CaptiveCoreStoragePath      string
	CaptiveCoreReuseStoragePath bool
	CaptiveCoreConfigUseDB      bool
	HistoryArchiveCaching       bool

	HcnetCoreURL string

	// MaxDBConnections has a priority over all 4 values below.
	MaxDBConnections            int
	AuroraDBMaxOpenConnections int
	AuroraDBMaxIdleConnections int

	SSEUpdateFrequency time.Duration
	ConnectionTimeout  time.Duration
	ClientQueryTimeout time.Duration
	// MaxHTTPRequestSize is the maximum allowed request payload size
	MaxHTTPRequestSize    uint
	RateQuota             *throttled.RateQuota
	MaxConcurrentRequests uint
	FriendbotURL          *url.URL
	LogLevel              logrus.Level
	LogFile               string

	// MaxPathLength is the maximum length of the path returned by `/paths` endpoint.
	MaxPathLength uint
	// MaxAssetsPerPathRequest is the maximum number of assets considered for `/paths/strict-send` and `/paths/strict-receive`
	MaxAssetsPerPathRequest int
	// DisablePoolPathFinding configures aurora to run path finding without including liquidity pools
	// in the path finding search.
	DisablePoolPathFinding bool
	// DisablePathFinding configures aurora without the path finding endpoint.
	DisablePathFinding bool
	// MaxPathFindingRequests is the maximum number of path finding requests aurora will allow
	// in a 1-second period. A value of 0 disables the limit.
	MaxPathFindingRequests uint

	NetworkPassphrase string
	SentryDSN         string
	LogglyToken       string
	LogglyTag         string
	// TLSCert is a path to a certificate file to use for aurora's TLS config
	TLSCert string
	// TLSKey is the path to a private key file to use for aurora's TLS config
	TLSKey string
	// Ingest toggles whether this aurora instance should run the data ingestion subsystem.
	Ingest bool
	// HistoryRetentionCount represents the minimum number of ledgers worth of
	// history data to retain in the aurora database. For the purposes of
	// determining a "retention duration", each ledger roughly corresponds to 10
	// seconds of real time.
	HistoryRetentionCount uint
	// HistoryRetentionReapCount is the number of ledgers worth of history data
	// to remove per second from the Aurora database. It is intended to allow
	// control over the amount of CPU and database load caused by reaping,
	// especially if enabling reaping for the first time or in times of
	// increased ledger load.
	HistoryRetentionReapCount uint
	// StaleThreshold represents the number of ledgers a history database may be
	// out-of-date by before aurora begins to respond with an error to history
	// requests.
	StaleThreshold uint
	// IngestDisableStateVerification disables state verification
	// `System.verifyState()` when set to `true`.
	IngestDisableStateVerification bool
	// IngestStateVerificationCheckpointFrequency configures how often state verification is performed.
	// If IngestStateVerificationCheckpointFrequency is set to 1 state verification is run on every checkpoint,
	// If IngestStateVerificationCheckpointFrequency is set to 2 state verification is run on every second checkpoint,
	// etc...
	IngestStateVerificationCheckpointFrequency uint
	// IngestStateVerificationTimeout configures a timeout on the state verification routine.
	// If IngestStateVerificationTimeout is set to 0 the timeout is disabled.
	IngestStateVerificationTimeout time.Duration
	// IngestEnableExtendedLogLedgerStats enables extended ledger stats in
	// logging.
	IngestEnableExtendedLogLedgerStats bool
	// ApplyMigrations will apply pending migrations to the aurora database
	// before starting the aurora service
	ApplyMigrations bool
	// CheckpointFrequency establishes how many ledgers exist between checkpoints
	CheckpointFrequency uint32
	// BehindCloudflare determines if Aurora instance is behind Cloudflare. In
	// such case http.Request.RemoteAddr will be replaced with Cloudflare header.
	BehindCloudflare bool
	// BehindAWSLoadBalancer determines if Aurora instance is behind AWS load
	// balances like ELB or ALB. In such case http.Request.RemoteAddr will be
	// replaced with the last IP in X-Forwarded-For header.
	BehindAWSLoadBalancer bool
	// RoundingSlippageFilter excludes trades from /trade_aggregations with rounding slippage >x bps
	RoundingSlippageFilter int
	// Hcnet network: 'testnet' or 'pubnet'
	Network string
	// DisableTxSub disables transaction submission functionality for Aurora.
	DisableTxSub bool
	// SkipTxmeta, when enabled, will not store meta xdr in history transaction table
	SkipTxmeta bool
}
