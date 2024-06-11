//lint:file-ignore U1001 Ignore all unused code, staticcheck doesn't understand testify/suite

package ingest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/HashCash-Consultants/go/ingest"
	"github.com/HashCash-Consultants/go/services/aurora/internal/db2/history"
	"github.com/HashCash-Consultants/go/services/aurora/internal/ingest/processors"
	"github.com/HashCash-Consultants/go/support/db"
	"github.com/HashCash-Consultants/go/xdr"
)

var _ auroraChangeProcessor = (*mockAuroraChangeProcessor)(nil)

type mockAuroraChangeProcessor struct {
	mock.Mock
}

func (m *mockAuroraChangeProcessor) Name() string {
	return "mockAuroraChangeProcessor"
}

func (m *mockAuroraChangeProcessor) ProcessChange(ctx context.Context, change ingest.Change) error {
	args := m.Called(ctx, change)
	return args.Error(0)
}

func (m *mockAuroraChangeProcessor) Commit(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

var _ auroraTransactionProcessor = (*mockAuroraTransactionProcessor)(nil)

type mockAuroraTransactionProcessor struct {
	mock.Mock
}

func (m *mockAuroraTransactionProcessor) Name() string {
	return "mockAuroraTransactionProcessor"
}

func (m *mockAuroraTransactionProcessor) ProcessTransaction(lcm xdr.LedgerCloseMeta, transaction ingest.LedgerTransaction) error {
	args := m.Called(lcm, transaction)
	return args.Error(0)
}

func (m *mockAuroraTransactionProcessor) Flush(ctx context.Context, session db.SessionInterface) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

type GroupChangeProcessorsTestSuiteLedger struct {
	suite.Suite
	ctx        context.Context
	processors *groupChangeProcessors
	processorA *mockAuroraChangeProcessor
	processorB *mockAuroraChangeProcessor
}

func TestGroupChangeProcessorsTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(GroupChangeProcessorsTestSuiteLedger))
}

func (s *GroupChangeProcessorsTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	s.processorA = &mockAuroraChangeProcessor{}
	s.processorB = &mockAuroraChangeProcessor{}
	s.processors = newGroupChangeProcessors([]auroraChangeProcessor{
		s.processorA,
		s.processorB,
	})
}

func (s *GroupChangeProcessorsTestSuiteLedger) TearDownTest() {
	s.processorA.AssertExpectations(s.T())
	s.processorB.AssertExpectations(s.T())
}

func (s *GroupChangeProcessorsTestSuiteLedger) TestProcessChangeFails() {
	change := ingest.Change{}
	s.processorA.
		On("ProcessChange", s.ctx, change).
		Return(errors.New("transient error")).Once()

	err := s.processors.ProcessChange(s.ctx, change)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error in *ingest.mockAuroraChangeProcessor.ProcessChange: transient error")
}

func (s *GroupChangeProcessorsTestSuiteLedger) TestProcessChangeSucceeds() {
	change := ingest.Change{}
	s.processorA.
		On("ProcessChange", s.ctx, change).
		Return(nil).Once()
	s.processorB.
		On("ProcessChange", s.ctx, change).
		Return(nil).Once()

	err := s.processors.ProcessChange(s.ctx, change)
	s.Assert().NoError(err)
}

func (s *GroupChangeProcessorsTestSuiteLedger) TestCommitFails() {
	s.processorA.
		On("Commit", s.ctx).
		Return(errors.New("transient error")).Once()

	err := s.processors.Commit(s.ctx)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error in *ingest.mockAuroraChangeProcessor.Commit: transient error")
}

func (s *GroupChangeProcessorsTestSuiteLedger) TestCommitSucceeds() {
	s.processorA.
		On("Commit", s.ctx).
		Return(nil).Once()
	s.processorB.
		On("Commit", s.ctx).
		Return(nil).Once()

	err := s.processors.Commit(s.ctx)
	s.Assert().NoError(err)
}

type GroupTransactionProcessorsTestSuiteLedger struct {
	suite.Suite
	ctx        context.Context
	processors *groupTransactionProcessors
	processorA *mockAuroraTransactionProcessor
	processorB *mockAuroraTransactionProcessor
	session    db.SessionInterface
}

func TestGroupTransactionProcessorsTestSuiteLedger(t *testing.T) {
	suite.Run(t, new(GroupTransactionProcessorsTestSuiteLedger))
}

func (s *GroupTransactionProcessorsTestSuiteLedger) SetupTest() {
	s.ctx = context.Background()
	statsProcessor := processors.NewStatsLedgerTransactionProcessor()

	tradesProcessor := processors.NewTradeProcessor(history.NewAccountLoaderStub().Loader,
		history.NewLiquidityPoolLoaderStub().Loader,
		history.NewAssetLoaderStub().Loader,
		&history.MockTradeBatchInsertBuilder{})

	s.processorA = &mockAuroraTransactionProcessor{}
	s.processorB = &mockAuroraTransactionProcessor{}
	s.processors = newGroupTransactionProcessors([]auroraTransactionProcessor{
		s.processorA,
		s.processorB,
	}, statsProcessor, tradesProcessor)
	s.session = &db.MockSession{}
}

func (s *GroupTransactionProcessorsTestSuiteLedger) TearDownTest() {
	s.processorA.AssertExpectations(s.T())
	s.processorB.AssertExpectations(s.T())
}

func (s *GroupTransactionProcessorsTestSuiteLedger) TestProcessTransactionFails() {
	transaction := ingest.LedgerTransaction{}
	closeMeta := xdr.LedgerCloseMeta{}
	s.processorA.
		On("ProcessTransaction", closeMeta, transaction).
		Return(errors.New("transient error")).Once()

	err := s.processors.ProcessTransaction(closeMeta, transaction)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error in *ingest.mockAuroraTransactionProcessor.ProcessTransaction: transient error")
}

func (s *GroupTransactionProcessorsTestSuiteLedger) TestProcessTransactionSucceeds() {
	transaction := ingest.LedgerTransaction{}
	closeMeta := xdr.LedgerCloseMeta{}
	s.processorA.
		On("ProcessTransaction", closeMeta, transaction).
		Return(nil).Once()
	s.processorB.
		On("ProcessTransaction", closeMeta, transaction).
		Return(nil).Once()

	err := s.processors.ProcessTransaction(closeMeta, transaction)
	s.Assert().NoError(err)
}

func (s *GroupTransactionProcessorsTestSuiteLedger) TestFlushFails() {
	s.processorA.
		On("Flush", s.ctx, s.session).
		Return(errors.New("transient error")).Once()

	err := s.processors.Flush(s.ctx, s.session)
	s.Assert().Error(err)
	s.Assert().EqualError(err, "error in *ingest.mockAuroraTransactionProcessor.Flush: transient error")
}

func (s *GroupTransactionProcessorsTestSuiteLedger) TestFlushSucceeds() {
	s.processorA.
		On("Flush", s.ctx, s.session).
		Return(nil).Once()
	s.processorB.
		On("Flush", s.ctx, s.session).
		Return(nil).Once()

	err := s.processors.Flush(s.ctx, s.session)
	s.Assert().NoError(err)
}
