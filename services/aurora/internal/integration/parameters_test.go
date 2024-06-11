//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite
package integration

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	stdLog "log"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/shantanu-hashcash/go/services/aurora/internal/paths"
	"github.com/shantanu-hashcash/go/services/aurora/internal/simplepath"

	auroracmd "github.com/shantanu-hashcash/go/services/aurora/cmd"
	aurora "github.com/shantanu-hashcash/go/services/aurora/internal"
	"github.com/shantanu-hashcash/go/services/aurora/internal/test/integration"

	"github.com/stretchr/testify/assert"
)

var defaultCaptiveCoreParameters = map[string]string{
	aurora.HcnetCoreBinaryPathName: os.Getenv("CAPTIVE_CORE_BIN"),
	aurora.HcnetCoreURLFlagName:    "",
}

var networkParamArgs = map[string]string{
	aurora.CaptiveCoreConfigPathName:   "",
	aurora.CaptiveCoreHTTPPortFlagName: "",
	aurora.HcnetCoreBinaryPathName:   "",
	aurora.HcnetCoreURLFlagName:      "",
	aurora.HistoryArchiveURLsFlagName:  "",
	aurora.NetworkPassphraseFlagName:   "",
}

const (
	SimpleCaptiveCoreToml = `
		PEER_PORT=11725
		ARTIFICIALLY_ACCELERATE_TIME_FOR_TESTING=true

		UNSAFE_QUORUM=true
		FAILURE_SAFETY=0

		[[VALIDATORS]]
		NAME="local_core"
		HOME_DOMAIN="core.local"
		PUBLIC_KEY="GD5KD2KEZJIGTC63IGW6UMUSMVUVG5IHG64HUTFWCHVZH2N2IBOQN7PS"
		ADDRESS="localhost"
		QUALITY="MEDIUM"`

	HcnetCoreURL = "http://localhost:11626"
)

var (
	CaptiveCoreConfigErrMsg = "error generating captive core configuration: invalid config: "
)

// Ensures that BUCKET_DIR_PATH is not an allowed value for Captive Core.
func TestBucketDirDisallowed(t *testing.T) {
	config := `BUCKET_DIR_PATH="/tmp"
		` + SimpleCaptiveCoreToml

	confName, _, cleanup := createCaptiveCoreConfig(config)
	defer cleanup()
	testConfig := integration.GetTestConfig()
	testConfig.AuroraIngestParameters = map[string]string{
		aurora.CaptiveCoreConfigPathName: confName,
		aurora.HcnetCoreBinaryPathName: os.Getenv("CAPTIVE_CORE_BIN"),
	}
	test := integration.NewTest(t, *testConfig)
	err := test.StartAurora()
	assert.Equal(t, err.Error(), integration.AuroraInitErrStr+": error generating captive core configuration:"+
		" invalid captive core toml file: could not unmarshal captive core toml: setting BUCKET_DIR_PATH is disallowed"+
		" for Captive Core, use CAPTIVE_CORE_STORAGE_PATH instead")
	time.Sleep(1 * time.Second)
	test.StopAurora()
	test.Shutdown()
}

func TestEnvironmentPreserved(t *testing.T) {
	// Who tests the tests? This test.
	//
	// It ensures that the global OS environmental variables are preserved after
	// running an integration test.

	// Note that we ALSO need to make sure we don't modify parent env state.
	value, isSet := os.LookupEnv("HCNET_CORE_URL")
	defer func() {
		if isSet {
			_ = os.Setenv("HCNET_CORE_URL", value)
		} else {
			_ = os.Unsetenv("HCNET_CORE_URL")
		}
	}()

	err := os.Setenv("HCNET_CORE_URL", "original value")
	assert.NoError(t, err)

	testConfig := integration.GetTestConfig()
	testConfig.AuroraEnvironment = map[string]string{
		"HCNET_CORE_URL": HcnetCoreURL,
	}
	test := integration.NewTest(t, *testConfig)

	err = test.StartAurora()
	assert.NoError(t, err)
	test.WaitForAurora()

	envValue := os.Getenv("HCNET_CORE_URL")
	assert.Equal(t, HcnetCoreURL, envValue)

	test.Shutdown()

	envValue = os.Getenv("HCNET_CORE_URL")
	assert.Equal(t, "original value", envValue)
}

