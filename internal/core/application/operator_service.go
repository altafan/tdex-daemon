package application

import (
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	"github.com/tdex-network/tdex-daemon/pkg/mathutil"
)

var (
	DefaultFeeFragmenterFragments = uint32(50)
	DefaultFeeFragmenterAmount    = uint64(5000)
	FragmentationMap              = map[int]int{
		1: 30,
		2: 15,
		3: 10,
		5: 2,
	}
	PollInterval              = 1 * time.Second
	MinFeeFragmenterAmount    = 5000
	MinMarketFragmenterAmount = 100000
)

// OperatorService defines the methods of the application layer for the operator service.
type OperatorService interface {
	GetInfo(ctx context.Context) (ports.WalletInfo, error)
	// Fee account
	GetFeeAddress(
		ctx context.Context, numOfAddresses int,
	) ([]AddressAndBlindingKey, error)
	ListFeeExternalAddresses(
		ctx context.Context,
	) ([]AddressAndBlindingKey, error)
	GetFeeBalance(ctx context.Context) (int64, int64, error)
	WithdrawFeeFunds(
		ctx context.Context, outputs Outputs, millisatsPerByte uint64,
	) ([]byte, []byte, error)
	// Market account
	NewMarket(ctx context.Context, market Market) error
	GetMarketInfo(ctx context.Context, market Market) (*MarketInfo, error)
	GetMarketAddress(
		ctx context.Context, market Market, numOfAddresses int,
	) ([]AddressAndBlindingKey, error)
	ListMarketExternalAddresses(
		ctx context.Context, req Market,
	) ([]AddressAndBlindingKey, error)
	GetMarketBalance(ctx context.Context, market Market) (map[string]ports.Balance, error)
	OpenMarket(ctx context.Context, market Market) error
	CloseMarket(ctx context.Context, market Market) error
	DropMarket(ctx context.Context, market Market) error
	GetMarketCollectedFee(
		ctx context.Context, market Market, page *Page,
	) (*ReportMarketFee, error)
	WithdrawMarketFunds(
		ctx context.Context,
		market Market, outputs Outputs, millisatPerByte uint64,
	) ([]byte, []byte, error)
	UpdateMarketPercentageFee(
		ctx context.Context, req MarketWithFee,
	) (*MarketWithFee, error)
	UpdateMarketFixedFee(
		ctx context.Context, req MarketWithFee,
	) (*MarketWithFee, error)
	UpdateMarketPrice(ctx context.Context, req MarketWithPrice) error
	UpdateMarketStrategy(ctx context.Context, req MarketStrategy) error
	// Fee Fragmenter account
	GetFeeFragmenterAddress(
		ctx context.Context, numOfAddresses int,
	) ([]AddressAndBlindingKey, error)
	ListFeeFragmenterExternalAddresses(
		ctx context.Context,
	) ([]AddressAndBlindingKey, error)
	GetFeeFragmenterBalance(ctx context.Context) (map[string]ports.Balance, error)
	FeeFragmenterSplitFunds(
		ctx context.Context, maxFragments uint32, millisatsPerByte uint64,
		chRes chan FragmenterSplitFundsReply,
	)
	WithdrawFeeFragmenterFunds(
		ctx context.Context, addr string, millisatsPerByte uint64,
	) (string, error)
	// Market fragmenter account
	GetMarketFragmenterAddress(
		ctx context.Context, numOfAddresses int,
	) ([]AddressAndBlindingKey, error)
	ListMarketFragmenterExternalAddresses(
		ctx context.Context,
	) ([]AddressAndBlindingKey, error)
	GetMarketFragmenterBalance(
		ctx context.Context,
	) (map[string]ports.Balance, error)
	MarketFragmenterSplitFunds(
		ctx context.Context, market Market, millisatsPerByte uint64,
		chRes chan FragmenterSplitFundsReply,
	)
	WithdrawMarketFragmenterFunds(
		ctx context.Context, addr string, millisatsPerByte uint64,
	) (string, error)
	// List methods
	ListMarkets(ctx context.Context) ([]MarketInfo, error)
	ListTrades(ctx context.Context, page *Page) ([]TradeInfo, error)
	ListTradesForMarket(
		ctx context.Context, market Market, page *Page,
	) ([]TradeInfo, error)
	ListUtxos(
		ctx context.Context, account string, page *Page,
	) (spendableUnspents []ports.Utxo, lockedUnspents []ports.Utxo, err error)
	ListDeposits(
		ctx context.Context, accountName string, page *Page,
	) (Deposits, error)
	ListWithdrawals(
		ctx context.Context, accountName string, page *Page,
	) (Withdrawals, error)
	// Webhook
	AddWebhook(ctx context.Context, hook Webhook) (string, error)
	RemoveWebhook(ctx context.Context, id string) error
	ListWebhooks(ctx context.Context, actionType int) ([]WebhookInfo, error)
}

type operatorService struct {
	repoManager                ports.RepoManager
	wallet                     Wallet
	pubsubService              ports.SecurePubSub
	marketBaseAsset            string
	marketQuoteAsset           string
	marketFee                  int64
	feeAccountBalanceThreshold uint64

	fragmenterLock *sync.RWMutex
}

// NewOperatorService is a constructor function for OperatorService.
func NewOperatorService(
	repoManager ports.RepoManager,
	wallet Wallet,
	pubsubService ports.SecurePubSub,
	marketBaseAsset, marketQuoteAsset string,
	marketFee int64,
	feeAccountBalanceThreshold uint64,
) OperatorService {
	return newOperatorService(
		repoManager, wallet, pubsubService,
		marketBaseAsset, marketQuoteAsset, marketFee, feeAccountBalanceThreshold,
	)
}

func newOperatorService(
	repoManager ports.RepoManager,
	wallet Wallet,
	pubsubService ports.SecurePubSub,
	marketBaseAsset, marketQuoteAsset string,
	marketFee int64,
	feeAccountBalanceThreshold uint64,
) *operatorService {
	svc := &operatorService{
		repoManager:                repoManager,
		wallet:                     wallet,
		pubsubService:              pubsubService,
		marketBaseAsset:            marketBaseAsset,
		marketQuoteAsset:           marketQuoteAsset,
		marketFee:                  marketFee,
		feeAccountBalanceThreshold: feeAccountBalanceThreshold,
		fragmenterLock:             &sync.RWMutex{},
	}
	svc.registerHandlerForWithdrawalEvent()
	return svc
}

