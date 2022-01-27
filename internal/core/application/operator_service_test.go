package application_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tdex-network/tdex-daemon/internal/core/application"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
)

var (
	nativeAsset      = "5ac9f65c0efcc4775e0baec4ec03abdde22473cd3cf33c0419ca290e0751b225"
	marketBaseAsset  = nativeAsset
	marketFee        = int64(25)
	marketQuoteAsset = randomHex(32)
	market           = application.Market{
		BaseAsset:  marketBaseAsset,
		QuoteAsset: marketQuoteAsset,
	}
	addr                = "el1qq2t8ytye2p9pretecfncchwrwrzq2n87d5ct6eng9926gyxcvskq4vn36ntvm7nyc69zm9xrr2h077ycay9qg04z20w0p95h3"
	millisatPerByte     = uint64(100)
	feeBalanceThreshold = uint64(5000)
)

func TestAccountManagement(t *testing.T) {
	operatorSvc := newOperatorService()

	markets, err := operatorSvc.ListMarkets(ctx)
	require.NoError(t, err)
	require.Len(t, markets, 0)

	marketInfo, err := operatorSvc.GetMarketInfo(ctx, market)
	require.EqualError(t, err, application.ErrMarketNotExist.Error())
	require.Nil(t, marketInfo)

	err = operatorSvc.NewMarket(ctx, market)
	require.NoError(t, err)

	markets, err = operatorSvc.ListMarkets(ctx)
	require.NoError(t, err)
	require.Len(t, markets, 1)
	require.Equal(t, market.BaseAsset, markets[0].Market.BaseAsset)
	require.Equal(t, market.QuoteAsset, markets[0].Market.QuoteAsset)
	require.False(t, markets[0].Tradable)
	require.Nil(t, markets[0].Balance)

	marketInfo, err = operatorSvc.GetMarketInfo(ctx, market)
	require.NoError(t, err)
	require.NotNil(t, marketInfo)
	require.Equal(t, market.BaseAsset, marketInfo.Market.BaseAsset)
	require.Equal(t, market.QuoteAsset, marketInfo.Market.QuoteAsset)
	require.False(t, marketInfo.Tradable)
	require.Nil(t, marketInfo.Balance)

	// Attempt opening the market while Fee and market account have 0 balance
	feeBalance, _, err := operatorSvc.GetFeeBalance(ctx)
	require.NoError(t, err)
	require.Zero(t, feeBalance)

	err = operatorSvc.OpenMarket(ctx, market)
	require.EqualError(t, err, application.ErrFeeAccountNotFunded.Error())
	// Now pretend the Fee account is funded but the market account isn't
	err = operatorSvc.OpenMarket(ctx, market)
	require.EqualError(t, err, application.ErrMarketNotFunded.Error())
	// Now pretend both Fee and market account are funded.
	// At this point, it should be possible to open the market
	err = operatorSvc.OpenMarket(ctx, market)
	require.NoError(t, err)

	markets, err = operatorSvc.ListMarkets(ctx)
	require.NoError(t, err)
	require.Len(t, markets, 1)
	require.Equal(t, market.BaseAsset, markets[0].Market.BaseAsset)
	require.Equal(t, market.QuoteAsset, markets[0].Market.QuoteAsset)
	require.True(t, markets[0].Tradable)
	require.NotNil(t, markets[0].Balance)

	marketInfo, err = operatorSvc.GetMarketInfo(ctx, market)
	require.NoError(t, err)
	require.NotNil(t, marketInfo)
	require.Equal(t, market.BaseAsset, marketInfo.Market.BaseAsset)
	require.Equal(t, market.QuoteAsset, marketInfo.Market.QuoteAsset)
	require.True(t, marketInfo.Tradable)
	require.NotNil(t, marketInfo.Balance)

	// Attempt dropping the market with no 0 balance
	err = operatorSvc.DropMarket(ctx, market)
	require.EqualError(t, err, application.ErrMarketIsOpen.Error())

	err = operatorSvc.CloseMarket(ctx, market)
	require.NoError(t, err)

	err = operatorSvc.DropMarket(ctx, market)
	require.EqualError(t, err, application.ErrMarketNonZeroBalance.Error())

	// Withdraw all the funds from the market
	baseAssetBalance := marketInfo.Balance[market.BaseAsset].Total()
	quoteAssetBalance := marketInfo.Balance[market.QuoteAsset].Total()
	outputs := []application.Output{
		application.NewOutput(addr, market.BaseAsset, baseAssetBalance),
		application.NewOutput(addr, market.QuoteAsset, quoteAssetBalance),
	}

	// Market is already closed, withdrawals should be allowed
	txHex, txid, err := operatorSvc.WithdrawMarketFunds(ctx, market, outputs, millisatPerByte)
	require.NoError(t, err)
	require.NotNil(t, txHex)
	require.NotNil(t, txid)

	marketInfo, err = operatorSvc.GetMarketInfo(ctx, market)
	require.NoError(t, err)
	require.NotNil(t, marketInfo)
	require.Equal(t, market.BaseAsset, marketInfo.Market.BaseAsset)
	require.Equal(t, market.QuoteAsset, marketInfo.Market.QuoteAsset)
	require.False(t, marketInfo.Tradable)
	require.Nil(t, marketInfo.Balance)

	// Now dropping the market should be possible
	err = operatorSvc.DropMarket(ctx, market)
	require.NoError(t, err)

	markets, err = operatorSvc.ListMarkets(ctx)
	require.NoError(t, err)
	require.Len(t, markets, 0)

	marketInfo, err = operatorSvc.GetMarketInfo(ctx, market)
	require.EqualError(t, err, application.ErrMarketNotExist.Error())
	require.Nil(t, marketInfo)

	marketBalance, err := operatorSvc.GetMarketBalance(ctx, market)
	require.EqualError(t, err, application.ErrMarketNotExist.Error())
	require.Nil(t, marketBalance)
}

func newOperatorService() application.OperatorService {
	repoManager, wallet := newServices()
	accountManager := wallet.(*mockedWallet).accountManager
	accountManager.On("CreateAccount", mock.Anything, mock.Anything).Return(uint64(randomIntInRange(0, 15)), randomBase64(), nil)
	accountManager.On("DeleteAccount", mock.Anything, mock.Anything).Return(nil)

	wallet.(*mockedWallet).On("NativeAsset").Return(nativeAsset)
	wallet.(*mockedWallet).On("BalanceForAccount", mock.Anything, market.Name()).Return(nil, nil).Times(3)
	wallet.(*mockedWallet).On("BalanceForAccount", mock.Anything, application.FeeAccount).Return(nil, nil).Twice()
	feeBalance := map[string]ports.Balance{
		nativeAsset: accountBalance{1000000, 1000000, 0},
	}
	wallet.(*mockedWallet).On("BalanceForAccount", mock.Anything, application.FeeAccount).Return(feeBalance, nil)
	marketBalance := map[string]ports.Balance{
		marketBaseAsset:  accountBalance{100000000, 100000000, 0},
		marketQuoteAsset: accountBalance{3000000000000, 3000000000000, 0},
	}
	wallet.(*mockedWallet).On("BalanceForAccount", mock.Anything, market.Name()).Return(marketBalance, nil).Times(4)
	wallet.(*mockedWallet).On("BalanceForAccount", mock.Anything, market.Name()).Return(nil, nil).Twice()
	wallet.(*mockedWallet).On("SendToManyWithFeeTopup", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(randomHex(100), randomHex(32), nil)
	return application.NewOperatorService(repoManager, wallet, nil, marketBaseAsset, "", marketFee, feeBalanceThreshold)
}