// TestInvalidNetworkParameters Ensure that Aurora returns an error when
// using NETWORK environment variables, history archive urls or network passphrase
// parameters are also set.
func TestInvalidNetworkParameters(t *testing.T) {
	var captiveCoreConfigErrMsg = integration.AuroraInitErrStr + ": error generating captive " +
		"core configuration: invalid config: %s parameter not allowed with the %s parameter"
	testCases := []struct {
		name         string
		errMsg       string
		networkValue string
		param        string
	}{
		{
			name: "history archive urls validation",
			errMsg: fmt.Sprintf(captiveCoreConfigErrMsg, aurora.HistoryArchiveURLsFlagName,
				aurora.NetworkFlagName),
			networkValue: aurora.HcnetPubnet,
			param:        aurora.HistoryArchiveURLsFlagName,
		},
		{
			name: "network-passphrase validation",
			errMsg: fmt.Sprintf(captiveCoreConfigErrMsg, aurora.NetworkPassphraseFlagName,
				aurora.NetworkFlagName),
			networkValue: aurora.HcnetTestnet,
			param:        aurora.NetworkPassphraseFlagName,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			localParams := integration.MergeMaps(networkParamArgs, map[string]string{
				aurora.NetworkFlagName: testCase.networkValue,
				testCase.param:          testCase.param, // set any value
			})
			testConfig := integration.GetTestConfig()
			testConfig.SkipCoreContainerCreation = true
			testConfig.AuroraIngestParameters = localParams
			test := integration.NewTest(t, *testConfig)
			err := test.StartAurora()
			// Adding sleep as a workaround for the race condition in the ingestion system.
			// https://github.com/shantanu-hashcash/go/issues/5005
			time.Sleep(2 * time.Second)
			assert.Equal(t, testCase.errMsg, err.Error())
			test.Shutdown()
		})
	}
}

// TestNetworkParameter Ensure that Aurora successfully starts the captive-core
// subprocess using the default configuration when --network [testnet|pubnet]
// commandline parameter.
//
// In integration tests, we start Aurora and hcnet-core containers in standalone mode
// simultaneously. We usually wait for Aurora to begin ingesting to verify the test's
// success. However, for "pubnet" or "testnet," we can not wait for Aurora to catch up,
// so we skip starting hcnet-core containers.
func TestNetworkParameter(t *testing.T) {
	testCases := []struct {
		networkValue       string
		networkPassphrase  string
		historyArchiveURLs []string
	}{
		{
			networkValue:       aurora.HcnetTestnet,
			networkPassphrase:  aurora.TestnetConf.NetworkPassphrase,
			historyArchiveURLs: aurora.TestnetConf.HistoryArchiveURLs,
		},
		{
			networkValue:       aurora.HcnetPubnet,
			networkPassphrase:  aurora.PubnetConf.NetworkPassphrase,
			historyArchiveURLs: aurora.PubnetConf.HistoryArchiveURLs,
		},
	}
	for _, tt := range testCases {
		t.Run(fmt.Sprintf("NETWORK parameter %s", tt.networkValue), func(t *testing.T) {
			localParams := integration.MergeMaps(networkParamArgs, map[string]string{
				aurora.NetworkFlagName: tt.networkValue,
			})
			testConfig := integration.GetTestConfig()
			testConfig.SkipCoreContainerCreation = true
			testConfig.AuroraIngestParameters = localParams
			test := integration.NewTest(t, *testConfig)
			err := test.StartAurora()
			// Adding sleep as a workaround for the race condition in the ingestion system.
			// https://github.com/shantanu-hashcash/go/issues/5005
			time.Sleep(2 * time.Second)
			assert.NoError(t, err)
			assert.Equal(t, test.GetAuroraIngestConfig().HistoryArchiveURLs, tt.historyArchiveURLs)
			assert.Equal(t, test.GetAuroraIngestConfig().NetworkPassphrase, tt.networkPassphrase)

			test.Shutdown()
		})
	}
}

