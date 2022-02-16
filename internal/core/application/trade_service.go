package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	pkgswap "github.com/tdex-network/tdex-daemon/pkg/swap"
)

type TradeService interface {
	GetTradableMarkets(ctx context.Context) ([]MarketWithFee, error)
	GetMarketPrice(
		ctx context.Context,
		market Market, tradeType int, amount uint64, asset string,
	) (*PriceWithFee, error)
	TradePropose(
		ctx context.Context,
		market Market, tradeType int, swapRequest domain.SwapRequest,
	) (domain.SwapAccept, domain.SwapFail, uint64, error)
	TradeComplete(
		ctx context.Context,
		swapComplete domain.SwapComplete, swapFail domain.SwapFail,
	) (string, domain.SwapFail, error)
	GetMarketBalance(ctx context.Context, market Market) (*BalanceWithFee, error)
}

type tradeService struct {
	repoManager                ports.RepoManager
	wallet                     Wallet
	pubsubService              ports.SecurePubSub
	expiryDuration             time.Duration
	priceSlippage              decimal.Decimal
	feeAccountBalanceThreshold uint64
	milliSatsPerByte           int

	lock *sync.Mutex
}

func NewTradeService(
	repoManager ports.RepoManager,
	wallet Wallet,
	pubsubService ports.SecurePubSub,
	expiryDuration time.Duration,
	satsPerByte float64,
	priceSlippage decimal.Decimal,
	feeAccountBalanceThreshold uint64,
) TradeService {
	return newTradeService(
		repoManager, wallet, pubsubService, expiryDuration,
		satsPerByte, priceSlippage, feeAccountBalanceThreshold,
	)
}

func newTradeService(
	repoManager ports.RepoManager,
	wallet Wallet,
	pubsubService ports.SecurePubSub,
	expiryDuration time.Duration,
	satsPerByte float64,
	priceSlippage decimal.Decimal,
	feeAccountBalanceThreshold uint64,
) *tradeService {
	svc := &tradeService{
		repoManager:                repoManager,
		wallet:                     wallet,
		pubsubService:              pubsubService,
		expiryDuration:             expiryDuration,
		milliSatsPerByte:           int(satsPerByte * 1000),
		priceSlippage:              priceSlippage,
		feeAccountBalanceThreshold: feeAccountBalanceThreshold,
		lock:                       &sync.Mutex{},
	}
	svc.registerHandlerForTradeEvent()
	return svc
}

func (t *tradeService) GetTradableMarkets(
	ctx context.Context,
) ([]MarketWithFee, error) {
	tradableMarkets, err := t.repoManager.MarketRepository().GetTradableMarkets(ctx)
	if err != nil {
		log.Debugf("error while retrieving markets: %s", err)
		return nil, ErrServiceUnavailable
	}

	marketsWithFee := make([]MarketWithFee, 0, len(tradableMarkets))
	for _, mkt := range tradableMarkets {
		marketsWithFee = append(marketsWithFee, MarketWithFee{
			Market: Market{
				BaseAsset:  mkt.BaseAsset,
				QuoteAsset: mkt.QuoteAsset,
			},
			Fee: Fee{
				BasisPoint:    mkt.Fee,
				FixedBaseFee:  mkt.FixedFee.BaseFee,
				FixedQuoteFee: mkt.FixedFee.QuoteFee,
			},
		})
	}

	return marketsWithFee, nil
}