func (o *operatorService) GetInfo(ctx context.Context) (ports.WalletInfo, error) {
	return o.wallet.WalletManager().GetInfo(ctx)
}

func (o *operatorService) GetFeeAddress(
	ctx context.Context, numOfAddresses int,
) ([]AddressAndBlindingKey, error) {
	if numOfAddresses <= 0 {
		numOfAddresses = 1
	}
	return o.wallet.DeriveAddressForAccount(ctx, FeeAccount, uint64(numOfAddresses))
}

func (o *operatorService) ListFeeExternalAddresses(
	ctx context.Context,
) ([]AddressAndBlindingKey, error) {
	return o.wallet.ListAddressesForAccount(ctx, FeeAccount)
}

func (o *operatorService) GetFeeBalance(ctx context.Context) (int64, int64, error) {
	balancePerAsset, err := o.wallet.BalanceForAccount(ctx, FeeAccount)
	if err != nil {
		return -1, -1, err
	}
	if balancePerAsset == nil {
		return 0, 0, nil
	}
	if _, ok := balancePerAsset[o.wallet.NativeAsset()]; !ok {
		return 0, 0, nil
	}
	return int64(balancePerAsset[o.wallet.NativeAsset()].Confirmed()),
		int64(balancePerAsset[o.wallet.NativeAsset()].Total()), nil
}

func (o *operatorService) WithdrawFeeFunds(
	ctx context.Context, outputs Outputs, millisatPerByte uint64,
) ([]byte, []byte, error) {
	txHex, err := o.wallet.TransactionManager().TransferFromAccount(
		ctx, FeeAccount, outputs.toPortableList(), millisatPerByte,
	)
	if err != nil {
		return nil, nil, err
	}

	txid, err := o.wallet.TransactionManager().BroadcastTransaction(ctx, txHex)
	if err != nil {
		return nil, nil, err
	}

	go func() {
		if _, err := o.repoManager.WithdrawalRepository().AddWithdrawals(
			ctx,
			[]domain.Withdrawal{
				{
					TxID:              txid,
					AccountName:       FeeAccount,
					Outputs:           outputs.toDomainList(),
					MillisatPerByte:   millisatPerByte,
					TotAmountPerAsset: outputs.totAmountPerAsset(),
					Timestamp:         uint64(time.Now().Unix()),
				},
			},
		); err != nil {
			log.WithError(err).Warn("an error occured while storing withdrawal info")
			return
		}
		log.Debug("added 1 withdrawal")
	}()

	rawTx, _ := hex.DecodeString(txHex)
	rawTxid, _ := hex.DecodeString(txid)
	return rawTx, rawTxid, nil
}

func (o *operatorService) NewMarket(ctx context.Context, mkt Market) error {
	if err := mkt.Validate(); err != nil {
		return err
	}
	if len(o.marketBaseAsset) > 0 && mkt.BaseAsset != o.marketBaseAsset {
		return ErrMarketInvalidBaseAsset
	}
	if len(o.marketQuoteAsset) > 0 && mkt.QuoteAsset != o.marketQuoteAsset {
		return ErrMarketInvalidQuoteAsset
	}

	_, existingAccountIndex, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, mkt.BaseAsset, mkt.QuoteAsset,
	)
	if err != nil {
		return err
	}
	if existingAccountIndex >= 0 {
		return ErrMarketAlreadyExist
	}

	accountName := mkt.Name()
	accountIndex, _, err := o.wallet.AccountManager().CreateAccount(ctx, accountName)
	if err != nil {
		return err
	}
	newMarket, err := domain.NewMarket(
		accountIndex, mkt.BaseAsset, mkt.QuoteAsset, o.marketFee,
	)
	if err != nil {
		return err
	}

	_, err = o.repoManager.MarketRepository().GetOrCreateMarket(ctx, newMarket)
	return err
}

func (o *operatorService) GetMarketInfo(ctx context.Context, mkt Market) (*MarketInfo, error) {
	market, _, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, mkt.BaseAsset, mkt.QuoteAsset,
	)
	if err != nil {
		return nil, err
	}
	if market == nil {
		return nil, ErrMarketNotExist
	}

	balance, err := o.wallet.BalanceForAccount(ctx, market.Name)
	if err != nil {
		return nil, err
	}

	return &MarketInfo{
		AccountIndex: uint64(market.AccountIndex),
		AccountName:  market.Name,
		Market: Market{
			BaseAsset:  market.BaseAsset,
			QuoteAsset: market.QuoteAsset,
		},
		Tradable:     market.Tradable,
		StrategyType: market.Strategy.Type,
		Price:        market.Price,
		Fee: Fee{
			BasisPoint:    market.Fee,
			FixedBaseFee:  market.FixedFee.BaseFee,
			FixedQuoteFee: market.FixedFee.QuoteFee,
		},
		Balance: balance,
	}, nil
}

func (o *operatorService) GetMarketAddress(
	ctx context.Context, mkt Market, numOfAddresses int,
) ([]AddressAndBlindingKey, error) {
	if err := mkt.Validate(); err != nil {
		return nil, err
	}

	market, _, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, mkt.BaseAsset, mkt.QuoteAsset,
	)
	if err != nil {
		return nil, err
	}
	if market == nil {
		return nil, ErrMarketNotExist
	}

	if numOfAddresses <= 0 {
		numOfAddresses = 1
	}
	return o.wallet.DeriveAddressForAccount(
		ctx, market.Name, uint64(numOfAddresses),
	)
}

func (o *operatorService) ListMarketExternalAddresses(
	ctx context.Context, mkt Market,
) ([]AddressAndBlindingKey, error) {
	if err := mkt.Validate(); err != nil {
		return nil, err
	}

	market, _, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, mkt.BaseAsset, mkt.QuoteAsset,
	)
	if err != nil {
		return nil, err
	}
	if market == nil {
		return nil, ErrMarketNotExist
	}

	return o.wallet.ListAddressesForAccount(ctx, market.Name)
}

func (o *operatorService) GetMarketBalance(
	ctx context.Context, mkt Market,
) (map[string]ports.Balance, error) {
	if err := mkt.Validate(); err != nil {
		return nil, err
	}

	market, _, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, mkt.BaseAsset, mkt.QuoteAsset,
	)
	if err != nil {
		return nil, err
	}
	if market == nil {
		return nil, ErrMarketNotExist
	}

	return o.wallet.BalanceForAccount(ctx, market.Name)
}

