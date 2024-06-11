//lint:file-ignore U1001 Ignore all unused code, this is only used in tests.
package integration

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/shantanu-hashcash/go/services/aurora/internal/test"

	"github.com/2opremio/pretty"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/jhttp"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	sdk "github.com/shantanu-hashcash/go/clients/auroraclient"
	"github.com/shantanu-hashcash/go/clients/hcnetcore"
	"github.com/shantanu-hashcash/go/ingest/ledgerbackend"
	"github.com/shantanu-hashcash/go/keypair"
	proto "github.com/shantanu-hashcash/go/protocols/aurora"
	aurora "github.com/shantanu-hashcash/go/services/aurora/internal"
	"github.com/shantanu-hashcash/go/services/aurora/internal/ingest"
	"github.com/shantanu-hashcash/go/support/config"
	"github.com/shantanu-hashcash/go/support/db/dbtest"
	"github.com/shantanu-hashcash/go/support/errors"
	"github.com/shantanu-hashcash/go/txnbuild"
	"github.com/shantanu-hashcash/go/xdr"
)

const (
	StandaloneNetworkPassphrase = "Standalone Network ; February 2017"
	hcnetCorePostgresPassword = "mysecretpassword"
	auroraDefaultPort          = "8000"
	adminPort                   = 6060
	hcnetCorePort             = 11626
	hcnetCorePostgresPort     = 5641
	historyArchivePort          = 1570
	sorobanRPCPort              = 8080
)

const AuroraInitErrStr = "cannot initialize Aurora"

type Config struct {
	ProtocolVersion           uint32
	EnableSorobanRPC          bool
	SkipCoreContainerCreation bool
	CoreDockerImage           string
	SorobanRPCDockerImage     string

	// Weird naming here because bools default to false, but we want to start
	// Aurora by default.
	SkipAuroraStart bool

	// If you want to override the default parameters passed to Aurora, you can
	// set this map accordingly. All of them are passed along as --k=v, but if
	// you pass an empty value, the parameter will be dropped. (Note that you
	// should exclude the prepending `--` from keys; this is for compatibility
	// with the constant names in flags.go)
	//
	// You can also control the environmental variables in a similar way, but
	// note that CLI args take precedence over envvars, so set the corresponding
	// CLI arg empty.
	AuroraWebParameters    map[string]string
	AuroraIngestParameters map[string]string
	AuroraEnvironment      map[string]string
}

type CaptiveConfig struct {
	binaryPath  string
	configPath  string
	storagePath string
	useDB       bool
}

type Test struct {
	t *testing.T

	composePath string

	config              Config
	coreConfig          CaptiveConfig
	auroraIngestConfig aurora.Config
	auroraWebConfig    aurora.Config
	environment         *test.EnvironmentManager

	auroraClient      *sdk.Client
	auroraAdminClient *sdk.AdminClient
	coreClient         *hcnetcore.Client

	webNode       *aurora.App
	ingestNode    *aurora.App
	appStopped    *sync.WaitGroup
	shutdownOnce  sync.Once
	shutdownCalls []func()
	masterKey     *keypair.Full
	passPhrase    string
}

// GetTestConfig returns the default test Config required to run NewTest.
func GetTestConfig() *Config {
	return &Config{
		ProtocolVersion:           17,
		SkipAuroraStart:          true,
		SkipCoreContainerCreation: false,
		AuroraIngestParameters:   map[string]string{},
		AuroraEnvironment:        map[string]string{},
	}
}

// NewTest starts a new environment for integration test at a given
// protocol version and blocks until Aurora starts ingesting.
//
// Skips the test if HORIZON_INTEGRATION_TESTS env variable is not set.
//
// WARNING: This requires Docker Compose installed.
func NewTest(t *testing.T, config Config) *Test {
	if os.Getenv("HORIZON_INTEGRATION_TESTS_ENABLED") == "" {
		t.Skip("skipping integration test: HORIZON_INTEGRATION_TESTS_ENABLED not set")
	}

	if config.ProtocolVersion == 0 {
		// Default to the maximum supported protocol version
		config.ProtocolVersion = ingest.MaxSupportedProtocolVersion
		// If the environment tells us that Core only supports up to certain version,
		// use that.
		maxSupportedCoreProtocolFromEnv := GetCoreMaxSupportedProtocol()
		if maxSupportedCoreProtocolFromEnv != 0 && maxSupportedCoreProtocolFromEnv < ingest.MaxSupportedProtocolVersion {
			config.ProtocolVersion = maxSupportedCoreProtocolFromEnv
		}
	}
	var i *Test
	if !config.SkipCoreContainerCreation {
		composePath := findDockerComposePath()
		i = &Test{
			t:           t,
			config:      config,
			composePath: composePath,
			passPhrase:  StandaloneNetworkPassphrase,
			environment: test.NewEnvironmentManager(),
		}
		i.configureCaptiveCore()
		// Only run Hcnet Core container and its dependencies.
		i.runComposeCommand("up", "--detach", "--quiet-pull", "--no-color", "core")
	} else {
		i = &Test{
			t:           t,
			config:      config,
			environment: test.NewEnvironmentManager(),
		}
	}

	i.prepareShutdownHandlers()
	i.coreClient = &hcnetcore.Client{URL: "http://localhost:" + strconv.Itoa(hcnetCorePort)}
	if !config.SkipCoreContainerCreation {
		i.waitForCore()
		if i.config.EnableSorobanRPC {
			i.runComposeCommand("up", "--detach", "--quiet-pull", "--no-color", "soroban-rpc")
			i.waitForSorobanRPC()
		}
	}

	if !config.SkipAuroraStart {
		if innerErr := i.StartAurora(); innerErr != nil {
			t.Fatalf("Failed to start Aurora: %v", innerErr)
		}

		i.WaitForAurora()
	}

	return i
}

