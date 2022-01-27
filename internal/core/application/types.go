package application

import (
	"fmt"
	"sync"

	"github.com/shopspring/decimal"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
)

type readyChan struct {
	lock    *sync.Mutex
	channel chan bool
}

func newReadyChan() readyChan {
	return readyChan{
		lock:    &sync.Mutex{},
		channel: make(chan bool, 1),
	}
}

func (c readyChan) send(val bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.channel <- val
}

func (c readyChan) close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	close(c.channel)
}

type pwChan struct {
	lock    *sync.Mutex
	channel chan PassphraseMsg
}

func newPwChan() pwChan {
	return pwChan{
		lock:    &sync.Mutex{},
		channel: make(chan PassphraseMsg, 1),
	}
}

func (c pwChan) send(msg PassphraseMsg) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.channel <- msg
}

func (c pwChan) close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	close(c.channel)
}

type PassphraseMsg struct {
	Method     int
	CurrentPwd string
	NewPwd     string
}

type InitWalletReply struct {
	Message string
	Err     error
}

// AccountInfo contains info about a wallet account.
type AccountInfo struct {
	Index               uint32
	DerivationPath      string
	Xpub                string
	LastExternalDerived uint32
	LastInternalDerived uint32
}

// SwapInfo contains info about a swap
type SwapInfo struct {
	AmountP uint64
	AssetP  string
	AmountR uint64
	AssetR  string
}

type SwapFailInfo struct {
	Code    int
	Message string
}

// TradeInfo contains info about a trade.
type TradeInfo struct {
	ID               string
	Status           domain.Status
	SwapInfo         SwapInfo
	SwapFailInfo     SwapFailInfo
	MarketWithFee    MarketWithFee
	Price            Price
	TxURL            string
	RequestTimeUnix  uint64
	AcceptTimeUnix   uint64
	CompleteTimeUnix uint64
	SettleTimeUnix   uint64
	ExpiryTimeUnix   uint64
}

// MarketInfo is the data struct returned by ListMarket RPC.
type MarketInfo struct {
	AccountIndex uint64
	AccountName  string
	Market       Market
	Fee          Fee
	Tradable     bool
	StrategyType int
	Price        domain.Prices
	Balance      map[string]ports.Balance
}

type Market struct {
	BaseAsset  string
	QuoteAsset string
}

func (m Market) Validate() error {
	if err := validateAssetString(m.BaseAsset); err != nil {
		return ErrMarketInvalidBaseAsset
	}
	if err := validateAssetString(m.QuoteAsset); err != nil {
		return ErrMarketInvalidQuoteAsset
	}
	if m.BaseAsset == m.QuoteAsset {
		return fmt.Errorf("quote asset must not be equal to base asset")
	}
	return nil
}

func (m Market) Name() string {
	return domain.MarketName(m.BaseAsset, m.QuoteAsset)
}

type Fee struct {
	BasisPoint    int64
	FixedBaseFee  int64
	FixedQuoteFee int64
}

type MarketWithFee struct {
	Market
	Fee
}

type MarketWithPrice struct {
	Market
	Price
}

type Price struct {
	BasePrice  decimal.Decimal
	QuotePrice decimal.Decimal
}

func (p Price) Validate() error {
	zero := decimal.NewFromInt(0)
	if p.BasePrice.LessThanOrEqual(zero) {
		return domain.ErrMarketInvalidBasePrice
	}
	if p.QuotePrice.LessThanOrEqual(zero) {
		return domain.ErrMarketInvalidQuotePrice
	}
	return nil
}

type PriceWithFee struct {
	Price   Price
	Fee     Fee
	Amount  uint64
	Asset   string
	Balance Balance
}

type MarketStrategy struct {
	Market
	Strategy domain.StrategyType
}

type Balance struct {
	BaseAmount  uint64
	QuoteAmount uint64
}

type BalanceWithFee struct {
	Balance Balance
	Fee     Fee
}