// TestNetworkEnvironmentVariable Ensure that Aurora successfully starts the captive-core
// subprocess using the default configuration when the NETWORK environment variable is set
// to either pubnet or testnet.
//
// In integration tests, we start Aurora and hcnet-core containers in standalone mode
// simultaneously. We usually wait for Aurora to begin ingesting to verify the test's
// success. However, for "pubnet" or "testnet," we can not wait for Aurora to catch up,
// so we skip starting hcnet-core containers.
func TestNetworkEnvironmentVariable(t *testing.T) {
	testCases := []string{
		aurora.HcnetPubnet,
		aurora.HcnetTestnet,
	}

	for _, networkValue := range testCases {
		t.Run(fmt.Sprintf("NETWORK environment variable %s", networkValue), func(t *testing.T) {
			value, isSet := os.LookupEnv("NETWORK")
			defer func() {
				if isSet {
					_ = os.Setenv("NETWORK", value)
				} else {
					_ = os.Unsetenv("NETWORK")
				}
			}()

			testConfig := integration.GetTestConfig()
			testConfig.SkipCoreContainerCreation = true
			testConfig.AuroraIngestParameters = networkParamArgs
			testConfig.AuroraEnvironment = map[string]string{"NETWORK": networkValue}
			test := integration.NewTest(t, *testConfig)
			err := test.StartAurora()
			// Adding sleep here as a workaround for the race condition in the ingestion system.
			// More details can be found at https://github.com/shantanu-hashcash/go/issues/5005
			time.Sleep(2 * time.Second)
			assert.NoError(t, err)
			test.Shutdown()
		})
	}
}

// Ensures that the filesystem ends up in the correct state with Captive Core.
func TestCaptiveCoreConfigFilesystemState(t *testing.T) {
	confName, storagePath, cleanup := createCaptiveCoreConfig(SimpleCaptiveCoreToml)
	defer cleanup()

	localParams := integration.MergeMaps(defaultCaptiveCoreParameters, map[string]string{
		"captive-core-storage-path":       storagePath,
		aurora.CaptiveCoreConfigPathName: confName,
	})
	testConfig := integration.GetTestConfig()
	testConfig.AuroraIngestParameters = localParams
	test := integration.NewTest(t, *testConfig)

	err := test.StartAurora()
	assert.NoError(t, err)
	test.WaitForAurora()

	t.Run("disk state", func(t *testing.T) {
		validateCaptiveCoreDiskState(test, storagePath)
	})

	t.Run("no bucket dir", func(t *testing.T) {
		validateNoBucketDirPath(test, storagePath)
	})
}

func TestMaxAssetsForPathRequests(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		test := integration.NewTest(t, *integration.GetTestConfig())
		err := test.StartAurora()
		assert.NoError(t, err)
		test.WaitForAurora()
		assert.Equal(t, test.AuroraIngest().Config().MaxAssetsPerPathRequest, 15)
		test.Shutdown()
	})
	t.Run("set to 2", func(t *testing.T) {
		testConfig := integration.GetTestConfig()
		testConfig.AuroraIngestParameters = map[string]string{"max-assets-per-path-request": "2"}
		test := integration.NewTest(t, *testConfig)
		err := test.StartAurora()
		assert.NoError(t, err)
		test.WaitForAurora()
		assert.Equal(t, test.AuroraIngest().Config().MaxAssetsPerPathRequest, 2)
		test.Shutdown()
	})
}

