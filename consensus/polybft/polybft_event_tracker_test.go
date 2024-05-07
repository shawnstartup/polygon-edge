package polybft

import (
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	edgeTracker "github.com/0xPolygon/polygon-edge/tracker"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/ethgo"
)

var _ edgeTracker.EventSubscription = (*mockEventSubscriber)(nil)

type mockEventSubscriber struct {
	logs []*ethgo.Log
}

func (m *mockEventSubscriber) AddLog(log *ethgo.Log) {
	m.logs = append(m.logs, log)
}

var _ BlockProvider = (*mockProvider)(nil)

type mockProvider struct {
	mock.Mock
}

// GetBlockByHash implements tracker.Provider.
func (m *mockProvider) GetBlockByHash(hash ethgo.Hash, full bool) (*ethgo.Block, error) {
	args := m.Called(hash, full)

	return1 := args.Get(0)

	if return1 != nil {
		return return1.(*ethgo.Block), args.Error(1)
	}

	return nil, args.Error(1)
}

// GetBlockByNumber implements tracker.Provider.
func (m *mockProvider) GetBlockByNumber(i ethgo.BlockNumber, full bool) (*ethgo.Block, error) {
	args := m.Called(i, full)

	return1 := args.Get(0)

	if return1 != nil {
		return return1.(*ethgo.Block), args.Error(1)
	}

	return nil, args.Error(1)
}

// GetLogs implements tracker.Provider.
func (m *mockProvider) GetLogs(filter *ethgo.LogFilter) ([]*ethgo.Log, error) {
	args := m.Called(filter)

	return1 := args.Get(0)

	if return1 != nil {
		return return1.([]*ethgo.Log), args.Error(1)
	}

	return nil, args.Error(1)
}

func TestEventTracker_getNewState(t *testing.T) {
	t.Parallel()

	t.Run("Add block by block - regular situation - no confirmed blocks", func(t *testing.T) {
		tracker, err := NewPolybftEventTracker(createTestTrackerConfig(t, 10, 10, 1000))

		require.NoError(t, err)

		// add some blocks, but don't go to confirmation level
		for i := uint64(1); i <= tracker.config.NumBlockConfirmations; i++ {
			require.NoError(t, tracker.trackBlock(
				&ethgo.Block{
					Number:     i,
					Hash:       ethgo.Hash{byte(i)},
					ParentHash: ethgo.Hash{byte(i - 1)},
				}))
		}

		// check that we have correct number of cached blocks
		require.Len(t, tracker.blockContainer.blocks, int(tracker.config.NumBlockConfirmations))
		require.Len(t, tracker.blockContainer.numToHashMap, int(tracker.config.NumBlockConfirmations))

		// check that we have no confirmed blocks
		require.Nil(t, tracker.blockContainer.GetConfirmedBlocks(tracker.config.NumBlockConfirmations))

		// check that the last processed block is 0, since we did not have any confirmed blocks
		require.Equal(t, uint64(0), tracker.blockContainer.LastProcessedBlockLocked())
		lastProcessedBlockInStore, err := tracker.config.Store.GetLastProcessedBlock()
		require.NoError(t, err)
		require.Equal(t, uint64(0), lastProcessedBlockInStore)

		// check that the last cached block is as expected
		require.Equal(t, tracker.config.NumBlockConfirmations, tracker.blockContainer.LastCachedBlock())
	})
}

func createTestTrackerConfig(t *testing.T, numBlockConfirmations, batchSize, maxBacklogSize uint64) *PolybftTrackerConfig {
	var stateSyncEvent contractsapi.StateSyncedEvent

	return &PolybftTrackerConfig{
		RpcEndpoint:           "http://some-rpc-url.com",
		StartBlockFromConfig:  0,
		NumBlockConfirmations: numBlockConfirmations,
		SyncBatchSize:         batchSize,
		MaxBacklogSize:        maxBacklogSize,
		PollInterval:          2 * time.Second,
		Logger:                hclog.NewNullLogger(),
		LogFilter: map[ethgo.Address][]ethgo.Hash{
			ethgo.ZeroAddress: {stateSyncEvent.Sig()},
		},
		Store:           newTestTrackerStore(t),
		EventSubscriber: new(mockEventSubscriber),
		BlockProvider:   new(mockProvider),
	}
}
