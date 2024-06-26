package ledgerbackend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/HashCash-Consultants/go/support/compressxdr"
	"github.com/HashCash-Consultants/go/support/datastore"
	"github.com/HashCash-Consultants/go/xdr"
)

var partitionSize = uint32(64000)
var ledgerPerFileCount = uint32(1)

func createBufferedStorageBackendConfigForTesting() BufferedStorageBackendConfig {
	param := make(map[string]string)
	param["destination_bucket_path"] = "testURL"

	ledgerBatchConfig := datastore.LedgerBatchConfig{
		LedgersPerFile:    1,
		FilesPerPartition: 64000,
		FileSuffix:        ".xdr.gz",
	}

	dataStore := new(datastore.MockDataStore)

	return BufferedStorageBackendConfig{
		LedgerBatchConfig: ledgerBatchConfig,
		CompressionType:   compressxdr.GZIP,
		DataStore:         dataStore,
		BufferSize:        100,
		NumWorkers:        5,
		RetryLimit:        3,
		RetryWait:         time.Microsecond,
	}
}

func createBufferedStorageBackendForTesting() BufferedStorageBackend {
	config := createBufferedStorageBackendConfigForTesting()
	ledgerMetaArchive := datastore.NewLedgerMetaArchive("", 0, 0)
	decoder, _ := compressxdr.NewXDRDecoder(config.CompressionType, nil)

	return BufferedStorageBackend{
		config:            config,
		dataStore:         config.DataStore,
		ledgerMetaArchive: ledgerMetaArchive,
		decoder:           decoder,
	}
}

func createMockdataStore(t *testing.T, start, end, partitionSize, count uint32) *datastore.MockDataStore {
	mockDataStore := new(datastore.MockDataStore)
	partition := count*partitionSize - 1
	for i := start; i <= end; i = i + count {
		var objectName string
		var readCloser io.ReadCloser
		if count > 1 {
			endFileSeq := i + count - 1
			readCloser = createLCMBatchReader(i, endFileSeq, count)
			objectName = fmt.Sprintf("0-%d/%d-%d.xdr.gz", partition, i, endFileSeq)
		} else {
			readCloser = createLCMBatchReader(i, i, count)
			objectName = fmt.Sprintf("0-%d/%d.xdr.gz", partition, i)
		}
		mockDataStore.On("GetFile", mock.Anything, objectName).Return(readCloser, nil)
	}

	t.Cleanup(func() {
		mockDataStore.AssertExpectations(t)
	})

	return mockDataStore
}

func createLCMForTesting(start, end uint32) []xdr.LedgerCloseMeta {
	var lcmArray []xdr.LedgerCloseMeta
	for i := start; i <= end; i++ {
		lcmArray = append(lcmArray, datastore.CreateLedgerCloseMeta(i))
	}

	return lcmArray
}

func createTestLedgerCloseMetaBatch(startSeq, endSeq, count uint32) xdr.LedgerCloseMetaBatch {
	var ledgerCloseMetas []xdr.LedgerCloseMeta
	for i := uint32(0); i < count; i++ {
		ledgerCloseMetas = append(ledgerCloseMetas, datastore.CreateLedgerCloseMeta(startSeq+uint32(i)))
	}
	return xdr.LedgerCloseMetaBatch{
		StartSequence:    xdr.Uint32(startSeq),
		EndSequence:      xdr.Uint32(endSeq),
		LedgerCloseMetas: ledgerCloseMetas,
	}
}

func createLCMBatchReader(start, end, count uint32) io.ReadCloser {
	testData := createTestLedgerCloseMetaBatch(start, end, count)
	encoder, _ := compressxdr.NewXDREncoder(compressxdr.GZIP, testData)
	var buf bytes.Buffer
	encoder.WriteTo(&buf)
	capturedBuf := buf.Bytes()
	reader := bytes.NewReader(capturedBuf)
	return io.NopCloser(reader)
}