func TestMaxPathFindingRequests(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		test := integration.NewTest(t, *integration.GetTestConfig())
		err := test.StartAurora()
		assert.NoError(t, err)
		test.WaitForAurora()
		assert.Equal(t, test.AuroraIngest().Config().MaxPathFindingRequests, uint(0))
		_, ok := test.AuroraIngest().Paths().(simplepath.InMemoryFinder)
		assert.True(t, ok)
		test.Shutdown()
	})
	t.Run("set to 5", func(t *testing.T) {
		testConfig := integration.GetTestConfig()
		testConfig.AuroraIngestParameters = map[string]string{"max-path-finding-requests": "5"}
		test := integration.NewTest(t, *testConfig)
		err := test.StartAurora()
		assert.NoError(t, err)
		test.WaitForAurora()
		assert.Equal(t, test.AuroraIngest().Config().MaxPathFindingRequests, uint(5))
		finder, ok := test.AuroraIngest().Paths().(*paths.RateLimitedFinder)
		assert.True(t, ok)
		assert.Equal(t, finder.Limit(), 5)
		test.Shutdown()
	})
}

func TestDisablePathFinding(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		test := integration.NewTest(t, *integration.GetTestConfig())
		err := test.StartAurora()
		assert.NoError(t, err)
		test.WaitForAurora()
		assert.Equal(t, test.AuroraIngest().Config().MaxPathFindingRequests, uint(0))
		_, ok := test.AuroraIngest().Paths().(simplepath.InMemoryFinder)
		assert.True(t, ok)
		test.Shutdown()
	})
	t.Run("set to true", func(t *testing.T) {
		testConfig := integration.GetTestConfig()
		testConfig.AuroraIngestParameters = map[string]string{"disable-path-finding": "true"}
		test := integration.NewTest(t, *testConfig)
		err := test.StartAurora()
		assert.NoError(t, err)
		test.WaitForAurora()
		assert.Nil(t, test.AuroraIngest().Paths())
		test.Shutdown()
	})
}

func TestDisableTxSub(t *testing.T) {
	t.Run("require hcnet-core-url when both DISABLE_TX_SUB=false and INGEST=false", func(t *testing.T) {
		localParams := integration.MergeMaps(networkParamArgs, map[string]string{
			aurora.NetworkFlagName:      "testnet",
			aurora.IngestFlagName:       "false",
			aurora.DisableTxSubFlagName: "false",
		})
		testConfig := integration.GetTestConfig()
		testConfig.AuroraIngestParameters = localParams
		testConfig.SkipCoreContainerCreation = true
		test := integration.NewTest(t, *testConfig)
		err := test.StartAurora()
		assert.ErrorContains(t, err, "cannot initialize Aurora: flag --hcnet-core-url cannot be empty")
		test.Shutdown()
	})
	t.Run("aurora starts successfully when DISABLE_TX_SUB=false, INGEST=false and hcnet-core-url is provided", func(t *testing.T) {
		localParams := integration.MergeMaps(networkParamArgs, map[string]string{
			aurora.NetworkFlagName:        "testnet",
			aurora.IngestFlagName:         "false",
			aurora.DisableTxSubFlagName:   "false",
			aurora.HcnetCoreURLFlagName: "http://localhost:11626",
		})
		testConfig := integration.GetTestConfig()
		testConfig.AuroraIngestParameters = localParams
		testConfig.SkipCoreContainerCreation = true
		test := integration.NewTest(t, *testConfig)
		err := test.StartAurora()
		assert.NoError(t, err)
		test.Shutdown()
	})
	t.Run("aurora starts successfully when DISABLE_TX_SUB=true and INGEST=true", func(t *testing.T) {
		testConfig := integration.GetTestConfig()
		testConfig.AuroraIngestParameters = map[string]string{
			"disable-tx-sub": "true",
			"ingest":         "true",
		}
		test := integration.NewTest(t, *testConfig)
		err := test.StartAurora()
		assert.NoError(t, err)
		test.WaitForAurora()
		test.Shutdown()
	})
	t.Run("do not require hcnet-core-url when both DISABLE_TX_SUB=true and INGEST=false", func(t *testing.T) {
		localParams := integration.MergeMaps(networkParamArgs, map[string]string{
			aurora.NetworkFlagName:      "testnet",
			aurora.IngestFlagName:       "false",
			aurora.DisableTxSubFlagName: "true",
		})
		testConfig := integration.GetTestConfig()
		testConfig.AuroraIngestParameters = localParams
		testConfig.SkipCoreContainerCreation = true
		test := integration.NewTest(t, *testConfig)
		err := test.StartAurora()
		assert.NoError(t, err)
		test.Shutdown()
	})
}

