package db_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	dbbadger "github.com/tdex-network/tdex-daemon/internal/infrastructure/storage/db/badger"
	"github.com/tdex-network/tdex-daemon/internal/infrastructure/storage/db/inmemory"
)

var (
	fee = int64(25)
)

func TestMarketRepositoryImplementations(t *testing.T) {
	repositories := createMarketRepositories(t)

	for i := range repositories {
		repo := repositories[i]

		t.Run(repo.Name, func(t *testing.T) {
			t.Parallel()

			t.Run("testGetOrCreateMarket", func(t *testing.T) {
				t.Parallel()
				testGetOrCreateMarket(t, repo)
			})

			t.Run("testGetMarketByAccount", func(t *testing.T) {
				t.Parallel()
				testGetMarketByAccount(t, repo)
			})

			t.Run("testGetMarketByAsset", func(t *testing.T) {
				t.Parallel()
				testGetMarketByAsset(t, repo)
			})

			t.Run("testGetAllMarkets", func(t *testing.T) {
				t.Parallel()
				testGetAllMarkets(t, repo)
			})

			t.Run("testOpenCloseMarket", func(t *testing.T) {
				t.Parallel()
				testOpenCloseMarket(t, repo)
			})
		})
	}
}

func testGetOrCreateMarket(t *testing.T, repo marketRepository) {
	// to create a market is mandatory to specify the account index, the asset
	// pair and the fee
	accountIndex := uint64(5)
	marketBaseAsset := "0000000000000000000000000000000000000000000000000000000000000000"
	marketQuoteAsset := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	marketRepo := repo.Repository
	ctx := context.Background()

	market, _ := domain.NewMarket(
		accountIndex, marketBaseAsset, marketQuoteAsset, fee,
	)
	newMarket, err := marketRepo.GetOrCreateMarket(ctx, market)
	require.NoError(t, err)
	require.NotNil(t, newMarket)
	require.Equal(t, accountIndex, newMarket.AccountIndex)
	require.Equal(t, marketBaseAsset, newMarket.BaseAsset)
	require.Equal(t, marketQuoteAsset, newMarket.QuoteAsset)
	require.Equal(t, fee, newMarket.Fee)

	// to retrieve an existing market is enough to specify just the AccountIndex
	existingMarket, err := marketRepo.GetOrCreateMarket(
		ctx, &domain.Market{AccountIndex: accountIndex},
	)
	require.NoError(t, err)
	require.NotNil(t, existingMarket)
	require.Exactly(t, newMarket, existingMarket)
}

func testGetMarketByAccount(t *testing.T, repo marketRepository) {
	accountIndex := uint64(6)
	baseAsset := "0000000000000000000000000000000000000000000000000000000000000000"
	quoteAsset := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	marketRepo := repo.Repository
	ctx := context.Background()

	market, err := marketRepo.GetMarketByAccount(ctx, accountIndex)
	require.NoError(t, err)
	require.Nil(t, market)

	market, _ = domain.NewMarket(
		accountIndex, baseAsset, quoteAsset, fee,
	)
	_, err = marketRepo.GetOrCreateMarket(ctx, market)
	require.NoError(t, err)

	market, err = marketRepo.GetMarketByAccount(ctx, accountIndex)
	require.NoError(t, err)
	require.NotNil(t, market)
}

func testGetMarketByAsset(t *testing.T, repo marketRepository) {
	accountIndex := uint64(7)
	baseAsset := "0000000000000000000000000000000000000000000000000000000000000000"
	quoteAsset := "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	marketRepo := repo.Repository
	ctx := context.Background()

	mkt, mktAccountIndex, err := marketRepo.GetMarketByAssets(ctx, baseAsset, quoteAsset)
	require.NoError(t, err)
	require.Nil(t, mkt)
	require.Equal(t, -1, mktAccountIndex)

	market, _ := domain.NewMarket(
		accountIndex, baseAsset, quoteAsset, fee,
	)
	_, err = marketRepo.GetOrCreateMarket(ctx, market)
	require.NoError(t, err)

	mkt, mktAccountIndex, err = marketRepo.GetMarketByAssets(ctx, baseAsset, quoteAsset)
	require.NoError(t, err)
	require.NotNil(t, mkt)
	require.Equal(t, int(accountIndex), mktAccountIndex)
}

func testGetAllMarkets(t *testing.T, repo marketRepository) {
	marketRepo := repo.Repository
	ctx := context.Background()

	_, err := marketRepo.GetAllMarkets(ctx)
	require.NoError(t, err)
}

func testOpenCloseMarket(t *testing.T, repo marketRepository) {
	accountIndex := uint64(8)
	baseAsset := "0000000000000000000000000000000000000000000000000000000000000000"
	quoteAsset := "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	marketRepo := repo.Repository
	ctx := context.Background()

	market, _ := domain.NewMarket(accountIndex, baseAsset, quoteAsset, fee)
	_, err := marketRepo.GetOrCreateMarket(ctx, market)
	require.NoError(t, err)

	err = marketRepo.OpenMarket(ctx, accountIndex)
	require.NoError(t, err)

	openMarkets, err := marketRepo.GetTradableMarkets(ctx)
	require.NotNil(t, openMarkets)
	require.True(t, len(openMarkets) > 0)

	err = marketRepo.CloseMarket(ctx, accountIndex)
	require.NoError(t, err)

	openMarkets, err = marketRepo.GetTradableMarkets(ctx)
	require.NoError(t, err)
	require.Len(t, openMarkets, 0)
}

func testUpdatePrices(t *testing.T, repo marketRepository) {
	accountIndex := uint64(9)
	marketRepo := repo.Repository
	ctx := context.Background()

	market, err := marketRepo.GetOrCreateMarket(
		ctx,
		&domain.Market{AccountIndex: accountIndex, Fee: fee},
	)
	require.NoError(t, err)
	require.True(t, market.Price.AreZero())

	err = marketRepo.UpdatePrices(
		ctx,
		accountIndex,
		domain.Prices{
			BasePrice:  decimal.NewFromFloat(0.00002),
			QuotePrice: decimal.NewFromInt(50000),
		},
	)
	require.NoError(t, err)

	market, err = marketRepo.GetOrCreateMarket(
		ctx,
		&domain.Market{AccountIndex: accountIndex},
	)
	require.NoError(t, err)
	require.False(t, market.Price.AreZero())
}

func createMarketRepositories(t *testing.T) []marketRepository {
	inmemoryDBManager := inmemory.NewRepoManager()
	badgerDBManager, err := dbbadger.NewRepoManager("", nil)
	require.NoError(t, err)

	return []marketRepository{
		{
			Name:       "badger",
			DBManager:  badgerDBManager,
			Repository: badgerDBManager.MarketRepository(),
		},
		{
			Name:       "inmemory",
			DBManager:  inmemoryDBManager,
			Repository: inmemoryDBManager.MarketRepository(),
		},
	}
}

type marketRepository struct {
	Name       string
	DBManager  ports.RepoManager
	Repository domain.MarketRepository
}

func mockMarketFunds(baseAsset, quoteAsset string) []domain.OutpointWithAsset {
	return []domain.OutpointWithAsset{
		{
			Asset: baseAsset,
			Txid:  "0000000000000000000000000000000000000000000000000000000000000000",
			Vout:  0,
		},
		{
			Asset: quoteAsset,
			Txid:  "0000000000000000000000000000000000000000000000000000000000000000",
			Vout:  1,
		},
	}
}