func (t *tradeService) GetMarketPrice(
	ctx context.Context,
	market Market, tradeType int, amount uint64, asset string,
) (*PriceWithFee, error) {
	if err := market.Validate(); err != nil {
		return nil, err
	}
	if err := validateAssetString(asset); err != nil {
		return nil, errors.New("invalid asset")
	}
	if asset != market.BaseAsset && asset != market.QuoteAsset {
		return nil, errors.New("asset must match one of those of the market")
	}

	mkt, mktAccountIndex, err := t.repoManager.MarketRepository().GetMarketByAssets(
		ctx, market.BaseAsset, market.QuoteAsset,
	)
	if err != nil {
		log.Debugf("error while retrieving market: %s", err)
		return nil, ErrServiceUnavailable
	}
	if mktAccountIndex < 0 {
		return nil, ErrMarketNotExist
	}

	if !mkt.IsTradable() {
		return nil, domain.ErrMarketIsClosed
	}

	mktBalance, err := t.wallet.BalanceForAccount(ctx, mkt.Name)
	if err != nil {
		log.Debugf("error while retrieving market balance: %s", err)
		return nil, ErrServiceUnavailable
	}
	var baseAssetBalance, quoteAssetBalance uint64
	if mktBalance != nil {
		baseAssetBalance = mktBalance[mkt.BaseAsset].Confirmed()
		quoteAssetBalance = mktBalance[mkt.QuoteAsset].Confirmed()
	}
	marketBalance := Balance{
		BaseAmount:  baseAssetBalance,
		QuoteAmount: quoteAssetBalance,
	}

	return previewForMarket(mkt, marketBalance, tradeType, amount, asset)
}

func (t *tradeService) GetMarketBalance(
	ctx context.Context, market Market,
) (*BalanceWithFee, error) {
	if err := market.Validate(); err != nil {
		return nil, err
	}

	m, accountIndex, err := t.repoManager.MarketRepository().GetMarketByAssets(
		ctx, market.BaseAsset, market.QuoteAsset,
	)
	if err != nil {
		log.WithError(err).Debug("error while retrieving market")
		return nil, ErrServiceUnavailable
	}
	if accountIndex < 0 {
		return nil, ErrMarketNotExist
	}

	balance, err := t.wallet.BalanceForAccount(ctx, m.Name)
	if err != nil {
		log.WithError(err).Debug("error while retrieving balance")
		return nil, ErrServiceUnavailable
	}

	var baseAssetBalance, quoteAssetBalance uint64
	if balance != nil {
		baseAssetBalance = balance[m.BaseAsset].Confirmed()
		quoteAssetBalance = balance[m.QuoteAsset].Confirmed()
	}

	return &BalanceWithFee{
		Balance: Balance{
			BaseAmount:  baseAssetBalance,
			QuoteAmount: quoteAssetBalance,
		},
		Fee: Fee{
			BasisPoint:    m.Fee,
			FixedBaseFee:  m.FixedFee.BaseFee,
			FixedQuoteFee: m.FixedFee.QuoteFee,
		},
	}, nil
}