func (o *operatorService) OpenMarket(ctx context.Context, mkt Market) error {
	if err := mkt.Validate(); err != nil {
		return err
	}

	feeBalance, err := o.wallet.BalanceForAccount(ctx, FeeAccount)
	if err != nil {
		return err
	}
	if feeBalance == nil ||
		feeBalance[o.wallet.NativeAsset()].Total() <= o.feeAccountBalanceThreshold {
		return ErrFeeAccountNotFunded
	}

	// Check if market exists
	market, _, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, mkt.BaseAsset, mkt.QuoteAsset,
	)
	if err != nil {
		return err
	}
	if market == nil {
		return ErrMarketNotExist
	}

	marketBalance, err := o.wallet.BalanceForAccount(ctx, market.Name)
	if err != nil {
		return err
	}
	if marketBalance == nil {
		return ErrMarketNotFunded
	}
	baseBalance := marketBalance[market.BaseAsset].Confirmed()
	quoteBalance := marketBalance[market.QuoteAsset].Confirmed()

	isZeroBalance := int64(baseBalance) <= market.FixedFee.BaseFee &&
		int64(quoteBalance) <= market.FixedFee.QuoteFee
	if market.IsStrategyBalanced() {
		isZeroBalance = int64(baseBalance) <= market.FixedFee.BaseFee ||
			int64(quoteBalance) <= market.FixedFee.QuoteFee
	}
	if isZeroBalance {
		return ErrMarketNotFunded
	}

	// Open the market
	return o.repoManager.MarketRepository().UpdateMarket(
		ctx, market.AccountIndex, func(m *domain.Market) (*domain.Market, error) {
			if err := m.MakeTradable(); err != nil {
				return nil, err
			}
			return m, nil
		},
	)
}

func (o *operatorService) CloseMarket(ctx context.Context, mkt Market) error {
	if err := mkt.Validate(); err != nil {
		return err
	}

	_, accountIndex, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, mkt.BaseAsset, mkt.QuoteAsset,
	)
	if err != nil {
		return err
	}
	if accountIndex < 0 {
		return ErrMarketNotExist
	}

	return o.repoManager.MarketRepository().UpdateMarket(
		ctx, uint64(accountIndex), func(m *domain.Market) (*domain.Market, error) {
			if err := m.MakeNotTradable(); err != nil {
				return nil, err
			}
			return m, nil
		})
}

func (o *operatorService) DropMarket(ctx context.Context, market Market) error {
	if err := market.Validate(); err != nil {
		return err
	}

	mkt, _, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, market.BaseAsset, market.QuoteAsset,
	)
	if err != nil {
		return err
	}
	if mkt == nil {
		return ErrMarketNotExist
	}
	if mkt.Tradable {
		return ErrMarketIsOpen
	}

	balance, err := o.wallet.BalanceForAccount(ctx, mkt.Name)
	if err != nil {
		return err
	}

	if balance != nil &&
		(balance[mkt.BaseAsset].Total() > 0 || balance[mkt.QuoteAsset].Total() > 0) {
		return ErrMarketNonZeroBalance
	}

	if err := o.wallet.AccountManager().DeleteAccount(ctx, mkt.Name); err != nil {
		return err
	}

	if err := o.repoManager.MarketRepository().DeleteMarket(
		ctx, mkt.AccountIndex,
	); err != nil {
		return err
	}

	return err
}

func (o *operatorService) GetMarketCollectedFee(
	ctx context.Context, mkt Market, page *Page,
) (*ReportMarketFee, error) {
	m, _, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, mkt.BaseAsset, mkt.QuoteAsset,
	)
	if err != nil {
		return nil, err
	}

	if m == nil {
		return nil, ErrMarketNotExist
	}

	var trades []*domain.Trade
	if page == nil {
		trades, err = o.repoManager.TradeRepository().GetCompletedTradesByMarket(
			ctx, mkt.QuoteAsset,
		)
	} else {
		pg := page.ToDomain()
		trades, err = o.repoManager.TradeRepository().GetCompletedTradesByMarketAndPage(
			ctx, mkt.QuoteAsset, pg,
		)
	}
	if err != nil {
		return nil, err
	}

	// sort trades by timestamp like done in ListTrades
	sort.SliceStable(trades, func(i, j int) bool {
		return trades[i].SwapRequest.Timestamp < trades[j].SwapRequest.Timestamp
	})

	fees := make([]FeeInfo, 0, len(trades))
	total := make(map[string]int64)
	for _, trade := range trades {
		feeBasisPoint := trade.MarketFee
		swapRequest := trade.SwapRequestMessage()
		feeAsset := swapRequest.GetAssetP()
		amountP := swapRequest.GetAmountP()
		_, percentageFeeAmount := mathutil.LessFee(amountP, uint64(feeBasisPoint))

		marketPrice := trade.MarketPrice.BasePrice
		fixedFeeAmount := uint64(trade.MarketFixedQuoteFee)
		if feeAsset == m.BaseAsset {
			marketPrice = trade.MarketPrice.QuotePrice
			fixedFeeAmount = uint64(trade.MarketFixedBaseFee)
		}

		fees = append(fees, FeeInfo{
			TradeID:             trade.ID.String(),
			BasisPoint:          feeBasisPoint,
			Asset:               feeAsset,
			PercentageFeeAmount: percentageFeeAmount,
			FixedFeeAmount:      fixedFeeAmount,
			MarketPrice:         marketPrice,
		})

		total[feeAsset] += int64(percentageFeeAmount) + int64(fixedFeeAmount)
	}

	return &ReportMarketFee{
		CollectedFees:              fees,
		TotalCollectedFeesPerAsset: total,
	}, nil
}

func (o *operatorService) WithdrawMarketFunds(
	ctx context.Context, market Market, outputs Outputs, millisatPerByte uint64,
) ([]byte, []byte, error) {
	mkt, _, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, market.BaseAsset, market.QuoteAsset,
	)
	if err != nil {
		return nil, nil, err
	}
	if mkt == nil {
		return nil, nil, ErrMarketNotExist
	}
	if mkt.Tradable {
		return nil, nil, ErrMarketIsOpen
	}

	txHex, txid, err := o.wallet.SendToManyWithFeeTopup(
		ctx, mkt.Name, outputs, millisatPerByte,
	)
	if err != nil {
		return nil, nil, err
	}

	go func() {
		if _, err := o.repoManager.WithdrawalRepository().AddWithdrawals(
			ctx,
			[]domain.Withdrawal{
				{
					TxID:              txid,
					AccountName:       mkt.Name,
					Outputs:           outputs.toDomainList(),
					MillisatPerByte:   millisatPerByte,
					TotAmountPerAsset: outputs.totAmountPerAsset(),
					Timestamp:         uint64(time.Now().Unix()),
				},
			},
		); err != nil {
			log.WithError(err).Warn("an error occured while storing withdrawal info")
			return
		}
		log.Debug("added 1 withdrawal")
	}()

	rawTx, _ := hex.DecodeString(txHex)
	rawTxid, _ := hex.DecodeString(txid)
	return rawTx, rawTxid, nil
}