func TestNewBufferedStorageBackend(t *testing.T) {
	ctx := context.Background()
	config := createBufferedStorageBackendConfigForTesting()

	bsb, err := NewBufferedStorageBackend(ctx, config)
	assert.NoError(t, err)

	assert.Equal(t, bsb.dataStore, config.DataStore)
	assert.Equal(t, ".xdr.gz", bsb.config.LedgerBatchConfig.FileSuffix)
	assert.Equal(t, uint32(1), bsb.config.LedgerBatchConfig.LedgersPerFile)
	assert.Equal(t, uint32(64000), bsb.config.LedgerBatchConfig.FilesPerPartition)
	assert.Equal(t, uint32(100), bsb.config.BufferSize)
	assert.Equal(t, uint32(5), bsb.config.NumWorkers)
	assert.Equal(t, uint32(3), bsb.config.RetryLimit)
	assert.Equal(t, time.Microsecond, bsb.config.RetryWait)
}

func TestNewLedgerBuffer(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(7)
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 2
	bsb.config.BufferSize = 5
	ledgerRange := BoundedRange(startLedger, endLedger)
	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	ledgerBuffer, err := bsb.newLedgerBuffer(ledgerRange)
	assert.Eventually(t, func() bool { return len(ledgerBuffer.ledgerQueue) == 5 }, time.Second*5, time.Millisecond*50)
	assert.NoError(t, err)

	latestSeq, err := ledgerBuffer.getLatestLedgerSequence()
	assert.NoError(t, err)
	assert.Equal(t, uint32(7), latestSeq)
	assert.Equal(t, ledgerRange, ledgerBuffer.ledgerRange)
}

func TestBSBGetLatestLedgerSequence(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)
	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 3 }, time.Second*5, time.Millisecond*50)

	latestSeq, err := bsb.GetLatestLedgerSequence(ctx)
	assert.NoError(t, err)

	assert.Equal(t, uint32(5), latestSeq)
}

func TestBSBGetLedger_SingleLedgerPerFile(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	lcmArray := createLCMForTesting(startLedger, endLedger)
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 3 }, time.Second*5, time.Millisecond*50)

	lcm, err := bsb.GetLedger(ctx, uint32(3))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[0], lcm)
	lcm, err = bsb.GetLedger(ctx, uint32(4))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[1], lcm)
	lcm, err = bsb.GetLedger(ctx, uint32(5))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[2], lcm)
}

func TestCloudStorageGetLedger_MultipleLedgerPerFile(t *testing.T) {
	startLedger := uint32(2)
	endLedger := uint32(5)
	lcmArray := createLCMForTesting(startLedger, endLedger)
	bsb := createBufferedStorageBackendForTesting()
	ctx := context.Background()
	bsb.config.LedgerBatchConfig.LedgersPerFile = uint32(2)
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, 2)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 2 }, time.Second*5, time.Millisecond*50)

	lcm, err := bsb.GetLedger(ctx, uint32(2))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[0], lcm)
	lcm, err = bsb.GetLedger(ctx, uint32(3))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[1], lcm)
	lcm, err = bsb.GetLedger(ctx, uint32(4))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[2], lcm)
}

func TestBSBGetLedger_ErrorPreceedingLedger(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	lcmArray := createLCMForTesting(startLedger, endLedger)
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 3 }, time.Second*5, time.Millisecond*50)

	lcm, err := bsb.GetLedger(ctx, uint32(3))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[0], lcm)

	_, err = bsb.GetLedger(ctx, uint32(2))
	assert.EqualError(t, err, "requested sequence preceeds current LedgerRange")
}

func TestBSBGetLedger_NotPrepared(t *testing.T) {
	bsb := createBufferedStorageBackendForTesting()
	ctx := context.Background()

	_, err := bsb.GetLedger(ctx, uint32(3))
	assert.EqualError(t, err, "session is not prepared, call PrepareRange first")
}

func TestBSBGetLedger_SequenceNotInBatch(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 3 }, time.Second*5, time.Millisecond*50)

	_, err := bsb.GetLedger(ctx, uint32(2))
	assert.EqualError(t, err, "requested sequence preceeds current LedgerRange")

	_, err = bsb.GetLedger(ctx, uint32(6))
	assert.EqualError(t, err, "requested sequence beyond current LedgerRange")
}

func TestBSBPrepareRange(t *testing.T) {
	startLedger := uint32(2)
	endLedger := uint32(3)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 2 }, time.Second*5, time.Millisecond*50)

	assert.NotNil(t, bsb.prepared)

	// check alreadyPrepared
	err := bsb.PrepareRange(ctx, ledgerRange)
	assert.NoError(t, err)
	assert.NotNil(t, bsb.prepared)
}

