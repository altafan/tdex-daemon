package inmemory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/tdex-network/tdex-daemon/internal/core/domain"
)

// marketRepositoryImpl represents an in memory storage
type marketRepositoryImpl struct {
	store *marketInmemoryStore
}

func NewMarketRepositoryImpl(
	store *marketInmemoryStore,
) domain.MarketRepository {
	return &marketRepositoryImpl{store}
}

func (r marketRepositoryImpl) GetOrCreateMarket(
	_ context.Context, market *domain.Market,
) (*domain.Market, error) {
	r.store.locker.Lock()
	defer r.store.locker.Unlock()

	return r.getOrCreateMarket(market)
}

func (r marketRepositoryImpl) GetMarketByAccount(
	_ context.Context, accountIndex uint64,
) (*domain.Market, error) {
	r.store.locker.Lock()
	defer r.store.locker.Unlock()

	return r.getMarketByAccount(accountIndex)
}

func (r marketRepositoryImpl) GetMarketByName(
	_ context.Context, accountName string,
) (*domain.Market, int, error) {
	r.store.locker.Lock()
	defer r.store.locker.Unlock()

	return r.getMarketByName(accountName)
}

func (r marketRepositoryImpl) GetMarketByAssets(
	_ context.Context, baseAsset, quoteAsset string,
) (market *domain.Market, accountIndex int, err error) {
	r.store.locker.Lock()
	defer r.store.locker.Unlock()

	return r.getMarketByAssets(baseAsset, quoteAsset)
}

func (r marketRepositoryImpl) GetTradableMarkets(
	_ context.Context,
) (tradableMarkets []domain.Market, err error) {
	r.store.locker.Lock()
	defer r.store.locker.Unlock()

	for _, mkt := range r.store.markets {
		if mkt.IsTradable() {
			tradableMarkets = append(tradableMarkets, mkt)
		}
	}

	return tradableMarkets, nil
}

func (r marketRepositoryImpl) GetAllMarkets(
	_ context.Context,
) ([]domain.Market, error) {
	r.store.locker.Lock()
	defer r.store.locker.Unlock()

	markets := make([]domain.Market, 0)

	for _, mkt := range r.store.markets {
		markets = append(markets, mkt)
	}

	return markets, nil
}

func (r marketRepositoryImpl) UpdateMarket(
	_ context.Context, accountIndex uint64,
	updateFn func(m *domain.Market) (*domain.Market, error),
) error {
	r.store.locker.Lock()
	defer r.store.locker.Unlock()

	currentMarket, err := r.getMarketByAccount(accountIndex)
	if err != nil {
		return err
	}
	if currentMarket == nil {
		return ErrMarketNotFound
	}

	updatedMarket, err := updateFn(currentMarket)
	if err != nil {
		return err
	}

	key := keyFromAssets(updatedMarket.BaseAsset, updatedMarket.QuoteAsset)
	r.store.markets[accountIndex] = *updatedMarket
	r.store.accountsByAssetsKey[key] = accountIndex

	return nil
}

func (r marketRepositoryImpl) OpenMarket(
	_ context.Context, accountIndex uint64,
) error {
	r.store.locker.Lock()
	defer r.store.locker.Unlock()

	currentMarket, err := r.getMarketByAccount(accountIndex)
	if err != nil {
		return err
	}
	if currentMarket == nil {
		return nil
	}

	// We update the market status only if the market is closed.
	if currentMarket.IsTradable() {
		return nil
	}

	err = currentMarket.MakeTradable()
	if err != nil {
		return err
	}

	r.store.markets[accountIndex] = *currentMarket

	return nil
}

func (r marketRepositoryImpl) CloseMarket(
	_ context.Context, accountIndex uint64,
) error {
	r.store.locker.Lock()
	defer r.store.locker.Unlock()

	currentMarket, err := r.getMarketByAccount(accountIndex)
	if err != nil {
		return err
	}

	// We update the market status only if the market is open.
	if !currentMarket.IsTradable() {
		return nil
	}

	err = currentMarket.MakeNotTradable()
	if err != nil {
		return err
	}

	r.store.markets[accountIndex] = *currentMarket

	return nil
}

func (r *marketRepositoryImpl) UpdatePrices(
	_ context.Context, accountIndex uint64, prices domain.Prices,
) error {
	r.store.locker.Lock()
	defer r.store.locker.Unlock()

	market, err := r.getMarketByAccount(accountIndex)
	if err != nil {
		return err
	}

	err = market.ChangeBasePrice(prices.BasePrice)
	if err != nil {
		return err
	}
	err = market.ChangeQuotePrice(prices.QuotePrice)
	if err != nil {
		return err
	}

	r.store.markets[accountIndex] = *market
	return nil
}

func (r *marketRepositoryImpl) DeleteMarket(
	_ context.Context, accountIndex uint64,
) error {
	delete(r.store.markets, accountIndex)

	return nil
}

func (r marketRepositoryImpl) getOrCreateMarket(
	market *domain.Market,
) (*domain.Market, error) {
	if market == nil {
		return nil, ErrMarketInvalidRequest
	}

	// we can safely skip checking the error here because this functions never
	// returns one actually. The err variable is only needed to be able to
	// override mkt into the if statement.
	mkt, err := r.getMarketByAccount(market.AccountIndex)
	if err != nil {
		return nil, err
	}
	if mkt == nil {
		key := keyFromAssets(market.BaseAsset, market.QuoteAsset)
		r.store.markets[market.AccountIndex] = *market
		r.store.accountsByAssetsKey[key] = market.AccountIndex
		r.store.accountsByName[market.Name] = market.AccountIndex
		mkt = market
	}
	return mkt, nil
}

func (r marketRepositoryImpl) getMarketByAccount(
	accountIndex uint64,
) (*domain.Market, error) {
	market, ok := r.store.markets[accountIndex]
	if !ok {
		return nil, nil
	}

	return &market, nil
}

func (r marketRepositoryImpl) getMarketByName(
	accountName string,
) (*domain.Market, int, error) {
	accountIndex, ok := r.store.accountsByName[accountName]
	if !ok {
		return nil, -1, nil
	}
	currentMarket, ok := r.store.markets[accountIndex]
	if !ok {
		return nil, -1, nil
	}
	return &currentMarket, int(accountIndex), nil
}
func (r marketRepositoryImpl) getMarketByAssets(
	baseAsset, quoteAsset string,
) (*domain.Market, int, error) {
	key := keyFromAssets(baseAsset, quoteAsset)
	selectedAccountIndex, assetExist := r.store.accountsByAssetsKey[key]
	if !assetExist {
		return nil, -1, nil
	}
	currentMarket, ok := r.store.markets[selectedAccountIndex]
	if !ok {
		return nil, -1, nil
	}
	return &currentMarket, int(selectedAccountIndex), nil
}

func keyFromAssets(baseAsset, quoteAsset string) string {
	key := baseAsset + quoteAsset
	keyBytes := sha256.Sum256([]byte(key))
	return hex.EncodeToString(keyBytes[:])
}