// UpdateMarketPercentageFee changes the Liquidity Provider fee for the given market.
// MUST be expressed as basis point.
// Eg. To change the fee on each swap from 0.25% to 1% you need to pass down 100
// The Market MUST be closed before doing this change.
func (o *operatorService) UpdateMarketPercentageFee(
	ctx context.Context, req MarketWithFee,
) (*MarketWithFee, error) {
	if err := req.Market.Validate(); err != nil {
		return nil, err
	}

	mkt, accountIndex, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, req.BaseAsset, req.QuoteAsset,
	)
	if err != nil {
		return nil, err
	}
	if accountIndex < 0 {
		return nil, ErrMarketNotExist
	}

	if err := mkt.ChangeFeeBasisPoint(req.BasisPoint); err != nil {
		return nil, err
	}

	if err := o.repoManager.MarketRepository().UpdateMarket(
		ctx, uint64(accountIndex), func(_ *domain.Market) (*domain.Market, error) {
			return mkt, nil
		},
	); err != nil {
		return nil, err
	}

	return &MarketWithFee{
		Market: Market{
			BaseAsset:  mkt.BaseAsset,
			QuoteAsset: mkt.QuoteAsset,
		},
		Fee: Fee{
			BasisPoint:    mkt.Fee,
			FixedBaseFee:  mkt.FixedFee.BaseFee,
			FixedQuoteFee: mkt.FixedFee.QuoteFee,
		},
	}, nil
}

// UpdateMarketFixedFee changes the Liquidity Provider fee for the given market.
// Values for both assets MUST be expressed as satoshis.
func (o *operatorService) UpdateMarketFixedFee(
	ctx context.Context, req MarketWithFee,
) (*MarketWithFee, error) {
	if err := req.Market.Validate(); err != nil {
		return nil, err
	}

	mkt, accountIndex, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, req.BaseAsset, req.QuoteAsset,
	)
	if err != nil {
		return nil, err
	}
	if accountIndex < 0 {
		return nil, ErrMarketNotExist
	}

	if err := mkt.ChangeFixedFee(req.FixedBaseFee, req.FixedQuoteFee); err != nil {
		return nil, err
	}

	if err := o.repoManager.MarketRepository().UpdateMarket(
		ctx, uint64(accountIndex), func(_ *domain.Market) (*domain.Market, error) {
			return mkt, nil
		},
	); err != nil {
		return nil, err
	}

	return &MarketWithFee{
		Market: Market{
			BaseAsset:  mkt.BaseAsset,
			QuoteAsset: mkt.QuoteAsset,
		},
		Fee: Fee{
			BasisPoint:    mkt.Fee,
			FixedBaseFee:  mkt.FixedFee.BaseFee,
			FixedQuoteFee: mkt.FixedFee.QuoteFee,
		},
	}, nil
}

// UpdateMarketPrice rpc updates the price for the given market
func (o *operatorService) UpdateMarketPrice(
	ctx context.Context, req MarketWithPrice,
) error {
	if err := req.Market.Validate(); err != nil {
		return err
	}
	if err := req.Price.Validate(); err != nil {
		return err
	}

	_, accountIndex, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, req.BaseAsset, req.QuoteAsset,
	)
	if err != nil {
		return err
	}
	if accountIndex < 0 {
		return ErrMarketNotExist
	}

	// Updates the base price and the quote price
	return o.repoManager.MarketRepository().UpdatePrices(
		ctx, uint64(accountIndex), domain.Prices{
			BasePrice:  req.Price.BasePrice,
			QuotePrice: req.Price.QuotePrice,
		},
	)
}

// UpdateMarketStrategy changes the current market making strategy,
// either using an automated market making formula or a pluggable price feed
func (o *operatorService) UpdateMarketStrategy(
	ctx context.Context, req MarketStrategy,
) error {
	if err := req.Market.Validate(); err != nil {
		return err
	}

	_, accountIndex, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, req.BaseAsset, req.QuoteAsset,
	)
	if err != nil {
		return err
	}

	if accountIndex < 0 {
		return ErrMarketNotExist
	}

	requestStrategy := req.Strategy

	return o.repoManager.MarketRepository().UpdateMarket(
		ctx, uint64(accountIndex), func(m *domain.Market) (*domain.Market, error) {
			switch requestStrategy {
			case domain.StrategyTypePluggable:
				if err := m.MakeStrategyPluggable(); err != nil {
					return nil, err
				}

			case domain.StrategyTypeBalanced:
				if err := m.MakeStrategyBalanced(); err != nil {
					return nil, err
				}

			default:
				return nil, ErrUnknownStrategy
			}

			return m, nil
		},
	)
}

func (o *operatorService) GetFeeFragmenterAddress(
	ctx context.Context, numOfAddresses int,
) ([]AddressAndBlindingKey, error) {
	if numOfAddresses <= 0 {
		numOfAddresses = 1
	}
	return o.wallet.DeriveAddressForAccount(
		ctx, FeeFragmenterAccount, uint64(numOfAddresses),
	)
}

func (o *operatorService) ListFeeFragmenterExternalAddresses(
	ctx context.Context,
) ([]AddressAndBlindingKey, error) {
	return o.wallet.ListAddressesForAccount(ctx, FeeFragmenterAccount)
}

func (o *operatorService) GetFeeFragmenterBalance(
	ctx context.Context,
) (map[string]ports.Balance, error) {
	return o.wallet.BalanceForAccount(ctx, FeeFragmenterAccount)
}