func TestDeprecatedOutputs(t *testing.T) {
	t.Run("deprecated output for ingestion filtering", func(t *testing.T) {
		originalStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w
		stdLog.SetOutput(os.Stderr)

		testConfig := integration.GetTestConfig()
		testConfig.AuroraIngestParameters = map[string]string{"exp-enable-ingestion-filtering": "false"}
		test := integration.NewTest(t, *testConfig)
		err := test.StartAurora()
		assert.NoError(t, err)
		test.WaitForAurora()

		// Use a wait group to wait for the goroutine to finish before proceeding
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.Close(); err != nil {
				t.Errorf("Failed to close Stdout")
				return
			}
		}()

		outputBytes, _ := io.ReadAll(r)
		wg.Wait() // Wait for the goroutine to finish before proceeding
		_ = r.Close()
		os.Stderr = originalStderr

		assert.Contains(t, string(outputBytes), "DEPRECATED - No ingestion filter rules are defined by default, which equates to "+
			"no filtering of historical data. If you have never added filter rules to this deployment, then no further action is needed. "+
			"If you have defined ingestion filter rules previously but disabled filtering overall by setting the env variable EXP_ENABLE_INGESTION_FILTERING=false, "+
			"then you should now delete the filter rules using the Aurora Admin API to achieve the same no-filtering result. Remove usage of this variable in all cases.")
	})
	t.Run("deprecated output for command-line flags", func(t *testing.T) {
		originalStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w
		stdLog.SetOutput(os.Stderr)

		config, flags := aurora.Flags()

		auroraCmd := &cobra.Command{
			Use:           "aurora",
			Short:         "Client-facing api server for the Hcnet network",
			SilenceErrors: true,
			SilenceUsage:  true,
			Long:          "Client-facing API server for the Hcnet network.",
			RunE: func(cmd *cobra.Command, args []string) error {
				_, err := aurora.NewAppFromFlags(config, flags)
				if err != nil {
					return err
				}
				return nil
			},
		}

		auroraCmd.SetArgs([]string{"--disable-tx-sub=true"})
		if err := flags.Init(auroraCmd); err != nil {
			fmt.Println(err)
		}
		if err := auroraCmd.Execute(); err != nil {
			fmt.Println(err)
		}

		// Use a wait group to wait for the goroutine to finish before proceeding
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.Close(); err != nil {
				t.Errorf("Failed to close Stdout")
				return
			}
		}()

		outputBytes, _ := io.ReadAll(r)
		wg.Wait() // Wait for the goroutine to finish before proceeding
		_ = r.Close()
		os.Stderr = originalStderr

		assert.Contains(t, string(outputBytes), "DEPRECATED - the use of command-line flags: "+
			"[--disable-tx-sub], has been deprecated in favor of environment variables. Please consult our "+
			"Configuring section in the developer documentation on how to use them - "+
			"https://developers.hcnet.org/docs/run-api-server/configuring")
	})
	t.Run("deprecated output for --captive-core-use-db", func(t *testing.T) {
		originalStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w
		stdLog.SetOutput(os.Stderr)

		testConfig := integration.GetTestConfig()
		testConfig.AuroraIngestParameters = map[string]string{"captive-core-use-db": "false"}
		test := integration.NewTest(t, *testConfig)
		err := test.StartAurora()
		assert.NoError(t, err)
		test.WaitForAurora()

		// Use a wait group to wait for the goroutine to finish before proceeding
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := w.Close(); err != nil {
				t.Errorf("Failed to close Stdout")
				return
			}
		}()

		outputBytes, _ := io.ReadAll(r)
		wg.Wait() // Wait for the goroutine to finish before proceeding
		_ = r.Close()
		os.Stderr = originalStderr

		assert.Contains(t, string(outputBytes), "The usage of the flag --captive-core-use-db has been deprecated. "+
			"Setting it to false to achieve in-memory functionality on captive core will be removed in "+
			"future releases. We recommend removing usage of this flag now in preparation.")
	})
}