func TestBSBIsPrepared_Bounded(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(5)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 3 }, time.Second*5, time.Millisecond*50)

	ok, err := bsb.IsPrepared(ctx, ledgerRange)
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = bsb.IsPrepared(ctx, BoundedRange(2, 4))
	assert.NoError(t, err)
	assert.False(t, ok)

	ok, err = bsb.IsPrepared(ctx, UnboundedRange(3))
	assert.NoError(t, err)
	assert.False(t, ok)

	ok, err = bsb.IsPrepared(ctx, UnboundedRange(2))
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestBSBIsPrepared_Unbounded(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(8)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 2
	bsb.config.BufferSize = 5
	ledgerRange := UnboundedRange(3)
	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 5 }, time.Second*5, time.Millisecond*50)

	ok, err := bsb.IsPrepared(ctx, ledgerRange)
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = bsb.IsPrepared(ctx, BoundedRange(3, 4))
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = bsb.IsPrepared(ctx, BoundedRange(2, 4))
	assert.NoError(t, err)
	assert.False(t, ok)

	ok, err = bsb.IsPrepared(ctx, UnboundedRange(4))
	assert.NoError(t, err)
	assert.True(t, ok)

	ok, err = bsb.IsPrepared(ctx, UnboundedRange(2))
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestBSBClose(t *testing.T) {
	startLedger := uint32(2)
	endLedger := uint32(3)
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 2 }, time.Second*5, time.Millisecond*50)

	err := bsb.Close()
	assert.NoError(t, err)
	assert.Equal(t, true, bsb.closed)

	_, err = bsb.GetLatestLedgerSequence(ctx)
	assert.EqualError(t, err, "BufferedStorageBackend is closed; cannot GetLatestLedgerSequence")

	_, err = bsb.GetLedger(ctx, 3)
	assert.EqualError(t, err, "BufferedStorageBackend is closed; cannot GetLedger")

	err = bsb.PrepareRange(ctx, ledgerRange)
	assert.EqualError(t, err, "BufferedStorageBackend is closed; cannot PrepareRange")

	_, err = bsb.IsPrepared(ctx, ledgerRange)
	assert.EqualError(t, err, "BufferedStorageBackend is closed; cannot IsPrepared")
}

func TestLedgerBufferInvariant(t *testing.T) {
	startLedger := uint32(3)
	endLedger := uint32(6)
	ctx := context.Background()
	lcmArray := createLCMForTesting(startLedger, endLedger)
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 2
	bsb.config.BufferSize = 2
	ledgerRange := BoundedRange(startLedger, endLedger)

	mockDataStore := createMockdataStore(t, startLedger, endLedger, partitionSize, ledgerPerFileCount)
	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 2 }, time.Second*5, time.Millisecond*50)

	// Buffer should have hit the BufferSize limit
	assert.Equal(t, 2, len(bsb.ledgerBuffer.ledgerQueue))

	lcm, err := bsb.GetLedger(ctx, uint32(3))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[0], lcm)
	lcm, err = bsb.GetLedger(ctx, uint32(4))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[1], lcm)

	// Buffer should fill up with remaining ledgers
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 2 }, time.Second*5, time.Millisecond*50)
	assert.Equal(t, 2, len(bsb.ledgerBuffer.ledgerQueue))

	lcm, err = bsb.GetLedger(ctx, uint32(5))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[2], lcm)

	// Buffer should only have the final ledger
	assert.Eventually(t, func() bool { return len(bsb.ledgerBuffer.ledgerQueue) == 1 }, time.Second*5, time.Millisecond*50)
	assert.Equal(t, 1, len(bsb.ledgerBuffer.ledgerQueue))

	lcm, err = bsb.GetLedger(ctx, uint32(6))
	assert.NoError(t, err)
	assert.Equal(t, lcmArray[3], lcm)

	// Buffer should be empty
	assert.Equal(t, 0, len(bsb.ledgerBuffer.ledgerQueue))
}