func (i *Test) configureCaptiveCore() {
	composePath := findDockerComposePath()
	i.coreConfig.binaryPath = os.Getenv("HORIZON_INTEGRATION_TESTS_CAPTIVE_CORE_BIN")
	coreConfigFile := "captive-core-classic-integration-tests.cfg"
	if i.config.ProtocolVersion >= ledgerbackend.MinimalSorobanProtocolSupport {
		coreConfigFile = "captive-core-integration-tests.cfg"
	}
	i.coreConfig.configPath = filepath.Join(composePath, coreConfigFile)
	i.coreConfig.storagePath = i.CurrentTest().TempDir()
	i.coreConfig.useDB = true

	if value := i.getIngestParameter(
		aurora.HcnetCoreBinaryPathName,
		"HCNET_CORE_BINARY_PATH",
	); value != "" {
		i.coreConfig.binaryPath = value
	}
	if value := i.getIngestParameter(
		aurora.CaptiveCoreConfigPathName,
		"CAPTIVE_CORE_CONFIG_PATH",
	); value != "" {
		i.coreConfig.configPath = value
	}
}

func (i *Test) getIngestParameter(argName, envName string) string {
	if value, ok := i.config.AuroraEnvironment[envName]; ok {
		return value
	}
	if value, ok := i.config.AuroraIngestParameters[argName]; ok {
		return value
	}
	return ""
}

// Runs a docker-compose command applied to the above configs
func (i *Test) runComposeCommand(args ...string) {
	integrationYaml := filepath.Join(i.composePath, "docker-compose.integration-tests.yml")
	integrationSorobanRPCYaml := filepath.Join(i.composePath, "docker-compose.integration-tests.soroban-rpc.yml")

	cmdline := args
	if i.config.EnableSorobanRPC {
		cmdline = append([]string{"-f", integrationSorobanRPCYaml}, cmdline...)
	}
	cmdline = append([]string{"-f", integrationYaml}, cmdline...)
	cmd := exec.Command("docker-compose", cmdline...)
	coreImageOverride := ""
	if i.config.CoreDockerImage != "" {
		coreImageOverride = i.config.CoreDockerImage
	} else if img := os.Getenv("HORIZON_INTEGRATION_TESTS_DOCKER_IMG"); img != "" {
		coreImageOverride = img
	}

	cmd.Env = os.Environ()
	if coreImageOverride != "" {
		cmd.Env = append(
			cmd.Environ(),
			fmt.Sprintf("CORE_IMAGE=%s", coreImageOverride),
		)
	}
	sorobanRPCOverride := ""
	if i.config.SorobanRPCDockerImage != "" {
		sorobanRPCOverride = i.config.CoreDockerImage
	} else if img := os.Getenv("HORIZON_INTEGRATION_TESTS_SOROBAN_RPC_DOCKER_IMG"); img != "" {
		sorobanRPCOverride = img
	}
	if sorobanRPCOverride != "" {
		cmd.Env = append(
			cmd.Environ(),
			fmt.Sprintf("SOROBAN_RPC_IMAGE=%s", sorobanRPCOverride),
		)
	}

	if i.config.ProtocolVersion < ledgerbackend.MinimalSorobanProtocolSupport {
		cmd.Env = append(
			cmd.Environ(),
			"CORE_CONFIG_FILE=hcnet-core-classic-integration-tests.cfg",
		)
	}

	i.t.Log("Running", cmd.Args)
	out, innerErr := cmd.Output()
	if len(out) > 0 {
		fmt.Printf("stdout:\n%s\n", string(out))
	}
	if exitErr, ok := innerErr.(*exec.ExitError); ok {
		fmt.Printf("stderr:\n%s\n", string(exitErr.Stderr))
	}

	if innerErr != nil {
		i.t.Fatalf("Compose command failed: %v", innerErr)
	}
}

func (i *Test) prepareShutdownHandlers() {
	i.shutdownCalls = append(i.shutdownCalls,
		func() {
			if i.webNode != nil {
				i.webNode.Close()
			}
			if i.ingestNode != nil {
				i.ingestNode.Close()
			}
			if !i.config.SkipCoreContainerCreation {
				i.runComposeCommand("rm", "-fvs", "core")
				i.runComposeCommand("rm", "-fvs", "core-postgres")
				if i.config.EnableSorobanRPC {
					i.runComposeCommand("logs", "soroban-rpc")
					i.runComposeCommand("rm", "-fvs", "soroban-rpc")
				}
			}
		},
		i.environment.Restore,
	)

	// Register cleanup handlers (on panic and ctrl+c) so the containers are
	// stopped even if ingestion or testing fails.
	i.t.Cleanup(i.Shutdown)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		i.Shutdown()
		os.Exit(int(syscall.SIGTERM))
	}()
}

func (i *Test) RestartAurora() error {
	i.StopAurora()

	if err := i.StartAurora(); err != nil {
		return err
	}

	i.WaitForAurora()
	return nil
}

func (i *Test) GetAuroraIngestConfig() aurora.Config {
	return i.auroraIngestConfig
}

func (i *Test) GetAuroraWebConfig() aurora.Config {
	return i.auroraWebConfig
}

// Shutdown stops the integration tests and destroys all its associated
// resources. It will be implicitly called when the calling test (i.e. the
// `testing.Test` passed to `New()`) is finished if it hasn't been explicitly
// called before.
func (i *Test) Shutdown() {
	i.shutdownOnce.Do(func() {
		// run them in the opposite order in which they where added
		for callI := len(i.shutdownCalls) - 1; callI >= 0; callI-- {
			i.shutdownCalls[callI]()
		}
	})
}