func TestGlobalFlagsOutput(t *testing.T) {

	// verify Help and Usage output from cli, both help and usage output follow the same
	// output rules of no globals when sub-comands exist, and only relevant globals
	// when down to leaf node command.

	dbParams := []string{"--max-db-connections", "--db-url"}
	// the space after '--ingest' is intentional to ensure correct matching behavior to
	// help output, as other flags also start with same prefix.
	apiParams := []string{"--port ", "--per-hour-rate-limit", "--ingest ", "sentry-dsn"}
	ingestionParams := []string{"--hcnet-core-binary-path", "--history-archive-urls", "--ingest-state-verification-checkpoint-frequency"}
	allParams := append(apiParams, append(dbParams, ingestionParams...)...)

	testCases := []struct {
		auroraHelpCommand          []string
		helpPrintedGlobalParams     []string
		helpPrintedSubCommandParams []string
		helpSkippedGlobalParams     []string
	}{
		{
			auroraHelpCommand:          []string{"ingest", "trigger-state-rebuild", "-h"},
			helpPrintedGlobalParams:     dbParams,
			helpPrintedSubCommandParams: []string{},
			helpSkippedGlobalParams:     append(apiParams, ingestionParams...),
		},
		{
			auroraHelpCommand:          []string{"ingest", "verify-range", "-h"},
			helpPrintedGlobalParams:     append(dbParams, ingestionParams...),
			helpPrintedSubCommandParams: []string{"--verify-state", "--from"},
			helpSkippedGlobalParams:     apiParams,
		},
		{
			auroraHelpCommand:          []string{"db", "reingest", "range", "-h"},
			helpPrintedGlobalParams:     append(dbParams, ingestionParams...),
			helpPrintedSubCommandParams: []string{"--parallel-workers", "--force"},
			helpSkippedGlobalParams:     apiParams,
		},
		{
			auroraHelpCommand:          []string{"db", "reingest", "range"},
			helpPrintedGlobalParams:     append(dbParams, ingestionParams...),
			helpPrintedSubCommandParams: []string{"--parallel-workers", "--force"},
			helpSkippedGlobalParams:     apiParams,
		},
		{
			auroraHelpCommand:          []string{"db", "fill-gaps", "-h"},
			helpPrintedGlobalParams:     append(dbParams, ingestionParams...),
			helpPrintedSubCommandParams: []string{"--parallel-workers", "--force"},
			helpSkippedGlobalParams:     apiParams,
		},
		{
			auroraHelpCommand:          []string{"db", "migrate", "up", "-h"},
			helpPrintedGlobalParams:     dbParams,
			helpPrintedSubCommandParams: []string{},
			helpSkippedGlobalParams:     append(apiParams, ingestionParams...),
		},
		{
			auroraHelpCommand:          []string{"db", "-h"},
			helpPrintedGlobalParams:     []string{},
			helpPrintedSubCommandParams: []string{},
			helpSkippedGlobalParams:     allParams,
		},
		{
			auroraHelpCommand:          []string{"db"},
			helpPrintedGlobalParams:     []string{},
			helpPrintedSubCommandParams: []string{},
			helpSkippedGlobalParams:     allParams,
		},
		{
			auroraHelpCommand:          []string{"-h"},
			helpPrintedGlobalParams:     []string{},
			helpPrintedSubCommandParams: []string{},
			helpSkippedGlobalParams:     allParams,
		},
		{
			auroraHelpCommand:          []string{"db", "reingest", "-h"},
			helpPrintedGlobalParams:     []string{},
			helpPrintedSubCommandParams: []string{},
			helpSkippedGlobalParams:     apiParams,
		},
		{
			auroraHelpCommand:          []string{"db", "reingest"},
			helpPrintedGlobalParams:     []string{},
			helpPrintedSubCommandParams: []string{},
			helpSkippedGlobalParams:     apiParams,
		},
		{
			auroraHelpCommand:          []string{"serve", "-h"},
			helpPrintedGlobalParams:     allParams,
			helpPrintedSubCommandParams: []string{},
			helpSkippedGlobalParams:     []string{},
		},
		{
			auroraHelpCommand:          []string{"record-metrics", "-h"},
			helpPrintedGlobalParams:     []string{"--admin-port"},
			helpPrintedSubCommandParams: []string{},
			helpSkippedGlobalParams:     allParams,
		},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("Aurora command line parameter %v", testCase.auroraHelpCommand), func(t *testing.T) {
			auroracmd.RootCmd.SetArgs(testCase.auroraHelpCommand)
			var writer io.Writer = &bytes.Buffer{}
			auroracmd.RootCmd.SetOutput(writer)
			auroracmd.RootCmd.Execute()

			output := writer.(*bytes.Buffer).String()
			for _, requiredParam := range testCase.helpPrintedSubCommandParams {
				assert.Contains(t, output, requiredParam, testCase.auroraHelpCommand)
			}
			for _, requiredParam := range testCase.helpPrintedGlobalParams {
				assert.Contains(t, output, requiredParam, testCase.auroraHelpCommand)
			}
			for _, skippedParam := range testCase.helpSkippedGlobalParams {
				assert.NotContains(t, output, skippedParam, testCase.auroraHelpCommand)
			}
		})
	}
}

