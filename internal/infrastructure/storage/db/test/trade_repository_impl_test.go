package db_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	dbbadger "github.com/tdex-network/tdex-daemon/internal/infrastructure/storage/db/badger"
	"github.com/tdex-network/tdex-daemon/internal/infrastructure/storage/db/inmemory"
)

func TestTradeRepositoryImplementations(t *testing.T) {
	repositories := createTradeRepositories(t)

	for i := range repositories {
		repo := repositories[i]

		t.Run(repo.Name, func(t *testing.T) {
			t.Parallel()

			t.Run("testGetOrCreateTrade", func(t *testing.T) {
				t.Parallel()
				testGetOrCreateTrade(t, repo)
			})

			t.Run("testGetAllTrades", func(t *testing.T) {
				t.Parallel()
				testGetAllTrades(t, repo)
			})

			t.Run("testGetAllTradesForMarket", func(t *testing.T) {
				t.Parallel()
				testGetAllTradesByMarket(t, repo)
			})

			t.Run("testGetCompletedTradesForMarket", func(t *testing.T) {
				t.Parallel()
				testGetCompletedTradesByMarket(t, repo)
			})

			t.Run("testGetTradeWithSwapAcceptID", func(t *testing.T) {
				t.Parallel()
				testGetTradeBySwapAcceptID(t, repo)
			})
		})
	}
}

func testGetOrCreateTrade(t *testing.T, repo tradeRepository) {
	tradeRepo := repo.Repository
	ctx := context.Background()

	trade, err := tradeRepo.GetOrCreateTrade(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, trade)
}

func testGetAllTrades(t *testing.T, repo tradeRepository) {
	tradeRepo := repo.Repository
	ctx := context.Background()

	_, err := tradeRepo.GetOrCreateTrade(ctx, nil)
	require.NoError(t, err)
	trades, err := tradeRepo.GetAllTrades(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(trades), 1)
}

func testGetAllTradesByMarket(t *testing.T, repo tradeRepository) {
	marketAsset := randomString(32)
	tradeRepo := repo.Repository
	ctx := context.Background()

	trade, err := tradeRepo.GetOrCreateTrade(ctx, nil)
	require.NoError(t, err)
	trades, err := tradeRepo.GetAllTradesByMarket(ctx, marketAsset)
	require.NoError(t, err)
	require.Len(t, trades, 0)

	tradeID := trade.ID
	err = tradeRepo.UpdateTrade(
		ctx,
		&tradeID,
		func(trade *domain.Trade) (*domain.Trade, error) {
			trade.MarketQuoteAsset = marketAsset
			return trade, nil
		},
	)
	require.NoError(t, err)

	trades, err = tradeRepo.GetAllTradesByMarket(ctx, marketAsset)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(trades), 1)
}

func testGetCompletedTradesByMarket(t *testing.T, repo tradeRepository) {
	marketAsset := randomString(32)
	tradeRepo := repo.Repository
	ctx := context.Background()

	trade, err := tradeRepo.GetOrCreateTrade(ctx, nil)
	require.NoError(t, err)

	tradeID := trade.ID
	err = tradeRepo.UpdateTrade(
		ctx,
		&tradeID,
		func(trade *domain.Trade) (*domain.Trade, error) {
			trade.MarketQuoteAsset = marketAsset
			return trade, nil
		},
	)
	trades, err := tradeRepo.GetCompletedTradesByMarket(ctx, marketAsset)
	require.NoError(t, err)
	require.Len(t, trades, 0)

	err = tradeRepo.UpdateTrade(
		ctx,
		&tradeID,
		func(trade *domain.Trade) (*domain.Trade, error) {
			trade.Status = domain.CompletedStatus
			return trade, nil
		},
	)
	require.NoError(t, err)

	trades, err = tradeRepo.GetCompletedTradesByMarket(ctx, marketAsset)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(trades), 1)

	trade, err = tradeRepo.GetOrCreateTrade(ctx, nil)
	require.NoError(t, err)

	err = tradeRepo.UpdateTrade(
		ctx,
		&trade.ID,
		func(trade *domain.Trade) (*domain.Trade, error) {
			trade.MarketQuoteAsset = marketAsset
			trade.Status = domain.SettledStatus
			return trade, nil
		},
	)
	require.NoError(t, err)

	trades, err = tradeRepo.GetCompletedTradesByMarket(ctx, marketAsset)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(trades), 2)
}

func testGetTradeBySwapAcceptID(t *testing.T, repo tradeRepository) {
	swapAcceptID := uuid.New().String()
	tradeRepo := repo.Repository
	ctx := context.Background()

	_, err := tradeRepo.GetOrCreateTrade(ctx, nil)
	require.NoError(t, err)

	trades, err := tradeRepo.GetTradeBySwapAcceptID(ctx, swapAcceptID)
	require.NoError(t, err)
	require.Nil(t, trades)
}

func testGetTradeByTxID(t *testing.T, repo tradeRepository) {
	txId := randomString(32)
	tradeRepo := repo.Repository
	ctx := context.Background()

	trade, err := tradeRepo.GetOrCreateTrade(ctx, nil)
	require.NoError(t, err)

	tradeId := trade.ID
	trades, err := tradeRepo.GetTradeByTxID(ctx, txId)
	require.NoError(t, err)
	require.Len(t, trades, 0)

	err = tradeRepo.UpdateTrade(
		ctx,
		&tradeId,
		func(trade *domain.Trade) (*domain.Trade, error) {
			trade.TxID = txId
			return trade, nil
		},
	)
	require.NoError(t, err)

	trade, err = tradeRepo.GetTradeByTxID(ctx, txId)
	require.NoError(t, err)
	require.NotNil(t, trade)
}

func createTradeRepositories(t *testing.T) []tradeRepository {
	inmemoryDBManager := inmemory.NewRepoManager()
	badgerDBManager, err := dbbadger.NewRepoManager("", nil)
	require.NoError(t, err)

	badgerDBManager.RegisterHandlerForTradeEvent(
		domain.TradeSettledEvent, func(event domain.TradeEvent) {
			require.Equal(t, event.EventType, domain.TradeSettledEvent)
			require.NotEmpty(t, event.Trade)
			require.True(t, event.Trade.IsSettled())
		},
	)

	return []tradeRepository{
		{
			Name:       "badger",
			DBManager:  badgerDBManager,
			Repository: badgerDBManager.TradeRepository(),
		},
		{
			Name:       "inmemory",
			DBManager:  inmemoryDBManager,
			Repository: inmemoryDBManager.TradeRepository(),
		},
	}
}

type tradeRepository struct {
	Name       string
	DBManager  ports.RepoManager
	Repository domain.TradeRepository
}

func randomString(len int) string {
	return hex.EncodeToString(randomBytes(32))
}

func randomBytes(len int) []byte {
	b := make([]byte, len)
	rand.Read(b)
	return b
}