// StartAurora initializes and starts the Aurora client-facing API server and the ingest server.
func (i *Test) StartAurora() error {
	postgres := dbtest.Postgres(i.t)
	i.shutdownCalls = append(i.shutdownCalls, func() {
		i.StopAurora()
		postgres.Close()
	})

	// To facilitate custom runs of Aurora, we merge a default set of
	// parameters with the tester-supplied ones (if any).
	mergedWebArgs := MergeMaps(i.getDefaultWebArgs(postgres), i.config.AuroraWebParameters)
	webArgs := mapToFlags(mergedWebArgs)
	i.t.Log("Aurora command line webArgs:", webArgs)

	mergedIngestArgs := MergeMaps(i.getDefaultIngestArgs(postgres), i.config.AuroraIngestParameters)
	ingestArgs := mapToFlags(mergedIngestArgs)
	i.t.Log("Aurora command line ingestArgs:", ingestArgs)

	// setup Aurora web command
	var err error
	webConfig, webConfigOpts := aurora.Flags()
	webCmd := i.createWebCommand(webConfig, webConfigOpts)
	webCmd.SetArgs(webArgs)
	if err = webConfigOpts.Init(webCmd); err != nil {
		return errors.Wrap(err, "cannot initialize params")
	}

	// setup Aurora ingest command
	ingestConfig, ingestConfigOpts := aurora.Flags()
	ingestCmd := i.createIngestCommand(ingestConfig, ingestConfigOpts)
	ingestCmd.SetArgs(ingestArgs)
	if err = ingestConfigOpts.Init(ingestCmd); err != nil {
		return errors.Wrap(err, "cannot initialize params")
	}

	if err = i.initializeEnvironmentVariables(); err != nil {
		return err
	}

	if err = ingestCmd.Execute(); err != nil {
		return errors.Wrap(err, AuroraInitErrStr)
	}

	if err = webCmd.Execute(); err != nil {
		return errors.Wrap(err, AuroraInitErrStr)
	}

	// Set up Aurora clients
	i.setupAuroraClient(mergedWebArgs)
	if err = i.setupAuroraAdminClient(mergedIngestArgs); err != nil {
		return err
	}

	i.auroraIngestConfig = *ingestConfig
	i.auroraWebConfig = *webConfig

	i.appStopped = &sync.WaitGroup{}
	i.appStopped.Add(2)
	go func() {
		_ = i.ingestNode.Serve()
		i.appStopped.Done()
	}()
	go func() {
		_ = i.webNode.Serve()
		i.appStopped.Done()
	}()

	return nil
}

func (i *Test) getDefaultArgs(postgres *dbtest.DB) map[string]string {
	// TODO: Ideally, we'd be pulling host/port information from the Docker
	//       Compose YAML file itself rather than hardcoding it.
	return map[string]string{
		"ingest":               "false",
		"history-archive-urls": fmt.Sprintf("http://%s:%d", "localhost", historyArchivePort),
		"db-url":               postgres.RO_DSN,
		"hcnet-core-url":     i.coreClient.URL,
		"network-passphrase":   i.passPhrase,
		"apply-migrations":     "true",
		"port":                 auroraDefaultPort,
		// due to ARTIFICIALLY_ACCELERATE_TIME_FOR_TESTING
		"checkpoint-frequency": "8",
		"per-hour-rate-limit":  "0",  // disable rate limiting
		"max-db-connections":   "50", // the postgres container supports 100 connections, be conservative
	}
}

func (i *Test) getDefaultWebArgs(postgres *dbtest.DB) map[string]string {
	return MergeMaps(i.getDefaultArgs(postgres), map[string]string{"admin-port": "0"})
}

func (i *Test) getDefaultIngestArgs(postgres *dbtest.DB) map[string]string {
	return MergeMaps(i.getDefaultArgs(postgres), map[string]string{
		"admin-port":                strconv.Itoa(i.AdminPort()),
		"port":                      "8001",
		"db-url":                    postgres.DSN,
		"hcnet-core-binary-path":  i.coreConfig.binaryPath,
		"captive-core-config-path":  i.coreConfig.configPath,
		"captive-core-http-port":    "21626",
		"captive-core-use-db":       strconv.FormatBool(i.coreConfig.useDB),
		"captive-core-storage-path": i.coreConfig.storagePath,
		"ingest":                    "true"})
}

func (i *Test) createWebCommand(webConfig *aurora.Config, webConfigOpts config.ConfigOptions) *cobra.Command {
	webCmd := &cobra.Command{
		Use:   "aurora",
		Short: "Client-facing API server for the Hcnet network",
		Long:  "Client-facing API server for the Hcnet network.",
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			i.webNode, err = aurora.NewAppFromFlags(webConfig, webConfigOpts)
			if err != nil {
				// Explicitly exit here as that's how these tests are structured for now.
				fmt.Println(err)
				os.Exit(1)
			}
		},
	}
	return webCmd
}

func (i *Test) createIngestCommand(ingestConfig *aurora.Config, ingestConfigOpts config.ConfigOptions) *cobra.Command {
	ingestCmd := &cobra.Command{
		Use:   "aurora",
		Short: "Ingest of Hcnet network",
		Long:  "Ingest of Hcnet network.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			i.ingestNode, err = aurora.NewAppFromFlags(ingestConfig, ingestConfigOpts)
			if err != nil {
				fmt.Println(err)
			}
			return err
		},
	}
	return ingestCmd
}

func (i *Test) initializeEnvironmentVariables() error {
	var env strings.Builder
	for key, value := range i.config.AuroraEnvironment {
		env.WriteString(fmt.Sprintf("%s=%s ", key, value))
	}
	i.t.Logf("Aurora environmental variables: %s\n", env.String())

	// prepare env
	for key, value := range i.config.AuroraEnvironment {
		innerErr := i.environment.Add(key, value)
		if innerErr != nil {
			return errors.Wrap(innerErr, fmt.Sprintf(
				"failed to set envvar (%s=%s)", key, value))
		}
	}
	return nil
}