type BalanceInfo struct {
	TotalBalance       uint64
	ConfirmedBalance   uint64
	UnconfirmedBalance uint64
}

type FragmenterSplitFundsReply struct {
	Msg string
	Err error
}

type WithdrawMarketReq struct {
	Market
	BalanceToWithdraw Balance
	MillisatPerByte   int64
	Address           string
	Push              bool
}

func (r WithdrawMarketReq) Validate() error {
	if err := validateAssetString(r.BaseAsset); err != nil {
		return ErrMarketInvalidBaseAsset
	}
	if err := validateAssetString(r.QuoteAsset); err != nil {
		return ErrMarketInvalidQuoteAsset
	}
	if r.BalanceToWithdraw.BaseAmount == 0 && r.BalanceToWithdraw.QuoteAmount == 0 {
		return ErrMissingMarketBalanceToWithdraw
	}
	if r.Address == "" {
		return ErrMissingWithdrawAddress
	}
	return nil
}

type ReportMarketFee struct {
	CollectedFees              []FeeInfo
	TotalCollectedFeesPerAsset map[string]int64
}

type AddressAndBlindingKey struct {
	Address     string
	BlindingKey string
}

type FeeInfo struct {
	TradeID             string
	BasisPoint          int64
	Asset               string
	PercentageFeeAmount uint64
	FixedFeeAmount      uint64
	MarketPrice         decimal.Decimal
}

type Utxo struct {
	txid  string
	index uint32
}

func NewUtxo(txid string, index uint32) Utxo {
	return Utxo{txid, index}
}

func (u Utxo) TxID() string {
	return u.txid
}

func (u Utxo) Index() uint32 {
	return u.index
}

type Output struct {
	asset   string
	value   uint64
	address string
}

func (o Output) Asset() string {
	return o.asset
}

func (o Output) Value() uint64 {
	return o.value
}

func (o Output) Address() string {
	return o.address
}

func (o Output) toDomain() domain.Output {
	return domain.Output{
		Asset:   o.asset,
		Value:   o.value,
		Address: o.address,
	}
}

type Outputs []Output

func (outputs Outputs) totAmountPerAsset() map[string]uint64 {
	totAmountPerAsset := make(map[string]uint64)
	for _, o := range outputs {
		totAmountPerAsset[o.Asset()] += uint64(o.Value())
	}
	return totAmountPerAsset
}

func (outputs Outputs) toPortableList() []ports.Output {
	list := make([]ports.Output, 0, len(outputs))
	for _, o := range outputs {
		list = append(list, o)
	}
	return list
}

func (outputs Outputs) toDomainList() []domain.Output {
	list := make([]domain.Output, 0, len(outputs))
	for _, o := range outputs {
		list = append(list, o.toDomain())
	}
	return list
}

func NewOutput(address, asset string, value uint64) Output {
	return Output{asset, value, address}
}

type UtxoInfoList struct {
	Unspents []UtxoInfo
	Spents   []UtxoInfo
	Locks    []UtxoInfo
}

type UtxoInfo struct {
	Outpoint *Utxo
	Value    uint64
	Asset    string
}

type Webhook struct {
	ActionType int
	Endpoint   string
	Secret     string
}
type WebhookInfo struct {
	Id         string
	ActionType int
	Endpoint   string
	IsSecured  bool
}

type Page domain.Page

func (p *Page) ToDomain() domain.Page {
	return domain.NewPage(p.Number, p.Size)
}

type Deposits []domain.Deposit
type Withdrawal domain.Withdrawal

func (w Withdrawal) OutputAddresses() []string {
	addresses := make([]string, 0)
	addrMap := make(map[string]struct{})
	for _, output := range w.Outputs {
		addrMap[output.Address] = struct{}{}
	}
	for addr := range addrMap {
		addresses = append(addresses, addr)
	}
	return addresses
}

type Withdrawals []Withdrawal