func TestLedgerBufferClose(t *testing.T) {
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 1
	bsb.config.BufferSize = 5
	ledgerRange := UnboundedRange(3)

	mockDataStore := new(datastore.MockDataStore)
	partition := ledgerPerFileCount*partitionSize - 1

	objectName := fmt.Sprintf("0-%d/%d.xdr.gz", partition, 3)
	afterPrepareRange := make(chan struct{})
	mockDataStore.On("GetFile", mock.Anything, objectName).Return(io.NopCloser(&bytes.Buffer{}), context.Canceled).Run(func(args mock.Arguments) {
		<-afterPrepareRange
		go bsb.ledgerBuffer.close()
	}).Once()

	t.Cleanup(func() {
		mockDataStore.AssertExpectations(t)
	})

	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))
	close(afterPrepareRange)

	bsb.ledgerBuffer.wg.Wait()

	_, err := bsb.GetLedger(ctx, 3)
	assert.EqualError(t, err, "failed getting next ledger batch from queue: context canceled")
}

func TestLedgerBufferBoundedObjectNotFound(t *testing.T) {
	ctx := context.Background()
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 1
	bsb.config.BufferSize = 5
	ledgerRange := BoundedRange(3, 5)

	mockDataStore := new(datastore.MockDataStore)
	partition := ledgerPerFileCount*partitionSize - 1

	objectName := fmt.Sprintf("0-%d/%d.xdr.gz", partition, 3)
	mockDataStore.On("GetFile", mock.Anything, objectName).Return(io.NopCloser(&bytes.Buffer{}), os.ErrNotExist).Once()
	t.Cleanup(func() {
		mockDataStore.AssertExpectations(t)
	})

	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))

	bsb.ledgerBuffer.wg.Wait()

	_, err := bsb.GetLedger(ctx, 3)
	assert.EqualError(t, err, "failed getting next ledger batch from queue: ledger object containing sequence 3 is missing: file does not exist")
}

func TestLedgerBufferUnboundedObjectNotFound(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 1
	bsb.config.BufferSize = 5
	ledgerRange := UnboundedRange(3)

	mockDataStore := new(datastore.MockDataStore)
	partition := ledgerPerFileCount*partitionSize - 1

	objectName := fmt.Sprintf("0-%d/%d.xdr.gz", partition, 3)
	iteration := &atomic.Int32{}
	cancelAfter := int32(bsb.config.RetryLimit) + 2
	mockDataStore.On("GetFile", mock.Anything, objectName).Return(io.NopCloser(&bytes.Buffer{}), os.ErrNotExist).Run(func(args mock.Arguments) {
		if iteration.Load() >= cancelAfter {
			cancel()
		}
		iteration.Add(1)
	})
	t.Cleanup(func() {
		mockDataStore.AssertExpectations(t)
	})

	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(ctx, ledgerRange))

	_, err := bsb.GetLedger(ctx, 3)
	assert.EqualError(t, err, "failed getting next ledger batch from queue: context canceled")
	assert.GreaterOrEqual(t, iteration.Load(), cancelAfter)
	assert.NoError(t, bsb.Close())
}

func TestLedgerBufferRetryLimit(t *testing.T) {
	bsb := createBufferedStorageBackendForTesting()
	bsb.config.NumWorkers = 1
	bsb.config.BufferSize = 5
	ledgerRange := UnboundedRange(3)

	mockDataStore := new(datastore.MockDataStore)
	partition := ledgerPerFileCount*partitionSize - 1

	objectName := fmt.Sprintf("0-%d/%d.xdr.gz", partition, 3)
	mockDataStore.On("GetFile", mock.Anything, objectName).
		Return(io.NopCloser(&bytes.Buffer{}), fmt.Errorf("transient error")).
		Times(int(bsb.config.RetryLimit) + 1)
	t.Cleanup(func() {
		mockDataStore.AssertExpectations(t)
	})

	bsb.dataStore = mockDataStore

	assert.NoError(t, bsb.PrepareRange(context.Background(), ledgerRange))

	bsb.ledgerBuffer.wg.Wait()

	_, err := bsb.GetLedger(context.Background(), 3)
	assert.EqualError(t, err, "failed getting next ledger batch from queue: maximum retries exceeded for downloading object containing sequence 3: transient error")
}