func (i *Test) setupAuroraAdminClient(ingestArgs map[string]string) error {
	adminPort := uint16(i.AdminPort())
	if port, ok := ingestArgs["admin-port"]; ok {
		if cmdAdminPort, parseErr := strconv.ParseInt(port, 0, 16); parseErr == nil {
			adminPort = uint16(cmdAdminPort)
		}
	}

	var err error
	i.auroraAdminClient, err = sdk.NewAdminClient(adminPort, "", 0)
	if err != nil {
		return errors.Wrap(err, "cannot initialize Aurora admin client")
	}
	return nil
}

func (i *Test) setupAuroraClient(webArgs map[string]string) {
	hostname := "localhost"
	auroraPort := auroraDefaultPort
	if port, ok := webArgs["port"]; ok {
		auroraPort = port
	}

	i.auroraClient = &sdk.Client{
		AuroraURL: fmt.Sprintf("http://%s:%s", hostname, auroraPort),
	}
}

const maxWaitForCoreStartup = 30 * time.Second
const maxWaitForCoreUpgrade = 5 * time.Second
const coreStartupPingInterval = time.Second

// Wait for core to be up and manually close the first ledger
func (i *Test) waitForCore() {
	i.t.Log("Waiting for core to be up...")
	startTime := time.Now()
	for time.Since(startTime) < maxWaitForCoreStartup {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		infoTime := time.Now()
		_, err := i.coreClient.Info(ctx)
		cancel()
		if err != nil {
			i.t.Logf("could not obtain info response: %v", err)
			// sleep up to a second between consecutive calls.
			if durationSince := time.Since(infoTime); durationSince < coreStartupPingInterval {
				time.Sleep(coreStartupPingInterval - durationSince)
			}
			continue
		}
		break
	}

	i.UpgradeProtocol(i.config.ProtocolVersion)

	startTime = time.Now()
	for time.Since(startTime) < maxWaitForCoreUpgrade {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		infoTime := time.Now()
		info, err := i.coreClient.Info(ctx)
		cancel()
		if err != nil || !info.IsSynced() {
			i.t.Logf("Core is still not synced: %v %v", err, info)
			// sleep up to a second between consecutive calls.
			if durationSince := time.Since(infoTime); durationSince < coreStartupPingInterval {
				time.Sleep(coreStartupPingInterval - durationSince)
			}
			continue
		}
		i.t.Log("Core is up.")
		return
	}
	i.t.Fatalf("Core could not sync after %v + %v", maxWaitForCoreStartup, maxWaitForCoreUpgrade)
}

const sorobanRPCInitTime = 20 * time.Second
const sorobanRPCHealthCheckInterval = time.Second

// Wait for SorobanRPC to be up
func (i *Test) waitForSorobanRPC() {
	i.t.Log("Waiting for Soroban RPC to be up...")

	start := time.Now()
	for time.Since(start) < sorobanRPCInitTime {
		ctx, cancel := context.WithTimeout(context.Background(), sorobanRPCHealthCheckInterval)
		// TODO: soroban-tools should be exporting a proper Go client
		ch := jhttp.NewChannel("http://localhost:"+strconv.Itoa(sorobanRPCPort), nil)
		sorobanRPCClient := jrpc2.NewClient(ch, nil)
		callTime := time.Now()
		_, err := sorobanRPCClient.Call(ctx, "getHealth", nil)
		cancel()
		if err != nil {
			i.t.Logf("SorobanRPC is unhealthy: %v", err)
			// sleep up to a second between consecutive calls.
			if durationSince := time.Since(callTime); durationSince < sorobanRPCHealthCheckInterval {
				time.Sleep(sorobanRPCHealthCheckInterval - durationSince)
			}
			continue
		}
		i.t.Log("SorobanRPC is up.")
		return
	}

	i.t.Fatalf("SorobanRPC unhealthy after %v", time.Since(start))
}

type RPCSimulateHostFunctionResult struct {
	Auth []string `json:"auth"`
	XDR  string   `json:"xdr"`
}

type RPCSimulateTxResponse struct {
	Error           string                          `json:"error,omitempty"`
	TransactionData string                          `json:"transactionData"`
	Results         []RPCSimulateHostFunctionResult `json:"results"`
	MinResourceFee  int64                           `json:"minResourceFee,string"`
}

func (i *Test) PreflightHostFunctions(
	sourceAccount txnbuild.Account, function txnbuild.InvokeHostFunction,
) (txnbuild.InvokeHostFunction, int64) {
	if function.HostFunction.Type == xdr.HostFunctionTypeHostFunctionTypeInvokeContract {
		fmt.Printf("Preflighting function call to: %s\n", string(function.HostFunction.InvokeContract.FunctionName))
	}
	result, transactionData := i.simulateTransaction(sourceAccount, &function)
	function.Ext = xdr.TransactionExt{
		V:           1,
		SorobanData: &transactionData,
	}
	var funAuth []xdr.SorobanAuthorizationEntry
	for _, res := range result.Results {
		var decodedRes xdr.ScVal
		err := xdr.SafeUnmarshalBase64(res.XDR, &decodedRes)
		assert.NoError(i.t, err)
		fmt.Printf("Result:\n\n%# +v\n\n", pretty.Formatter(decodedRes))
		for _, authBase64 := range res.Auth {
			var authEntry xdr.SorobanAuthorizationEntry
			err = xdr.SafeUnmarshalBase64(authBase64, &authEntry)
			assert.NoError(i.t, err)
			fmt.Printf("Auth:\n\n%# +v\n\n", pretty.Formatter(authEntry))
			funAuth = append(funAuth, authEntry)
		}
	}
	function.Auth = funAuth

	return function, result.MinResourceFee
}

