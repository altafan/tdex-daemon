package application_test

import (
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tdex-network/tdex-daemon/internal/core/application"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	pbswap "github.com/tdex-network/tdex-protobuf/generated/go/swap"
)

var (
	tradeExpiryDuration = 120 * time.Second
	tradePriceSlippage  = decimal.NewFromFloat(0.1)
	tradeSatsPerByte    = 0.1

	marketQuotePrice = decimal.NewFromInt(30000)
	marketBasePrice  = decimal.NewFromInt(1).Div(marketQuotePrice)

	withFixedFees    = true
	withoutFixedFees = !withFixedFees
)

func TestMarketTrading(t *testing.T) {
	t.Run("without_fixed_fees", func(t *testing.T) {
		tradeSvc := newTradeService(withoutFixedFees)

		markets, err := tradeSvc.GetTradableMarkets(ctx)
		require.NoError(t, err)
		require.Len(t, markets, 1)

		market := markets[0].Market
		marketBalance, err := tradeSvc.GetMarketBalance(ctx, market)
		require.NoError(t, err)
		require.NotNil(t, marketBalance)
		require.Greater(t, int(marketBalance.Balance.BaseAmount), 0)
		require.Greater(t, int(marketBalance.Balance.QuoteAmount), 0)

		t.Run("buy LBTC fixed LBTC", func(t *testing.T) {
			t.Parallel()
			marketOrder(t, tradeSvc, market, application.TradeBuy, 0.1, marketBaseAsset)
		})
		t.Run("buy LBTC fixed USDT", func(t *testing.T) {
			t.Parallel()
			marketOrder(t, tradeSvc, market, application.TradeBuy, 900.0, marketQuoteAsset)
		})
		t.Run("sell LBTC fixed LBTC", func(t *testing.T) {
			t.Parallel()
			marketOrder(t, tradeSvc, market, application.TradeSell, 0.1, marketBaseAsset)
		})
		t.Run("sell LBTC fixed USDT", func(t *testing.T) {
			t.Parallel()
			marketOrder(t, tradeSvc, market, application.TradeSell, 900.0, marketQuoteAsset)
		})
	})

	t.Run("with_fixed_fees", func(t *testing.T) {
		tradeSvc := newTradeService(withFixedFees)

		markets, err := tradeSvc.GetTradableMarkets(ctx)
		require.NoError(t, err)
		require.Len(t, markets, 1)

		market := markets[0].Market
		marketBalance, err := tradeSvc.GetMarketBalance(ctx, market)
		require.NoError(t, err)
		require.NotNil(t, marketBalance)
		require.Greater(t, int(marketBalance.Balance.BaseAmount), 0)
		require.Greater(t, int(marketBalance.Balance.QuoteAmount), 0)

		t.Run("buy LBTC fixed LBTC", func(t *testing.T) {
			t.Parallel()
			marketOrder(t, tradeSvc, market, application.TradeBuy, 0.1, marketBaseAsset)
		})
		t.Run("buy LBTC fixed USDT", func(t *testing.T) {
			t.Parallel()
			marketOrder(t, tradeSvc, market, application.TradeBuy, 900.0, marketQuoteAsset)
		})
		t.Run("sell LBTC fixed LBTC", func(t *testing.T) {
			t.Parallel()
			marketOrder(t, tradeSvc, market, application.TradeSell, 0.1, marketBaseAsset)
		})
		t.Run("sell LBTC fixed USDT", func(t *testing.T) {
			t.Parallel()
			marketOrder(t, tradeSvc, market, application.TradeSell, 900.0, marketQuoteAsset)
		})
	})
}

func newTradeService(withFixedFees bool) application.TradeService {
	repoManager, wallet := newServices()
	mktRepo := repoManager.MarketRepository()
	// Add a market to the repo
	mkt, _ := domain.NewMarket(5, marketBaseAsset, marketQuoteAsset, marketFee)
	mktRepo.GetOrCreateMarket(ctx, mkt)
	mktRepo.UpdateMarket(ctx, mkt.AccountIndex, func(m *domain.Market) (*domain.Market, error) {
		m.ChangeBasePrice(marketBasePrice)
		m.ChangeQuotePrice(marketQuotePrice)
		if withFixedFees {
			m.ChangeFixedFee(650, 5000)
		}
		m.MakeTradable()
		return m, nil
	})
	// Mock wallet apis for market
	feeBalance := map[string]ports.Balance{
		nativeAsset: accountBalance{1000000, 1000000, 0},
	}
	wallet.(*mockedWallet).On("BalanceForAccount", mock.Anything, application.FeeAccount).Return(feeBalance, nil)
	marketBalance := map[string]ports.Balance{
		marketBaseAsset:  accountBalance{100000000, 100000000, 0},
		marketQuoteAsset: accountBalance{3000000000000, 3000000000000, 0},
	}
	wallet.(*mockedWallet).On("BalanceForAccount", mock.Anything, market.Name()).Return(marketBalance, nil)
	wallet.(*mockedWallet).On("FillSwapTransaction", mock.Anything, market.Name(), mock.Anything).Return(randomBase64(), nil, nil, nil, nil)
	wallet.(*mockedWallet).txManager.On("BroadcastTransaction", mock.Anything, mock.Anything).Return(randomHex(32), nil)
	return application.NewTradeService(
		repoManager, wallet, nil,
		tradeExpiryDuration, tradeSatsPerByte, tradePriceSlippage,
		feeBalanceThreshold,
	)
}

func marketOrder(
	t *testing.T,
	tradeSvc application.TradeService,
	market application.Market,
	tradeType int,
	btcAmount float64,
	asset string,
) {
	amount := uint64(btcAmount * math.Pow10(8))
	preview, err := tradeSvc.GetMarketPrice(ctx, market, tradeType, amount, asset)
	require.NoError(t, err)
	require.NotNil(t, preview)

	assetToSend := asset
	amountToSend := amount
	assetToReceive := preview.Asset
	amountToReceive := preview.Amount
	if tradeType == application.TradeSell && asset == marketQuoteAsset {
		assetToSend, assetToReceive = assetToReceive, assetToSend
		amountToSend, amountToReceive = amountToReceive, amountToSend
	}
	if tradeType == application.TradeBuy && asset == marketBaseAsset {
		assetToSend, assetToReceive = assetToReceive, assetToSend
		amountToSend, amountToReceive = amountToReceive, amountToSend
	}

	txProposal, txCompleted := randomBase64(), randomBase64()
	swapRequest := &pbswap.SwapRequest{
		Id:                randomId(),
		AssetP:            assetToSend,
		AmountP:           amountToSend,
		AssetR:            assetToReceive,
		AmountR:           amountToReceive,
		Transaction:       txProposal,
		InputBlindingKey:  make(map[string][]byte),
		OutputBlindingKey: make(map[string][]byte),
	}

	swapAccept, swapFail, expiryTimestamp, err := tradeSvc.TradePropose(ctx, market, tradeType, swapRequest)
	require.NoError(t, err)
	require.Nil(t, swapFail)
	require.NotNil(t, swapAccept)
	require.True(t, time.Now().Before(time.Unix(int64(expiryTimestamp), 0)))

	swapComplete := &pbswap.SwapComplete{
		Id:          randomId(),
		AcceptId:    swapAccept.GetId(),
		Transaction: txCompleted,
	}

	// mock network overhead
	time.Sleep(200 * time.Millisecond)

	_, _, err = tradeSvc.TradeComplete(ctx, swapComplete, nil)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
}