func (t *tradeService) TradePropose(
	ctx context.Context, market Market, tradeType int, swapRequest domain.SwapRequest,
) (domain.SwapAccept, domain.SwapFail, uint64, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if err := market.Validate(); err != nil {
		return nil, nil, 0, err
	}

	mkt, _, err := t.repoManager.MarketRepository().GetMarketByAssets(
		ctx, market.BaseAsset, market.QuoteAsset,
	)
	if err != nil {
		log.Debugf("error while retrieving market: %s", err)
		return nil, nil, 0, ErrServiceUnavailable
	}
	if mkt == nil {
		return nil, nil, 0, ErrMarketNotExist
	}

	marketBalance, err := t.wallet.BalanceForAccount(ctx, mkt.Name)
	if err != nil {
		log.Debugf("error while retrieving market balance: %s", err)
		return nil, nil, 0, ErrServiceUnavailable
	}
	if marketBalance == nil {
		return nil, nil, 0, ErrMarketNotFunded
	}
	mktBalance := Balance{
		BaseAmount:  marketBalance[mkt.BaseAsset].Confirmed(),
		QuoteAmount: marketBalance[mkt.QuoteAsset].Confirmed(),
	}

	// parse swap proposal and possibly accept
	trade := domain.NewTrade()

	defer func() {
		if _, err := t.repoManager.TradeRepository().GetOrCreateTrade(
			ctx, &trade.ID,
		); err != nil {
			log.WithError(err).Warn("an error occured while adding new trade")
			return
		}
		if err := t.repoManager.TradeRepository().UpdateTrade(
			ctx, &trade.ID, func(_ *domain.Trade) (*domain.Trade, error) {
				return trade, nil
			},
		); err != nil {
			log.WithError(err).Warnf(
				"an error occured while updating trade with id %s", trade.ID,
			)
			return
		}
		log.Debugf("added new trade with id %s", trade.ID)
	}()

	if ok, _ := trade.Propose(
		swapRequest,
		market.BaseAsset, market.QuoteAsset,
		mkt.Fee, mkt.FixedFee.BaseFee, mkt.FixedFee.QuoteFee,
		nil,
	); !ok {
		return nil, trade.SwapFailMessage(), 0, nil
	}

	if !isValidTradePrice(swapRequest, tradeType, mkt, mktBalance, t.priceSlippage) {
		trade.Fail(
			swapRequest.GetId(),
			int(pkgswap.ErrCodeInvalidSwapRequest),
			"bad pricing",
		)
		return nil, trade.SwapFailMessage(), 0, nil
	}

	pset, selectedUtxos, inputBlindingKeys, outputBlindingKeys, err := t.wallet.
		FillSwapTransaction(ctx, mkt.Name, swapRequest)
	if err != nil {
		trade.Fail(
			swapRequest.GetId(),
			int(pkgswap.ErrCodeRejectedSwapRequest),
			"unable to fill swap transaction",
		)
		log.WithError(err).Infof("trade with id %s rejected", trade.ID)
		return nil, trade.SwapFailMessage(), 0, nil
	}

	if ok, _ := trade.Accept(
		pset, inputBlindingKeys, outputBlindingKeys,
		uint64(t.expiryDuration.Seconds()),
	); !ok {
		log.Infof("trade with id %s rejected", trade.ID)
		return nil, trade.SwapFailMessage(), 0, nil
	}

	log.Infof("trade with id %s accepted", trade.ID)

	// Register handler for either settle the trade or make it expiring
	go func() {
		t.wallet.RegisterHandlerForUtxoEvent(
			t.tradeSettleOrExpire(trade.ID, selectedUtxos),
		)
	}()

	return trade.SwapAcceptMessage(), nil, trade.ExpiryTime, nil
}

// TradeComplete is the domain controller for the TradeComplete RPC
func (t *tradeService) TradeComplete(
	ctx context.Context, swapComplete domain.SwapComplete, swapFail domain.SwapFail,
) (string, domain.SwapFail, error) {
	if swapFail != nil {
		swapFailMsg, err := t.tradeFail(ctx, swapFail)
		if err != nil {
			log.Debugf("error while aborting trade: %s", err)
			return "", nil, ErrServiceUnavailable
		}
		return "", swapFailMsg, nil
	}

	return t.tradeComplete(ctx, swapComplete)
}

func (t *tradeService) registerHandlerForTradeEvent() {
	if t.pubsubService == nil {
		return
	}

	wallet := t.wallet
	repoManager := t.repoManager
	pubsubService := t.pubsubService
	feeAccountBalanceThreshold := t.feeAccountBalanceThreshold

	repoManager.RegisterHandlerForTradeEvent(
		domain.TradeSettledEvent, func(event domain.TradeEvent) {
			trade := event.Trade
			ctx := context.Background()
			lbtc := wallet.NativeAsset()
			var feeAccountBalance uint64
			var market *domain.Market
			var marketBalance Balance

			market, _, _ = repoManager.MarketRepository().GetMarketByAssets(
				ctx, trade.MarketBaseAsset, trade.MarketQuoteAsset,
			)

			if market != nil {
				balance, _ := wallet.BalanceForAccount(ctx, market.Name)
				var baseAssetBalance, quoteAssetBalance uint64
				if balance != nil {
					if b, ok := balance[market.BaseAsset]; ok {
						baseAssetBalance = b.Total()
					}
					if b, ok := balance[market.QuoteAsset]; ok {
						quoteAssetBalance = b.Total()
					}
				}
				marketBalance = Balance{
					BaseAmount:  baseAssetBalance,
					QuoteAmount: quoteAssetBalance,
				}
				publishTradeSettledTopic(
					pubsubService, &trade, trade.MarketBaseAsset,
					baseAssetBalance, quoteAssetBalance,
				)
			}

			feeBalance, _ := wallet.BalanceForAccount(ctx, FeeAccount)
			if feeBalance != nil {
				if b, ok := feeBalance[lbtc]; ok {
					feeAccountBalance = b.Total()
				}
			}

			checkForFeeAndMarketLowBalances(
				pubsubService, feeAccountBalance, feeAccountBalanceThreshold,
				market, marketBalance,
			)
		},
	)
}