func (i *Test) simulateTransaction(
	sourceAccount txnbuild.Account, op txnbuild.Operation,
) (RPCSimulateTxResponse, xdr.SorobanTransactionData) {
	// Before preflighting, make sure soroban-rpc is in sync with Aurora
	root, err := i.auroraClient.Root()
	assert.NoError(i.t, err)
	i.syncWithSorobanRPC(uint32(root.AuroraSequence))

	// TODO: soroban-tools should be exporting a proper Go client
	ch := jhttp.NewChannel("http://localhost:"+strconv.Itoa(sorobanRPCPort), nil)
	sorobanRPCClient := jrpc2.NewClient(ch, nil)
	txParams := GetBaseTransactionParamsWithFee(sourceAccount, txnbuild.MinBaseFee, op)
	txParams.IncrementSequenceNum = false
	tx, err := txnbuild.NewTransaction(txParams)
	assert.NoError(i.t, err)
	base64, err := tx.Base64()
	assert.NoError(i.t, err)
	result := RPCSimulateTxResponse{}
	fmt.Printf("Preflight TX:\n\n%v \n\n", base64)
	err = sorobanRPCClient.CallResult(context.Background(), "simulateTransaction", struct {
		Transaction string `json:"transaction"`
	}{base64}, &result)
	assert.NoError(i.t, err)
	assert.Empty(i.t, result.Error)
	var transactionData xdr.SorobanTransactionData
	err = xdr.SafeUnmarshalBase64(result.TransactionData, &transactionData)
	assert.NoError(i.t, err)
	fmt.Printf("Transaction Data:\n\n%# +v\n\n", pretty.Formatter(transactionData))
	return result, transactionData
}

func (i *Test) syncWithSorobanRPC(ledgerToWaitFor uint32) {
	for j := 0; j < 20; j++ {
		result := struct {
			Sequence uint32 `json:"sequence"`
		}{}
		ch := jhttp.NewChannel("http://localhost:"+strconv.Itoa(sorobanRPCPort), nil)
		sorobanRPCClient := jrpc2.NewClient(ch, nil)
		err := sorobanRPCClient.CallResult(context.Background(), "getLatestLedger", nil, &result)
		assert.NoError(i.t, err)
		if result.Sequence >= ledgerToWaitFor {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	i.t.Fatal("Time out waiting for soroban-rpc to sync")
}

func (i *Test) WaitUntilLedgerEntryTTL(ledgerKey xdr.LedgerKey) {
	ch := jhttp.NewChannel("http://localhost:"+strconv.Itoa(sorobanRPCPort), nil)
	client := jrpc2.NewClient(ch, nil)

	keyB64, err := xdr.MarshalBase64(ledgerKey)
	assert.NoError(i.t, err)
	request := struct {
		Keys []string `json:"keys"`
	}{
		Keys: []string{keyB64},
	}
	ttled := false
	for attempt := 0; attempt < 50; attempt++ {
		var result struct {
			Entries []struct {
				LiveUntilLedgerSeq *uint32 `json:"liveUntilLedgerSeq,omitempty"`
			} `json:"entries"`
		}
		err := client.CallResult(context.Background(), "getLedgerEntries", request, &result)
		assert.NoError(i.t, err)
		if len(result.Entries) > 0 {
			liveUntilLedgerSeq := *result.Entries[0].LiveUntilLedgerSeq

			root, err := i.auroraClient.Root()
			assert.NoError(i.t, err)
			if uint32(root.AuroraSequence) > liveUntilLedgerSeq {
				ttled = true
				i.t.Logf("ledger entry ttl'ed")
				break
			}
			i.t.Log("waiting for ledger entry to ttl at ledger", liveUntilLedgerSeq)
		} else {
			i.t.Log("waiting for soroban-rpc to ingest the ledger entries")
		}
		time.Sleep(time.Second)
	}
	assert.True(i.t, ttled)
}

func (i *Test) PreflightExtendExpiration(
	account string, ledgerKeys []xdr.LedgerKey, extendAmt uint32,
) (proto.Account, txnbuild.ExtendFootprintTtl, int64) {
	sourceAccount, err := i.Client().AccountDetail(sdk.AccountRequest{
		AccountID: account,
	})
	assert.NoError(i.t, err)

	bumpFootprint := txnbuild.ExtendFootprintTtl{
		ExtendTo:      extendAmt,
		SourceAccount: "",
		Ext: xdr.TransactionExt{
			V: 1,
			SorobanData: &xdr.SorobanTransactionData{
				Ext: xdr.ExtensionPoint{},
				Resources: xdr.SorobanResources{
					Footprint: xdr.LedgerFootprint{
						ReadOnly:  ledgerKeys,
						ReadWrite: nil,
					},
				},
				ResourceFee: 0,
			},
		},
	}
	result, transactionData := i.simulateTransaction(&sourceAccount, &bumpFootprint)
	bumpFootprint.Ext = xdr.TransactionExt{
		V:           1,
		SorobanData: &transactionData,
	}

	return sourceAccount, bumpFootprint, result.MinResourceFee
}

func (i *Test) RestoreFootprint(
	account string, ledgerKey xdr.LedgerKey,
) (proto.Account, txnbuild.RestoreFootprint, int64) {
	sourceAccount, err := i.Client().AccountDetail(sdk.AccountRequest{
		AccountID: account,
	})
	assert.NoError(i.t, err)

	restoreFootprint := txnbuild.RestoreFootprint{
		SourceAccount: "",
		Ext: xdr.TransactionExt{
			V: 1,
			SorobanData: &xdr.SorobanTransactionData{
				Ext: xdr.ExtensionPoint{},
				Resources: xdr.SorobanResources{
					Footprint: xdr.LedgerFootprint{
						ReadWrite: []xdr.LedgerKey{ledgerKey},
					},
				},
				ResourceFee: 0,
			},
		},
	}
	result, transactionData := i.simulateTransaction(&sourceAccount, &restoreFootprint)
	restoreFootprint.Ext = xdr.TransactionExt{
		V:           1,
		SorobanData: &transactionData,
	}

	return sourceAccount, restoreFootprint, result.MinResourceFee
}

// UpgradeProtocol arms Core with upgrade and blocks until protocol is upgraded.
func (i *Test) UpgradeProtocol(version uint32) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err := i.coreClient.Upgrade(ctx, int(version))
	cancel()
	if err != nil {
		i.t.Fatalf("could not upgrade protocol: %v", err)
	}

	for t := 0; t < 10; t++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		info, err := i.coreClient.Info(ctx)
		cancel()
		if err != nil {
			i.t.Logf("could not obtain info response: %v", err)
			time.Sleep(time.Second)
			continue
		}

		if info.Info.Ledger.Version == int(version) {
			i.t.Logf("Protocol upgraded to: %d", info.Info.Ledger.Version)
			return
		}
		time.Sleep(time.Second)
	}

	i.t.Fatalf("could not upgrade protocol in 10s")
}

