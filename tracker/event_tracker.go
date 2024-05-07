package tracker

import (
	"context"
	"time"

	"github.com/0xPolygon/polygon-edge/helper/common"
	hcf "github.com/hashicorp/go-hclog"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/blocktracker"
	"github.com/umbracle/ethgo/jsonrpc"
	"github.com/umbracle/ethgo/tracker"
)

const minBlockMaxBacklog = 96

type EventSubscription interface {
	AddLog(log *ethgo.Log)
}

type EventTracker struct {
	dbPath                string
	rpcEndpoint           string
	contractAddr          ethgo.Address
	startBlock            uint64
	subscriber            EventSubscription
	logger                hcf.Logger
	numBlockConfirmations uint64 // minimal number of child blocks required for the parent block to be considered final
	pollInterval          common.Duration
}

func NewEventTracker(
	dbPath string,
	rpcEndpoint string,
	contractAddr ethgo.Address,
	subscriber EventSubscription,
	numBlockConfirmations uint64,
	startBlock uint64,
	logger hcf.Logger,
	pollInterval common.Duration,
) *EventTracker {
	return &EventTracker{
		dbPath:                dbPath,
		rpcEndpoint:           rpcEndpoint,
		contractAddr:          contractAddr,
		subscriber:            subscriber,
		numBlockConfirmations: numBlockConfirmations,
		startBlock:            startBlock,
		logger:                logger.Named("event_tracker"),
		pollInterval:          pollInterval,
	}
}

func (e *EventTracker) Start(ctx context.Context) error {
	e.logger.Info("Start tracking events",
		"contract", e.contractAddr,
		"JSON RPC address", e.rpcEndpoint,
		"num block confirmations", e.numBlockConfirmations,
		"start block", e.startBlock,
		"poll interval", e.pollInterval)

	provider, err := jsonrpc.NewClient(e.rpcEndpoint)
	if err != nil {
		return err
	}

	store, err := NewEventTrackerStore(e.dbPath, e.numBlockConfirmations, e.subscriber, e.logger)
	if err != nil {
		return err
	}

	blockMaxBacklog := e.numBlockConfirmations * 2
	if blockMaxBacklog < minBlockMaxBacklog {
		blockMaxBacklog = minBlockMaxBacklog
	}

	jsonBlockTracker := blocktracker.NewJSONBlockTracker(provider.Eth())
	jsonBlockTracker.PollInterval = e.pollInterval.Duration
	blockTracker := blocktracker.NewBlockTracker(
		provider.Eth(),
		blocktracker.WithBlockMaxBacklog(blockMaxBacklog),
		blocktracker.WithTracker(jsonBlockTracker),
	)

	go func() {
		<-ctx.Done()
		blockTracker.Close()
		store.Close()
	}()

	// Init and start block tracker concurrently, retrying indefinitely
	go common.RetryForever(ctx, time.Second, func(context.Context) error {
		start := time.Now().UTC()

		if err := blockTracker.Init(); err != nil {
			e.logger.Error("failed to init blocktracker", "error", err)

			return err
		}

		elapsed := time.Now().UTC().Sub(start) // Calculate the elapsed time

		if err := blockTracker.Start(); err != nil {
			e.logger.Error("failed to start blocktracker", "error", err)

			return err
		}

		e.logger.Info("Block tracker has been started", "max backlog", blockMaxBacklog, "init time", elapsed)

		return nil
	})

	tt, err := tracker.NewTracker(provider.Eth(),
		tracker.WithBatchSize(10),
		tracker.WithBlockTracker(blockTracker),
		tracker.WithStore(store),
		tracker.WithFilter(&tracker.FilterConfig{
			Async: true,
			Address: []ethgo.Address{
				e.contractAddr,
			},
			Start: e.startBlock,
		}),
	)
	if err != nil {
		return err
	}
	// Sync concurrently, retrying indefinitely
	go common.RetryForever(ctx, time.Second, func(context.Context) error {
		if err := tt.Sync(ctx); err != nil {
			e.logger.Error("failed to sync", "error", err)

			return err
		}

		return nil
	})

	return nil
}