func (t *tradeService) tradeComplete(
	ctx context.Context, swapComplete domain.SwapComplete,
) (txID string, swapFail domain.SwapFail, err error) {
	swapID := swapComplete.GetAcceptId()
	trade, err := t.repoManager.TradeRepository().GetTradeBySwapAcceptID(ctx, swapID)
	if err != nil {
		return
	}
	if trade == nil {
		err = fmt.Errorf("trade with swap id %s not found", swapComplete.GetAcceptId())
		return
	}

	tx := swapComplete.GetTransaction()

	res, err := trade.Complete(tx)
	if err != nil {
		return
	}

	defer func() {
		if err := t.repoManager.TradeRepository().UpdateTrade(
			ctx, &trade.ID, func(_ *domain.Trade) (*domain.Trade, error) {
				return trade, nil
			},
		); err != nil {
			log.WithError(err).Warnf(
				"an error occured while storing updates for trade %s", trade.ID,
			)
		}
	}()

	if !res.OK {
		swapFail = trade.SwapFailMessage()
		return
	}
	log.Infof("trade with id %s completed", trade.ID)

	txid, err := t.wallet.TransactionManager().BroadcastTransaction(
		ctx, res.TxHex,
	)
	if err != nil {
		trade.Fail(
			swapID, int(pkgswap.ErrCodeFailedToComplete), fmt.Sprintf(
				"failed to broadcast tx: %s", err.Error(),
			))
		log.WithError(err).WithField("hex", res.TxHex).Warnf(
			"an error occured while broadcasting trade with id %s", trade.ID,
		)
		return
	}
	trade.TxID = txid
	trade.TxHex = res.TxHex

	log.Infof("trade with id %s broadcasted: %s", trade.ID, txid)
	return
}

func (t *tradeService) tradeFail(
	ctx context.Context, swapFail domain.SwapFail,
) (domain.SwapFail, error) {
	swapID := swapFail.GetMessageId()
	trade, err := t.repoManager.TradeRepository().GetTradeBySwapAcceptID(
		ctx, swapID,
	)
	if err != nil {
		return nil, err
	}

	tradeID := trade.ID
	if err := t.repoManager.TradeRepository().UpdateTrade(
		ctx, &tradeID, func(trade *domain.Trade) (*domain.Trade, error) {
			trade.Fail(
				swapID,
				int(pkgswap.ErrCodeFailedToComplete),
				"set failed by counter-party",
			)
			return trade, nil
		},
	); err != nil {
		return nil, err
	}

	return swapFail, nil
}

func (t *tradeService) tradeSettleOrExpire(
	tradeID uuid.UUID, selectedUtxos []ports.UtxoKey,
) UtxoNotificationHandler {
	return func(notification ports.UtxoNotification) bool {
		utxo := notification.Utxo()
		eventType := notification.EventType()
		txDetails, blockDetails, err := t.wallet.TransactionManager().GetTransaction(context.Background(), utxo.TxID())
		if err != nil {
			return false
		}

		isSelectedUtxo := false
		for _, u := range selectedUtxos {
			if utxo.TxID() == u.TxID() && utxo.Index() == u.Index() {
				isSelectedUtxo = true
				break
			}
		}
		if !isSelectedUtxo {
			return false
		}

		if eventType.IsUtxoSpent() {
			if err := t.repoManager.TradeRepository().UpdateTrade(
				context.Background(),
				&tradeID,
				func(trade *domain.Trade) (*domain.Trade, error) {
					if _, err := trade.Settle(uint64(blockDetails.Timestamp())); err != nil {
						return nil, err
					}

					if trade.TxHex == "" {
						trade.TxHex = txDetails.Hex()
					}
					if trade.TxID == "" {
						trade.TxID = txDetails.Hash()
					}
					return trade, nil
				},
			); err != nil {
				return false
			}
			log.Debugf("trade with id %s settled", tradeID)
			return true
		}

		if eventType.IsUtxoUnlocked() {
			if err := t.repoManager.TradeRepository().UpdateTrade(
				context.Background(),
				&tradeID,
				func(trade *domain.Trade) (*domain.Trade, error) {
					if _, err := trade.Expire(); err != nil {
						return nil, err
					}
					return trade, nil
				},
			); err != nil {
				return false
			}
			log.Debugf("trade with id %s expired", tradeID)
			return true
		}

		return false
	}
}