func (o *operatorService) FeeFragmenterSplitFunds(
	ctx context.Context, maxFragments uint32, millisatsPerByte uint64,
	chRes chan FragmenterSplitFundsReply,
) {
	defer close(chRes)

	chRes <- FragmenterSplitFundsReply{
		Msg: fmt.Sprintf("fetching %s funds", FeeFragmenterAccount),
	}

	balance, err := o.wallet.BalanceForAccount(ctx, FeeFragmenterAccount)
	if err != nil {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"error while fetching %s funds: %s", FeeFragmenterAccount, err,
			),
		}
		return
	}
	if len(balance) <= 0 {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf("no funds detected for %s", FeeFragmenterAccount),
		}
		return
	}

	if len(balance) > 1 {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"detected funds with asset different from LBTC. The fragmentation " +
					"can't proceed any longer until those funds are withdrawn",
			),
		}
		return
	}

	totalAmount := balance[o.wallet.NativeAsset()].Total()
	if totalAmount == 0 {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf("no LBTC funds detected for %s", FeeFragmenterAccount),
		}
		return
	}

	chRes <- FragmenterSplitFundsReply{
		Msg: fmt.Sprintf("detected LBTC funds of total amount %d", totalAmount),
	}

	if totalAmount < uint64(MinFeeFragmenterAmount) {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"total amount %d to fragment is too small, should be at least %d sats",
				totalAmount, MinFeeFragmenterAmount,
			),
		}
		return
	}

	fragmentedAmounts := feeFragmentAmount(
		totalAmount, DefaultFeeFragmenterAmount, maxFragments,
	)
	numOfFragments := len(fragmentedAmounts)

	chRes <- FragmenterSplitFundsReply{
		Msg: fmt.Sprintf(
			"splitting total amount %d into %d fragments",
			totalAmount, numOfFragments,
		),
	}

	chRes <- FragmenterSplitFundsReply{
		Msg: fmt.Sprintf("creating outputs for fragments"),
	}

	outputs := make(Outputs, 0, numOfFragments)
	addresses, err := o.wallet.AccountManager().DeriveAddressesForAccount(
		ctx, FeeAccount, uint64(numOfFragments),
	)
	if err != nil {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf("error while creating outputs for fragments: %s", err),
		}
		return
	}
	i := 0
	for _, amount := range fragmentedAmounts {
		outputs = append(outputs, Output{
			o.wallet.NativeAsset(), amount, addresses[i],
		})
		i++
	}

	chRes <- FragmenterSplitFundsReply{
		Msg: fmt.Sprintf("creating transaction to deposit fragments to %s", FeeAccount),
	}

	txHex, err := o.wallet.TransactionManager().TransferFromAccount(
		ctx, FeeFragmenterAccount, outputs.toPortableList(), millisatsPerByte,
	)
	if err != nil {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"error while creating transaction to deposit fragments to %s: %s",
				FeeAccount, err,
			),
		}
		return
	}

	chRes <- FragmenterSplitFundsReply{
		Msg: fmt.Sprintf("broadcasting %s funding transaction", FeeAccount),
	}

	txid, err := o.wallet.TransactionManager().BroadcastTransaction(ctx, txHex)
	if err != nil {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"error while broadcasting %s funding transaction: %s", FeeAccount, err,
			),
		}
	}

	chRes <- FragmenterSplitFundsReply{
		Msg: fmt.Sprintf("%s funding transaction: %s", FeeAccount, txid),
	}

	go func() {
		deposits := make([]domain.Deposit, 0, len(outputs))
		now := uint64(time.Now().Unix())
		for i, o := range outputs {
			deposits = append(deposits, domain.Deposit{
				AccountName: FeeAccount,
				TxID:        txid,
				VOut:        i,
				Asset:       o.asset,
				Value:       o.value,
				Timestamp:   now,
			})
		}

		count, err := o.repoManager.DepositRepository().AddDeposits(ctx, deposits)
		if err != nil {
			log.WithError(err).Warn("an error occured while adding new deposits")
		}
		log.Debugf("added %d deposits", count)
	}()

	chRes <- FragmenterSplitFundsReply{
		Msg: "fragmentation succeeded",
	}
}

func (o *operatorService) WithdrawFeeFragmenterFunds(
	ctx context.Context, addr string, millisatsPerByte uint64,
) (string, error) {
	balance, err := o.wallet.BalanceForAccount(ctx, FeeFragmenterAccount)
	if err != nil {
		return "", err
	}

	outputs := make(Outputs, 0, len(balance))
	for asset, value := range balance {
		outputs = append(outputs, NewOutput(addr, asset, value.Total()))
	}
	_, txid, err := o.wallet.SendToManyWithFeeTopup(
		ctx, FeeFragmenterAccount, outputs, millisatsPerByte,
	)
	if err != nil {
		return "", err
	}

	go func() {
		if _, err := o.repoManager.WithdrawalRepository().AddWithdrawals(ctx, []domain.Withdrawal{
			{
				TxID:              txid,
				AccountName:       FeeFragmenterAccount,
				Outputs:           outputs.toDomainList(),
				MillisatPerByte:   millisatsPerByte,
				TotAmountPerAsset: outputs.totAmountPerAsset(),
				Timestamp:         uint64(time.Now().Unix()),
			},
		}); err != nil {
			log.WithError(err).Warn("an error occured while adding withdrawal info")
		}
		log.Debug("added 1 withdrawal")
	}()

	return txid, nil
}

func (o *operatorService) GetMarketFragmenterAddress(
	ctx context.Context, numOfAddresses int,
) ([]AddressAndBlindingKey, error) {
	if numOfAddresses <= 0 {
		numOfAddresses = 1
	}
	return o.wallet.DeriveAddressForAccount(
		ctx, MarketFragmenterAccount, uint64(numOfAddresses),
	)
}

func (o *operatorService) ListMarketFragmenterExternalAddresses(
	ctx context.Context,
) ([]AddressAndBlindingKey, error) {
	return o.wallet.ListAddressesForAccount(ctx, MarketFragmenterAccount)
}

func (o *operatorService) GetMarketFragmenterBalance(
	ctx context.Context,
) (map[string]ports.Balance, error) {
	return o.wallet.BalanceForAccount(ctx, MarketFragmenterAccount)
}