func (i *Test) WaitForAurora() {
	for t := 60; t >= 0; t -= 1 {
		time.Sleep(time.Second)

		i.t.Log("Waiting for ingestion and protocol upgrade...")
		root, err := i.auroraClient.Root()
		if err != nil {
			i.t.Logf("could not obtain root response %v", err)
			continue
		}

		if root.AuroraSequence < 3 ||
			int(root.AuroraSequence) != int(root.IngestSequence) {
			i.t.Logf("Aurora ingesting... %v", root)
			continue
		}

		if uint32(root.CurrentProtocolVersion) == i.config.ProtocolVersion {
			i.t.Logf("Aurora protocol version matches %d: %+v",
				root.CurrentProtocolVersion, root)
			return
		}
	}

	i.t.Fatal("Aurora not ingesting...")
}

// Config returns the testing configuration for the current integration test run.
func (i *Test) Config() Config {
	return i.config
}

// CoreClient returns a hcnet core client connected to the Hcnet Core instance.
func (i *Test) CoreClient() *hcnetcore.Client {
	return i.coreClient
}

// Client returns aurora.Client connected to started Aurora instance.
func (i *Test) Client() *sdk.Client {
	return i.auroraClient
}

// AdminClient returns aurora.Client connected to started Aurora instance.
func (i *Test) AdminClient() *sdk.AdminClient {
	return i.auroraAdminClient
}

// AuroraWeb returns the aurora.App instance for the current integration test
func (i *Test) AuroraWeb() *aurora.App {
	return i.webNode
}

func (i *Test) AuroraIngest() *aurora.App {
	return i.ingestNode
}

// StopAurora shuts down the running Aurora process
func (i *Test) StopAurora() {
	if i.webNode != nil {
		i.webNode.Close()
	}
	if i.ingestNode != nil {
		i.ingestNode.Close()
	}

	// Wait for Aurora to shut down completely.
	if i.appStopped != nil {
		i.appStopped.Wait()
	}
	i.webNode = nil
	i.ingestNode = nil
}

// AdminPort returns Aurora admin port.
func (i *Test) AdminPort() int {
	return adminPort
}

// Metrics URL returns Aurora metrics URL.
func (i *Test) MetricsURL() string {
	return fmt.Sprintf("http://localhost:%d/metrics", i.AdminPort())
}

// AsyncTxSubOpenAPISpecURL returns the URL for getting the openAPI spec yaml for async-txsub endpoint.
func (i *Test) AsyncTxSubOpenAPISpecURL() string {
	return fmt.Sprintf("http://localhost:%d/transactions_async", i.AdminPort())
}

// Master returns a keypair of the network masterKey account.
func (i *Test) Master() *keypair.Full {
	if i.masterKey != nil {
		return i.masterKey
	}
	return keypair.Master(i.passPhrase).(*keypair.Full)
}

func (i *Test) MasterAccount() txnbuild.Account {
	account := i.MasterAccountDetails()
	return &account
}

func (i *Test) MasterAccountDetails() proto.Account {
	return i.MustGetAccount(i.Master())
}

func (i *Test) CurrentTest() *testing.T {
	return i.t
}

/* Utility functions for easier test case creation. */

// Creates new accounts via the master account.
//
// It funds each account with the given balance and then queries the API to
// find the randomized sequence number for future operations.
//
// Returns: The slice of created keypairs and account objects.
//
// Note: panics on any errors, since we assume that tests cannot proceed without
// this method succeeding.
func (i *Test) CreateAccounts(count int, initialBalance string) ([]*keypair.Full, []txnbuild.Account) {
	client := i.Client()
	master := i.Master()

	pairs := make([]*keypair.Full, count)
	ops := make([]txnbuild.Operation, count)

	// Two paths here: either caller already did some stuff with the master
	// account so we should retrieve the sequence number, or caller hasn't and
	// we start from scratch.
	request := sdk.AccountRequest{AccountID: master.Address()}
	account, err := client.AccountDetail(request)
	panicIf(err)

	masterAccount := txnbuild.SimpleAccount{
		AccountID: master.Address(),
		Sequence:  account.Sequence,
	}

	for i := 0; i < count; i++ {
		pair, _ := keypair.Random()
		pairs[i] = pair

		ops[i] = &txnbuild.CreateAccount{
			SourceAccount: masterAccount.AccountID,
			Destination:   pair.Address(),
			Amount:        initialBalance,
		}
	}

	// Submit transaction, then retrieve new account details.
	_ = i.MustSubmitOperations(&masterAccount, master, ops...)

	accounts := make([]txnbuild.Account, count)
	for i, kp := range pairs {
		request := sdk.AccountRequest{AccountID: kp.Address()}
		account, err := client.AccountDetail(request)
		panicIf(err)

		accounts[i] = &account
	}

	for _, keys := range pairs {
		i.t.Logf("Funded %s (%s) with %s XLM.\n",
			keys.Seed(), keys.Address(), initialBalance)
	}

	return pairs, accounts
}

// CreateAccount creates a new account via the master account.
func (i *Test) CreateAccount(initialBalance string) (*keypair.Full, txnbuild.Account) {
	kps, accts := i.CreateAccounts(1, initialBalance)
	return kps[0], accts[0]
}