// previewForMarket returns the current price and balances of a market, along
// with a preview amount for a BUY or SELL trade based on the strategy type.
func previewForMarket(
	market *domain.Market, marketBalance Balance,
	tradeType int, amount uint64, asset string,
) (*PriceWithFee, error) {
	isBuy := tradeType == TradeBuy
	isBaseAsset := asset == market.BaseAsset

	preview, err := market.Preview(
		marketBalance.BaseAmount, marketBalance.QuoteAmount, amount,
		isBaseAsset, isBuy,
	)
	if err != nil {
		return nil, err
	}

	return &PriceWithFee{
		Price: Price(preview.Price),
		Fee: Fee{
			BasisPoint:    market.Fee,
			FixedBaseFee:  market.FixedFee.BaseFee,
			FixedQuoteFee: market.FixedFee.QuoteFee,
		},
		Amount:  preview.Amount,
		Asset:   preview.Asset,
		Balance: marketBalance,
	}, nil
}

// isValidPrice checks that the amounts of the trade are valid by
// making a preview of each counter amounts of the swap given the
// current price of the market.
// Since the price is variable in time, the predicted amounts are not compared
// against those of the swap, but rather they are used to create a range in
// which the swap amounts must be included to be considered valid.
func isValidTradePrice(
	swapRequest domain.SwapRequest, tradeType int,
	market *domain.Market, marketBalance Balance,
	slippage decimal.Decimal,
) bool {
	// TODO: parallelize the 2 ways of calculating and validating the preview
	// amount to speed up the process.
	amount := swapRequest.GetAmountR()
	if tradeType == TradeSell {
		amount = swapRequest.GetAmountP()
	}

	preview, _ := previewForMarket(
		market, marketBalance, tradeType, amount, market.BaseAsset,
	)

	if preview != nil {
		if isPriceInRange(swapRequest, tradeType, preview.Amount, true, slippage) {
			return true
		}
	}

	amount = swapRequest.GetAmountP()
	if tradeType == TradeSell {
		amount = swapRequest.GetAmountR()
	}

	preview, _ = previewForMarket(
		market, marketBalance, tradeType, amount, market.QuoteAsset,
	)

	if preview == nil {
		return false
	}

	return isPriceInRange(swapRequest, tradeType, preview.Amount, false, slippage)
}

func isPriceInRange(
	swapRequest domain.SwapRequest, tradeType int,
	previewAmount uint64, isPreviewForQuoteAsset bool,
	slippage decimal.Decimal,
) bool {
	amountToCheck := decimal.NewFromInt(int64(swapRequest.GetAmountP()))
	if tradeType == TradeSell {
		if isPreviewForQuoteAsset {
			amountToCheck = decimal.NewFromInt(int64(swapRequest.GetAmountR()))
		}
	} else {
		if !isPreviewForQuoteAsset {
			amountToCheck = decimal.NewFromInt(int64(swapRequest.GetAmountR()))
		}
	}

	expectedAmount := decimal.NewFromInt(int64(previewAmount))
	lowerBound := expectedAmount.Mul(decimal.NewFromInt(1).Sub(slippage))
	upperBound := expectedAmount.Mul(decimal.NewFromInt(1).Add(slippage))

	return amountToCheck.GreaterThanOrEqual(lowerBound) && amountToCheck.LessThanOrEqual(upperBound)
}
