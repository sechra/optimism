package fault

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type gameAgent interface {
	ProgressGame(ctx context.Context) bool
}

type gameCreator func(address common.Address) (gameAgent, error)
type blockNumberFetcher func(ctx context.Context) (uint64, error)

// gameSource loads information about the games available to play
type gameSource interface {
	FetchAllGamesAtBlock(ctx context.Context, blockNumber *big.Int) ([]FaultDisputeGame, error)
}

type gameMonitor struct {
	logger           log.Logger
	source           gameSource
	createGame       gameCreator
	fetchBlockNumber blockNumberFetcher
	games            map[common.Address]gameAgent
}

func newGameMonitor(logger log.Logger, fetchBlockNumber blockNumberFetcher, source gameSource, createGame gameCreator) *gameMonitor {
	return &gameMonitor{
		logger:           logger,
		source:           source,
		createGame:       createGame,
		fetchBlockNumber: fetchBlockNumber,
		games:            make(map[common.Address]gameAgent),
	}
}

func (m *gameMonitor) progressGames(ctx context.Context) error {
	blockNum, err := m.fetchBlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to load current block number: %w", err)
	}
	games, err := m.source.FetchAllGamesAtBlock(ctx, new(big.Int).SetUint64(blockNum))
	if err != nil {
		return fmt.Errorf("failed to load games: %w", err)
	}
	for _, game := range games {
		if err := m.progressGame(ctx, game); err != nil {
			m.logger.Error("Error while progressing game", "game", game.Proxy, "err", err)
		}
	}
	return nil
}

func (m *gameMonitor) progressGame(ctx context.Context, gameData FaultDisputeGame) error {
	game, ok := m.games[gameData.Proxy]
	if !ok {
		newGame, err := m.createGame(gameData.Proxy)
		if err != nil {
			return fmt.Errorf("failed to progress game %v: %w", gameData.Proxy, err)
		}
		m.games[gameData.Proxy] = newGame
		game = newGame
	}
	game.ProgressGame(ctx)
	return nil
}

func (m *gameMonitor) MonitorGames(ctx context.Context) error {
	m.logger.Info("Monitoring fault dispute games")

	for {
		err := m.progressGames(ctx)
		if err != nil {
			m.logger.Error("Failed to progress games", "err", err)
		}
		select {
		case <-time.After(300 * time.Millisecond):
		// Continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