// validateNoBucketDirPath ensures the Hcnet Core auto-generated configuration
// file doesn't contain the BUCKET_DIR_PATH entry, which is forbidden when using
// Captive Core.
//
// Pass "rootDirectory" set to whatever it is you pass to
// "--captive-core-storage-path".
func validateNoBucketDirPath(itest *integration.Test, rootDir string) {
	tt := assert.New(itest.CurrentTest())

	coreConf := path.Join(rootDir, "captive-core", "hcnet-core.conf")
	tt.FileExists(coreConf)

	result, err := ioutil.ReadFile(coreConf)
	tt.NoError(err)

	bucketPathSet := strings.Contains(string(result), "BUCKET_DIR_PATH")
	tt.False(bucketPathSet)
}

// validateCaptiveCoreDiskState ensures that running Captive Core creates a
// sensible directory structure.
//
// Pass "rootDirectory" set to whatever it is you pass to
// "--captive-core-storage-path".
func validateCaptiveCoreDiskState(itest *integration.Test, rootDir string) {
	tt := assert.New(itest.CurrentTest())

	storageDir := path.Join(rootDir, "captive-core")
	coreConf := path.Join(storageDir, "hcnet-core.conf")

	tt.DirExists(rootDir)
	tt.DirExists(storageDir)
	tt.FileExists(coreConf)
}

// createCaptiveCoreConfig will create a temporary TOML config with the
// specified contents as well as a temporary storage directory. You should
// `defer` the returned function to clean these up when you're done.
func createCaptiveCoreConfig(contents string) (string, string, func()) {
	tomlFile, err := ioutil.TempFile("", "captive-core-test-*.toml")
	defer tomlFile.Close()
	if err != nil {
		panic(err)
	}

	_, err = tomlFile.WriteString(contents)
	if err != nil {
		panic(err)
	}

	storagePath, err := os.MkdirTemp("", "captive-core-test-*-storage")
	if err != nil {
		panic(err)
	}

	filename := tomlFile.Name()
	return filename, storagePath, func() {
		os.Remove(filename)
		os.RemoveAll(storagePath)
	}
}