func (o *operatorService) MarketFragmenterSplitFunds(
	ctx context.Context, market Market, millisatsPerByte uint64,
	chRes chan FragmenterSplitFundsReply,
) {
	defer close(chRes)

	if err := market.Validate(); err != nil {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf("invalid market: %s", err),
		}
		return
	}

	mkt, accountIndex, err := o.repoManager.MarketRepository().GetMarketByAssets(
		ctx, market.BaseAsset, market.QuoteAsset,
	)
	if err != nil {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf("failed to retrieve market: %s", err),
		}
		return
	}
	if accountIndex < 0 {
		chRes <- FragmenterSplitFundsReply{
			Err: ErrMarketNotExist,
		}
		return
	}

	chRes <- FragmenterSplitFundsReply{
		Msg: fmt.Sprintf("fetching %s funds", MarketFragmenterAccount),
	}

	fragmenterBalance, err := o.wallet.BalanceForAccount(
		ctx, MarketFragmenterAccount,
	)
	if err != nil {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"error while fetching %s funds: %s", MarketFragmenterAccount, err,
			),
		}
		return
	}
	if fragmenterBalance == nil {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf("no funds detected for %s", MarketFragmenterAccount),
		}
		return
	}

	if len(fragmenterBalance) != 2 {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"fetched funds with assets different from the market pair. " +
					"You need to withdraw them to proceed with the fragmentation",
			),
		}
		return
	}

	baseAssetBalance := fragmenterBalance[mkt.BaseAsset]
	quoteAssetBalance := fragmenterBalance[mkt.QuoteAsset]
	if baseAssetBalance.Total() > 0 &&
		baseAssetBalance.Total() < uint64(MinMarketFragmenterAmount) {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"base asset amount to fragment %d is too small. "+
					"Must be at least %d sats. Top-up with other funds to proceed with "+
					"fragmentation or either withdraw those already deposited to abort",
				baseAssetBalance.Total(), MinMarketFragmenterAmount,
			),
		}
		return
	}
	if quoteAssetBalance.Total() > 0 &&
		quoteAssetBalance.Total() < uint64(MinMarketFragmenterAmount) {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"quote asset amount to fragment %d is too small. "+
					"Must be at least %d sats. Top-up with other funds to proceed with "+
					"fragmentation or either withdraw those already deposited to abort",
				quoteAssetBalance.Total(), MinMarketFragmenterAmount,
			),
		}
		return
	}

	// If market has balanced strategy and zero balance, it's mandatory to fund
	// the market fragmenter account with funds of both assets.
	if mkt.IsStrategyBalanced() {
		chRes <- FragmenterSplitFundsReply{
			Msg: "market with balanced strategy. Fetching market funds",
		}

		mktBalance, err := o.wallet.BalanceForAccount(ctx, mkt.Name)
		if err != nil {
			chRes <- FragmenterSplitFundsReply{
				Err: fmt.Errorf("error while fetching market funds: %s", err),
			}
			return
		}

		mktIsZeroBaseBalance := mktBalance == nil ||
			mktBalance[mkt.BaseAsset].Total() == 0
		if mktIsZeroBaseBalance && baseAssetBalance.Total() == 0 {
			chRes <- FragmenterSplitFundsReply{
				Err: fmt.Errorf("missing base funds to deposit to market account"),
			}
			return
		}
		mktIsZeroQuoteBalance := mktBalance == nil ||
			mktBalance[mkt.QuoteAsset].Total() == 0
		if mktIsZeroQuoteBalance && quoteAssetBalance.Total() == 0 {
			chRes <- FragmenterSplitFundsReply{
				Err: fmt.Errorf("missing quote funds to deposit to market account"),
			}
			return
		}
	}

	chRes <- FragmenterSplitFundsReply{
		Msg: fmt.Sprintf("fetching %s funds", FeeAccount),
	}
	feeBalance, err := o.wallet.BalanceForAccount(ctx, FeeAccount)
	if err != nil {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"error while fetching fee account funds: %s", err,
			),
		}
		return
	}
	if feeBalance == nil || feeBalance[o.wallet.NativeAsset()].Total() == 0 {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"no funds detected for %s.\n"+
					"You need to deposit some LBTC funds to this account used to "+
					"pay for network fees of transactions", FeeAccount,
			),
		}
		return
	}

	assetValuePair := pair{
		baseAsset:  mkt.BaseAsset,
		baseValue:  baseAssetBalance.Total(),
		quoteAsset: mkt.QuoteAsset,
		quoteValue: quoteAssetBalance.Total(),
	}

	baseFragments, quoteFragments, _ := marketFragmentAmount(
		assetValuePair, FragmentationMap,
	)
	numOuts := len(baseFragments) + len(quoteFragments)

	if len(baseFragments) <= 0 {
		chRes <- FragmenterSplitFundsReply{
			Msg: "no base asset funds detected",
		}
	} else {
		chRes <- FragmenterSplitFundsReply{
			Msg: fmt.Sprintf(
				"fetched funds of base asset with total amount %d",
				baseAssetBalance.Total(),
			),
		}
		chRes <- FragmenterSplitFundsReply{
			Msg: fmt.Sprintf(
				"splitting base asset funds of total amount %d into %d fragments",
				baseAssetBalance.Total(), len(baseFragments),
			),
		}
	}

	if len(quoteFragments) <= 0 {
		chRes <- FragmenterSplitFundsReply{
			Msg: "no quote asset funds detected",
		}
	} else {
		chRes <- FragmenterSplitFundsReply{
			Msg: fmt.Sprintf(
				"fetched funds of quote asset with total amount %d",
				quoteAssetBalance.Total(),
			),
		}
		chRes <- FragmenterSplitFundsReply{
			Msg: fmt.Sprintf(
				"splitting quote asset funds of total amount %d into %d fragments",
				quoteAssetBalance.Total(), len(quoteFragments),
			),
		}
	}

	chRes <- FragmenterSplitFundsReply{
		Msg: "creating ouputs for fragments",
	}
	outputs := make(Outputs, 0, numOuts)
	addresses, err := o.wallet.DeriveAddressForAccount(
		ctx, mkt.Name, uint64(numOuts),
	)
	if err != nil {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf("error while creating outputs for fragments: %s", err),
		}
		return
	}
	i := 0
	for _, amount := range baseFragments {
		outputs = append(outputs, Output{
			mkt.BaseAsset, amount, addresses[i].Address,
		})
	}
	for _, amount := range quoteFragments {
		outputs = append(outputs, Output{mkt.QuoteAsset, amount, addresses[i].Address})
	}

	chRes <- FragmenterSplitFundsReply{
		Msg: "creating transaction to deposit funds to market account",
	}
	_, txid, err := o.wallet.SendToManyWithFeeTopup(
		ctx, MarketFragmenterAccount, outputs, millisatsPerByte,
	)
	if err != nil {
		chRes <- FragmenterSplitFundsReply{
			Err: fmt.Errorf(
				"error while creating transaction to deposit funds to market "+
					"account: %s", err,
			),
		}
		return
	}

	chRes <- FragmenterSplitFundsReply{
		Msg: fmt.Sprintf("market funding transaction: %s", txid),
	}

	go func() {
		deposits := make([]domain.Deposit, 0, numOuts)
		now := uint64(time.Now().Unix())
		for i, o := range outputs {
			deposits = append(deposits, domain.Deposit{
				AccountName: mkt.Name,
				TxID:        txid,
				VOut:        i,
				Asset:       o.asset,
				Value:       o.value,
				Timestamp:   now,
			})
		}

		count, err := o.repoManager.DepositRepository().AddDeposits(ctx, deposits)
		if err != nil {
			log.WithError(err).Warn("an error occured while adding deposits info")
			return
		}
		log.Debugf("added %d deposits", count)
	}()

	chRes <- FragmenterSplitFundsReply{
		Msg: "fragmentation succeeded",
	}
}

