package domain

import (
	"encoding/hex"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/shopspring/decimal"
	mm "github.com/tdex-network/tdex-daemon/pkg/marketmaking"
	"github.com/tdex-network/tdex-daemon/pkg/marketmaking/formula"
)

type FixedFee struct {
	BaseFee  int64
	QuoteFee int64
}

// Market defines the Market entity data structure for holding an asset pair state
type Market struct {
	// AccountIndex links a market to a HD wallet account derivation.
	AccountIndex uint64
	Name         string
	BaseAsset    string
	QuoteAsset   string
	// Each Market has a different fee expressed in basis point of each swap.
	Fee      int64
	FixedFee FixedFee
	// if curretly open for trades
	Tradable bool
	// Market Making strategy
	Strategy mm.MakingStrategy
	// Pluggable Price of the asset pair.
	Price Prices
}

// OutpointWithAsset contains the transaction outpoint (tx hash and vout) along with the asset hash
type OutpointWithAsset struct {
	Asset string
	Txid  string
	Vout  int
}

// Prices ...
type Prices struct {
	// how much 1 base asset is valued in quote asset.
	BasePrice decimal.Decimal
	// how much 1 quote asset is valued in base asset
	QuotePrice decimal.Decimal
}

// StrategyType is the Market making strategy type
type StrategyType int32

// PreviewInfo contains info about a price preview based on the market's current
// strategy.
type PreviewInfo struct {
	Price  Prices
	Amount uint64
	Asset  string
}

// NewMarket returns a new market with an account index, the asset pair and the
// percentage fee set.
func NewMarket(
	accountIndex uint64, baseAsset, quoteAsset string, feeInBasisPoint int64,
) (*Market, error) {
	if !isValidAsset(baseAsset) {
		return nil, ErrMarketInvalidBaseAsset
	}
	if !isValidAsset(quoteAsset) {
		return nil, ErrMarketInvalidQuoteAsset
	}
	if err := validateFee(feeInBasisPoint); err != nil {
		return nil, err
	}

	name := MarketName(baseAsset, quoteAsset)
	return &Market{
		AccountIndex: accountIndex,
		Name:         name,
		BaseAsset:    baseAsset,
		QuoteAsset:   quoteAsset,
		Fee:          feeInBasisPoint,
		Strategy:     mm.NewStrategyFromFormula(formula.BalancedReserves{}),
	}, nil
}

func MarketName(baseAsset, quoteAsset string) string {
	buf, _ := hex.DecodeString(baseAsset + quoteAsset)
	h := chainhash.DoubleHashB(buf)
	return hex.EncodeToString(h[:4])
}

func isValidAsset(asset string) bool {
	buf, err := hex.DecodeString(asset)
	if err != nil {
		return false
	}
	return len(buf) == 32
}