// Panics on any error establishing a trustline.
func (i *Test) MustEstablishTrustline(
	truster *keypair.Full, account txnbuild.Account, asset txnbuild.Asset,
) (resp proto.Transaction) {
	txResp, err := i.EstablishTrustline(truster, account, asset)
	panicIf(err)
	return txResp
}

// EstablishTrustline works on a given asset for a particular account.
func (i *Test) EstablishTrustline(
	truster *keypair.Full, account txnbuild.Account, asset txnbuild.Asset,
) (proto.Transaction, error) {
	if asset.IsNative() {
		return proto.Transaction{}, nil
	}
	line, err := asset.ToChangeTrustAsset()
	if err != nil {
		return proto.Transaction{}, err
	}
	return i.SubmitOperations(account, truster, &txnbuild.ChangeTrust{
		Line:  line,
		Limit: "2000",
	})
}

// MustCreateClaimableBalance panics on any error creating a claimable balance.
func (i *Test) MustCreateClaimableBalance(
	source *keypair.Full, asset txnbuild.Asset, amount string,
	claimants ...txnbuild.Claimant,
) (claim proto.ClaimableBalance) {
	account := i.MustGetAccount(source)
	_ = i.MustSubmitOperations(&account, source,
		&txnbuild.CreateClaimableBalance{
			Destinations: claimants,
			Asset:        asset,
			Amount:       amount,
		},
	)

	// Ensure it exists in the global list
	balances, err := i.Client().ClaimableBalances(sdk.ClaimableBalanceRequest{})
	panicIf(err)

	claims := balances.Embedded.Records
	if len(claims) == 0 {
		panic(-1)
	}

	claim = claims[len(claims)-1] // latest one
	i.t.Logf("Created claimable balance w/ id=%s", claim.BalanceID)
	return
}

// MustGetAccount panics on any error retrieves an account's details from its
// key. This means it must have previously been funded.
func (i *Test) MustGetAccount(source *keypair.Full) proto.Account {
	client := i.Client()
	account, err := client.AccountDetail(sdk.AccountRequest{AccountID: source.Address()})
	panicIf(err)
	return account
}

// MustSubmitOperations submits a signed transaction from an account with
// standard options.
//
// Namely, we set the standard fee, time bounds, etc. to "non-production"
// defaults that work well for tests.
//
// Most transactions only need one signer, so see the more verbose
// `MustSubmitOperationsWithSigners` below for multi-sig transactions.
//
// Note: We assume that transaction will be successful here so we panic in case
// of all errors. To allow failures, use `SubmitOperations`.
func (i *Test) MustSubmitOperations(
	source txnbuild.Account, signer *keypair.Full, ops ...txnbuild.Operation,
) proto.Transaction {
	return i.MustSubmitOperationsWithFee(source, signer, txnbuild.MinBaseFee, ops...)
}

func (i *Test) MustSubmitOperationsWithFee(
	source txnbuild.Account, signer *keypair.Full, fee int64, ops ...txnbuild.Operation,
) proto.Transaction {
	tx, err := i.SubmitOperationsWithFee(source, signer, fee, ops...)
	panicIf(err)
	return tx
}

func (i *Test) SubmitOperations(
	source txnbuild.Account, signer *keypair.Full, ops ...txnbuild.Operation,
) (proto.Transaction, error) {
	return i.SubmitMultiSigOperations(source, []*keypair.Full{signer}, ops...)
}

func (i *Test) SubmitMultiSigOperations(
	source txnbuild.Account, signers []*keypair.Full, ops ...txnbuild.Operation,
) (proto.Transaction, error) {
	return i.SubmitMultiSigOperationsWithFee(source, signers, txnbuild.MinBaseFee, ops...)
}

func (i *Test) SubmitOperationsWithFee(
	source txnbuild.Account, signer *keypair.Full, fee int64, ops ...txnbuild.Operation,
) (proto.Transaction, error) {
	return i.SubmitMultiSigOperationsWithFee(source, []*keypair.Full{signer}, fee, ops...)
}

func (i *Test) SubmitMultiSigOperationsWithFee(
	source txnbuild.Account, signers []*keypair.Full, fee int64, ops ...txnbuild.Operation,
) (proto.Transaction, error) {
	tx, err := i.CreateSignedTransactionFromOpsWithFee(source, signers, fee, ops...)
	if err != nil {
		return proto.Transaction{}, err
	}
	return i.Client().SubmitTransaction(tx)
}

func (i *Test) MustSubmitMultiSigOperations(
	source txnbuild.Account, signers []*keypair.Full, ops ...txnbuild.Operation,
) proto.Transaction {
	tx, err := i.SubmitMultiSigOperations(source, signers, ops...)
	panicIf(err)
	return tx
}

func (i *Test) MustSubmitTransaction(signer *keypair.Full, txParams txnbuild.TransactionParams,
) proto.Transaction {
	tx, err := i.SubmitTransaction(signer, txParams)
	panicIf(err)
	return tx
}

func (i *Test) SubmitTransaction(
	signer *keypair.Full, txParams txnbuild.TransactionParams,
) (proto.Transaction, error) {
	return i.SubmitMultiSigTransaction([]*keypair.Full{signer}, txParams)
}

func (i *Test) SubmitMultiSigTransaction(
	signers []*keypair.Full, txParams txnbuild.TransactionParams,
) (proto.Transaction, error) {
	tx, err := i.CreateSignedTransaction(signers, txParams)
	if err != nil {
		return proto.Transaction{}, err
	}
	return i.Client().SubmitTransaction(tx)
}

func (i *Test) AsyncSubmitTransaction(
	signer *keypair.Full, txParams txnbuild.TransactionParams,
) (proto.AsyncTransactionSubmissionResponse, error) {
	tx, err := i.CreateSignedTransaction([]*keypair.Full{signer}, txParams)
	if err != nil {
		return proto.AsyncTransactionSubmissionResponse{}, err
	}
	return i.Client().AsyncSubmitTransaction(tx)
}