func (o *operatorService) WithdrawMarketFragmenterFunds(
	ctx context.Context, addr string, millisatPerByte uint64,
) (string, error) {
	balance, err := o.wallet.BalanceForAccount(ctx, MarketFragmenterAccount)
	if err != nil {
		return "", err
	}

	outputs := make(Outputs, 0, len(balance))
	for asset, value := range balance {
		outputs = append(outputs, NewOutput(addr, asset, value.Total()))
	}

	_, txid, err := o.wallet.SendToManyWithFeeTopup(
		ctx, MarketFragmenterAccount, outputs, millisatPerByte,
	)
	return txid, err
}

// ListMarkets a set of informations about all the markets.
func (o *operatorService) ListMarkets(ctx context.Context) ([]MarketInfo, error) {
	markets, err := o.repoManager.MarketRepository().GetAllMarkets(ctx)
	if err != nil {
		return nil, err
	}

	marketInfo := make([]MarketInfo, 0, len(markets))
	for _, market := range markets {
		balance, err := o.wallet.BalanceForAccount(ctx, market.Name)
		if err != nil {
			return nil, err
		}

		marketInfo = append(marketInfo, MarketInfo{
			AccountIndex: uint64(market.AccountIndex),
			AccountName:  market.Name,
			Market: Market{
				BaseAsset:  market.BaseAsset,
				QuoteAsset: market.QuoteAsset,
			},
			Tradable:     market.Tradable,
			StrategyType: market.Strategy.Type,
			Price:        market.Price,
			Fee: Fee{
				BasisPoint:    market.Fee,
				FixedBaseFee:  market.FixedFee.BaseFee,
				FixedQuoteFee: market.FixedFee.QuoteFee,
			},
			Balance: balance,
		})
	}

	return marketInfo, nil
}

// ListTrades returns the list of all trads processed by the daemon
func (o *operatorService) ListTrades(
	ctx context.Context, page *Page,
) ([]TradeInfo, error) {
	var trades []*domain.Trade
	var err error
	if page == nil {
		trades, err = o.repoManager.TradeRepository().GetAllTrades(ctx)
	} else {
		pg := page.ToDomain()
		trades, err = o.repoManager.TradeRepository().GetAllTradesForPage(ctx, pg)
	}
	if err != nil {
		return nil, err
	}

	return tradesToTradeInfo(trades, o.marketBaseAsset, o.wallet.Network()), nil
}

func (o *operatorService) ListTradesForMarket(
	ctx context.Context, market Market, page *Page,
) ([]TradeInfo, error) {
	var trades []*domain.Trade
	var err error
	if page == nil {
		trades, err = o.repoManager.TradeRepository().GetAllTradesByMarket(
			ctx, market.QuoteAsset,
		)
	} else {
		pg := page.ToDomain()
		trades, err = o.repoManager.TradeRepository().GetAllTradesByMarketAndPage(
			ctx, market.QuoteAsset, pg,
		)
	}
	if err != nil {
		return nil, err
	}

	return tradesToTradeInfo(trades, market.BaseAsset, o.wallet.Network()), nil
}

func (o *operatorService) ListUtxos(
	ctx context.Context, account string, page *Page,
) ([]ports.Utxo, []ports.Utxo, error) {
	return o.wallet.AccountManager().ListUtxosForAccount(ctx, account)
}

func (o *operatorService) ListDeposits(
	ctx context.Context, accountName string, page *Page,
) (Deposits, error) {
	var deposits []domain.Deposit
	var err error
	if page == nil {
		deposits, err = o.repoManager.DepositRepository().ListDepositsForAccount(
			ctx, accountName,
		)
	} else {
		pg := page.ToDomain()
		deposits, err = o.repoManager.DepositRepository().ListDepositsForAccountAndPage(
			ctx, accountName, pg,
		)
	}
	if err != nil {
		return nil, err
	}

	return Deposits(deposits), nil
}

func (o *operatorService) ListWithdrawals(
	ctx context.Context, accountName string, page *Page,
) (Withdrawals, error) {
	var list []domain.Withdrawal
	var err error
	if page == nil {
		list, err = o.repoManager.WithdrawalRepository().ListWithdrawalsForAccount(
			ctx, accountName,
		)
	} else {
		pg := page.ToDomain()
		list, err = o.repoManager.WithdrawalRepository().
			ListWithdrawalsForAccountAndPage(ctx, accountName, pg)
	}
	if err != nil {
		return nil, err
	}

	withdrawals := make(Withdrawals, 0, len(list))
	for _, w := range list {
		withdrawals = append(withdrawals, Withdrawal(w))
	}

	return withdrawals, nil
}

func (o *operatorService) AddWebhook(
	_ context.Context, hook Webhook,
) (string, error) {
	if o.pubsubService == nil {
		return "", ErrPubSubServiceNotInitialized
	}

	topics := o.pubsubService.TopicsByCode()
	topic, ok := topics[hook.ActionType]
	if !ok {
		return "", ErrInvalidActionType
	}

	return o.pubsubService.Subscribe(
		topic.Label(), hook.Endpoint, hook.Secret,
	)
}

func (o *operatorService) RemoveWebhook(
	_ context.Context, hookID string,
) error {
	if o.pubsubService == nil {
		return ErrPubSubServiceNotInitialized
	}
	return o.pubsubService.Unsubscribe("", hookID)
}