func (i *Test) MustSubmitMultiSigTransaction(
	signers []*keypair.Full, txParams txnbuild.TransactionParams,
) proto.Transaction {
	tx, err := i.SubmitMultiSigTransaction(signers, txParams)
	panicIf(err)
	return tx
}

func (i *Test) CreateSignedTransaction(signers []*keypair.Full, txParams txnbuild.TransactionParams,
) (*txnbuild.Transaction, error) {
	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return nil, err
	}

	for _, signer := range signers {
		tx, err = tx.Sign(i.passPhrase, signer)
		if err != nil {
			return nil, err
		}
	}

	return tx, nil
}

func (i *Test) CreateSignedTransactionFromOps(
	source txnbuild.Account, signers []*keypair.Full, ops ...txnbuild.Operation,
) (*txnbuild.Transaction, error) {
	return i.CreateSignedTransactionFromOpsWithFee(source, signers, txnbuild.MinBaseFee, ops...)
}

func (i *Test) CreateSignedTransactionFromOpsWithFee(
	source txnbuild.Account, signers []*keypair.Full, fee int64, ops ...txnbuild.Operation,
) (*txnbuild.Transaction, error) {
	txParams := GetBaseTransactionParamsWithFee(source, fee, ops...)
	return i.CreateSignedTransaction(signers, txParams)
}

func GetBaseTransactionParamsWithFee(source txnbuild.Account, fee int64, ops ...txnbuild.Operation) txnbuild.TransactionParams {
	return txnbuild.TransactionParams{
		SourceAccount:        source,
		Operations:           ops,
		BaseFee:              fee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
		IncrementSequenceNum: true,
	}
}

func (i *Test) CreateUnsignedTransaction(
	source txnbuild.Account, ops ...txnbuild.Operation,
) (*txnbuild.Transaction, error) {
	txParams := GetBaseTransactionParamsWithFee(source, txnbuild.MinBaseFee, ops...)
	return txnbuild.NewTransaction(txParams)
}

func (i *Test) GetCurrentCoreLedgerSequence() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	info, err := i.coreClient.Info(ctx)
	if err != nil {
		return 0, err
	}
	return info.Info.Ledger.Num, nil
}

// LogFailedTx is a convenience function to provide verbose information about a
// failing transaction to the test output log, if it's expected to succeed.
func (i *Test) LogFailedTx(txResponse proto.Transaction, auroraResult error) {
	t := i.CurrentTest()
	assert.NoErrorf(t, auroraResult, "Submitting the transaction failed")
	if prob := sdk.GetError(auroraResult); prob != nil {
		t.Logf("  problem: %s\n", prob.Problem.Detail)
		t.Logf("  extras: %s\n", prob.Problem.Extras["result_codes"])
		return
	}

	var txResult xdr.TransactionResult
	err := xdr.SafeUnmarshalBase64(txResponse.ResultXdr, &txResult)
	assert.NoErrorf(t, err, "Unmarshaling transaction failed.")
	assert.Equalf(t, xdr.TransactionResultCodeTxSuccess, txResult.Result.Code,
		"Transaction did not succeed: %d", txResult.Result.Code)
}

func (i *Test) GetPassPhrase() string {
	return i.passPhrase
}

// Cluttering code with if err != nil is absolute nonsense.
func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

// findDockerComposePath performs a best-effort attempt to find the project's
// Docker Compose files.
func findDockerComposePath() string {
	// Lets you check if a particular directory contains a file.
	directoryContainsFilename := func(dir string, filename string) bool {
		files, innerErr := ioutil.ReadDir(dir)
		panicIf(innerErr)

		for _, file := range files {
			if file.Name() == filename {
				return true
			}
		}

		return false
	}

	current, err := os.Getwd()
	panicIf(err)

	//
	// We have a primary and backup attempt for finding the necessary docker
	// files: via $GOPATH and via local directory traversal.
	//

	if gopath := os.Getenv("GOPATH"); gopath != "" {
		monorepo := filepath.Join(gopath, "src", "github.com", "hcnet", "go")
		if _, err = os.Stat(monorepo); !os.IsNotExist(err) {
			current = monorepo
		}
	}

	// In either case, we try to walk up the tree until we find "go.mod",
	// which we hope is the root directory of the project.
	for !directoryContainsFilename(current, "go.mod") {
		current, err = filepath.Abs(filepath.Join(current, ".."))

		// FIXME: This only works on *nix-like systems.
		if err != nil || filepath.Base(current)[0] == filepath.Separator {
			fmt.Println("Failed to establish project root directory.")
			panic(err)
		}
	}

	// Directly jump down to the folder that should contain the configs
	return filepath.Join(current, "services", "aurora", "docker")
}

// MergeMaps returns a new map which contains the keys and values of *all* input
// maps, overwriting earlier values with later values on duplicate keys.
func MergeMaps(maps ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}
	return merged
}

// mapToFlags will convert a map of parameters into an array of CLI args (i.e.
// in the form --key=value). Note that an empty value for a key means to drop
// the parameter.
func mapToFlags(params map[string]string) []string {
	args := make([]string, 0, len(params))
	for key, value := range params {
		if value == "" {
			continue
		}

		args = append(args, fmt.Sprintf("--%s=%s", key, value))
	}
	return args
}

func GetCoreMaxSupportedProtocol() uint32 {
	str := os.Getenv("HORIZON_INTEGRATION_TESTS_CORE_MAX_SUPPORTED_PROTOCOL")
	if str == "" {
		return 0
	}
	version, err := strconv.ParseUint(str, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(version)
}

func (i *Test) GetEffectiveProtocolVersion() uint32 {
	return i.config.ProtocolVersion
}