func (o *operatorService) ListWebhooks(
	_ context.Context, actionType int,
) ([]WebhookInfo, error) {
	pubsubSvc := o.pubsubService
	if pubsubSvc == nil {
		return nil, ErrPubSubServiceNotInitialized
	}

	topics := pubsubSvc.TopicsByCode()
	topic, ok := topics[actionType]
	if !ok {
		return nil, ErrInvalidActionType
	}

	subs := pubsubSvc.ListSubscriptionsForTopic(topic.Label())
	hooks := make([]WebhookInfo, 0, len(subs))
	for _, s := range subs {
		hooks = append(hooks, WebhookInfo{
			Id:         s.Id(),
			ActionType: s.Topic().Code(),
			Endpoint:   s.NotifyAt(),
			IsSecured:  s.IsSecured(),
		})
	}
	return hooks, nil
}

func (o *operatorService) registerHandlerForWithdrawalEvent() {
	if o.pubsubService == nil {
		return
	}

	wallet := o.wallet
	repoManager := o.repoManager
	pubsubService := o.pubsubService
	feeAccountBalanceThreshold := o.feeAccountBalanceThreshold

	repoManager.RegisterHandlerForWithdrawalEvent(
		domain.NewWithdrawalEvent, func(event domain.WithdrawalEvent) {
			withdrawal := Withdrawal(event.Withdrawal)
			addresses := withdrawal.OutputAddresses()
			txid := withdrawal.TxID
			lbtc := wallet.NativeAsset()
			var market *domain.Market
			var marketBalance Balance
			var feeAccountBalance uint64

			balance, _ := wallet.BalanceForAccount(context.Background(), FeeAccount)
			if balance != nil {
				if b, ok := balance[lbtc]; ok {
					feeAccountBalance = b.Total()
				}
			}

			if event.Withdrawal.AccountName == FeeAccount {
				withdrewAmount := withdrawal.TotAmountPerAsset[lbtc]
				if withdrewAmount > 0 {
					publishFeeWithdrawTopic(
						pubsubService, feeAccountBalance, withdrewAmount, addresses[0],
						txid, lbtc,
					)
				}
				return
			}

			market, _, _ = repoManager.MarketRepository().GetMarketByName(
				context.Background(), withdrawal.AccountName,
			)
			if market != nil {
				balancePerAsset, _ := wallet.BalanceForAccount(context.Background(), market.Name)
				var baseAssetBalance, quoteAssetBalance uint64
				if balancePerAsset != nil {
					baseAssetBalance = balancePerAsset[market.BaseAsset].Total()
					quoteAssetBalance = balancePerAsset[market.QuoteAsset].Total()
				}
				marketBalance = Balance{
					BaseAmount:  baseAssetBalance,
					QuoteAmount: quoteAssetBalance,
				}
				withdrewAmount := Balance{
					BaseAmount:  withdrawal.TotAmountPerAsset[market.BaseAsset],
					QuoteAmount: withdrawal.TotAmountPerAsset[market.QuoteAsset],
				}
				publishMarketWithdrawTopic(
					pubsubService, market, marketBalance, withdrewAmount, addresses[0], txid,
				)
			}

			checkForFeeAndMarketLowBalances(
				pubsubService, feeAccountBalance, feeAccountBalanceThreshold,
				market, marketBalance,
			)
		},
	)
}

func tradesToTradeInfo(trades []*domain.Trade, marketBaseAsset, network string) []TradeInfo {
	tradeInfo := make([]TradeInfo, 0, len(trades))
	for _, trade := range trades {
		info := tradeToTradeInfo(trade, marketBaseAsset, network)
		if info != nil {
			tradeInfo = append(tradeInfo, *info)
		}
	}

	return tradeInfo
}

func tradeToTradeInfo(
	trade *domain.Trade, marketBaseAsset, net string,
) *TradeInfo {
	if trade.IsEmpty() {
		return nil
	}

	// to maintain backward compatibility, since trade.MarketBaseAsset has been
	// introduced only in versions above v0.7.1.
	mktBaseAsset := trade.MarketBaseAsset
	if len(mktBaseAsset) == 0 {
		mktBaseAsset = marketBaseAsset
	}

	info := &TradeInfo{
		ID:     trade.ID.String(),
		Status: trade.Status,
		MarketWithFee: MarketWithFee{
			Market{
				BaseAsset:  mktBaseAsset,
				QuoteAsset: trade.MarketQuoteAsset,
			},
			Fee{
				BasisPoint:    trade.MarketFee,
				FixedBaseFee:  trade.MarketFixedBaseFee,
				FixedQuoteFee: trade.MarketFixedQuoteFee,
			},
		},
		Price:            Price(trade.MarketPrice),
		RequestTimeUnix:  trade.SwapRequest.Timestamp,
		AcceptTimeUnix:   trade.SwapAccept.Timestamp,
		CompleteTimeUnix: trade.SwapComplete.Timestamp,
		SettleTimeUnix:   trade.SettlementTime,
		ExpiryTimeUnix:   trade.ExpiryTime,
	}

	if req := trade.SwapRequestMessage(); req != nil {
		info.SwapInfo = SwapInfo{
			AssetP:  req.GetAssetP(),
			AmountP: req.GetAmountP(),
			AssetR:  req.GetAssetR(),
			AmountR: req.GetAmountR(),
		}
	}

	if fail := trade.SwapFailMessage(); fail != nil {
		info.SwapFailInfo = SwapFailInfo{
			Code:    int(fail.GetFailureCode()),
			Message: fail.GetFailureMessage(),
		}
	}

	// if trade.IsSettled() {
	// 	_, outBlindingData, _ := TransactionManager.ExtractBlindingData(
	// 		trade.PsetBase64,
	// 		nil, trade.SwapAcceptMessage().GetOutputBlindingKey(),
	// 	)

	// 	var blinded string
	// 	for _, data := range outBlindingData {
	// 		blinded += fmt.Sprintf(
	// 			"%d,%s,%s,%s,",
	// 			data.Amount, data.Asset,
	// 			hex.EncodeToString(elementsutil.ReverseBytes(data.AmountBlinder)),
	// 			hex.EncodeToString(elementsutil.ReverseBytes(data.AssetBlinder)),
	// 		)
	// 	}
	// 	// remove trailing comma
	// 	blinded = strings.Trim(blinded, ",")

	// 	baseURL := fmt.Sprintf("%s/tx", esploraUrlByNetwork[net])
	// 	info.TxURL = fmt.Sprintf("%s/%s#blinded=%s", baseURL, trade.TxID, blinded)
	// }

	return info
}
