package application

import (
	"context"
	"fmt"
	"sync"

	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
)

const (
	FeeAccount              = "fee_account"
	PersonalAccount         = "personal_account"
	FeeFragmenterAccount    = "fee_fragmenter_account"
	MarketFragmenterAccount = "market_fragmenter_account"

	utxoEventUnknown utxoEventType = iota
	utxoEventLocked
	utxoEventUnlocked
	utxoEventSpent

	txEventUnknown txEventType = iota
	txEventBroadcasted
	txEventUnconfirmed
	txEventConfirmed
)

// Wallet interface is meant for testing purpose, to have the ability to mock
// the detached wallet service.
type Wallet interface {
	// managers to interact directly with underlying ocean wallet
	WalletManager() ports.WalletManager
	AccountManager() ports.AccountManager
	TransactionManager() ports.TransactionManager
	NotificationManager() ports.NotificationManager
	// settings
	Network() string
	NativeAsset() string
	// top-level functions
	DeriveAddressForAccount(
		ctx context.Context, account string, numOfAddresses uint64,
	) ([]AddressAndBlindingKey, error)
	ListAddressesForAccount(
		ctx context.Context, account string,
	) ([]AddressAndBlindingKey, error)
	BalanceForAccount(
		ctx context.Context, account string,
	) (map[string]ports.Balance, error)
	SendToManyWithFeeTopup(
		ctx context.Context, account string, outs Outputs, millisatPerByte uint64,
	) (string, string, error)
	FillSwapTransaction(
		ctx context.Context, account string, swapRequest domain.SwapRequest,
	) (string, []ports.UtxoKey, map[string][]byte, map[string][]byte, error)
	RegisterHandlerForUtxoEvent(
		handler UtxoNotificationHandler,
	)
	RegisterHandlerForTxEvent(
		handler TxNotificationHandler,
	)
}

type wallet struct {
	oceanWallet ports.OceanWallet
	network     string
	nativeAsset string

	UtxoNotificationHandlers utxoNotificationQueue
	TxNotificationHandlers   txNotificationQueue
}

func NewWallet(w ports.OceanWallet) (Wallet, error) {
	ctx := context.Background()
	info, err := w.WalletManager().GetInfo(ctx)
	if err != nil {
		return nil, err
	}
	network, err := portableNetworkToString(info.Network())
	if err != nil {
		return nil, err
	}

	nativeAsset := info.NativeAsset()
	utxoNotificationQueue := utxoNotificationQueue{}
	txNotificationQueue := txNotificationQueue{}
	wallet := &wallet{
		w, network, nativeAsset, utxoNotificationQueue, txNotificationQueue,
	}

	go wallet.listenToUtxoNotifications()
	go wallet.listenToTxNotifications()

	return wallet, nil
}

func (w *wallet) WalletManager() ports.WalletManager {
	return w.oceanWallet.WalletManager()
}

func (w *wallet) AccountManager() ports.AccountManager {
	return w.oceanWallet.AccountManager()
}

func (w *wallet) TransactionManager() ports.TransactionManager {
	return w.oceanWallet.TransactionManager()
}

func (w *wallet) NotificationManager() ports.NotificationManager {
	return w.oceanWallet.NotificationManager()
}

func (w *wallet) Network() string {
	return w.network
}

func (w *wallet) NativeAsset() string {
	return w.nativeAsset
}

func (w *wallet) DeriveAddressForAccount(
	ctx context.Context, account string, numOfAddresses uint64,
) ([]AddressAndBlindingKey, error) {
	if err := w.createAccountIfNotExists(ctx, account); err != nil {
		return nil, err
	}

	addresses, err := w.AccountManager().DeriveAddressesForAccount(
		ctx, account, numOfAddresses,
	)
	if err != nil {
		return nil, err
	}
	return toAddressAndBlindingKeyList(addresses), nil
}

func (w *wallet) ListAddressesForAccount(
	ctx context.Context, account string,
) ([]AddressAndBlindingKey, error) {
	exists, err := w.accountExists(ctx, account)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	addresses, err := w.AccountManager().ListAddressesForAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	return toAddressAndBlindingKeyList(addresses), nil
}

func (w *wallet) BalanceForAccount(
	ctx context.Context, account string,
) (map[string]ports.Balance, error) {
	exists, err := w.accountExists(ctx, account)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	return w.AccountManager().BalanceForAccount(ctx, account)
}

func (w *wallet) SendToManyWithFeeTopup(
	ctx context.Context, account string, outs Outputs, millisatPerByte uint64,
) (string, string, error) {
	accountManager := w.AccountManager()
	txManager := w.TransactionManager()
	totAmountPerAsset := outs.totAmountPerAsset()
	inputs := make([]ports.Input, 0)
	outputs := outs.toPortableList()
	changeAmountPerAsset := make(map[string]uint64)
	changeAddressPerAsset := make(map[string]string)

	for asset, amount := range totAmountPerAsset {
		utxos, change, err := txManager.SelectUnspentsForAccount(
			ctx, account, asset, amount, nil,
		)
		if err != nil {
			return "", "", err
		}
		for _, utxo := range utxos {
			inputs = append(inputs, Utxo{utxo.TxID(), utxo.Index()})
		}
		if change > 0 {
			changeAmountPerAsset[asset] = change
			changeAddressPerAsset[asset] = ""
		}
	}

	if numOfAddresses := len(changeAddressPerAsset); numOfAddresses > 0 {
		addresses, err := w.DeriveAddressForAccount(ctx, account, uint64(numOfAddresses))
		if err != nil {
			return "", "", err
		}

		i := 0
		for asset, amount := range changeAmountPerAsset {
			outputs = append(outputs, Output{asset, amount, addresses[i].Address})
			i++
		}
	}

	dummyFeeAmount := uint64(700)
	utxos, change, err := txManager.SelectUnspentsForAccount(
		ctx, FeeAccount, w.nativeAsset, dummyFeeAmount, nil,
	)
	if err != nil {
		return "", "", err
	}

	for _, u := range utxos {
		inputs = append(inputs, Utxo{u.TxID(), u.Index()})
	}
	if change > 0 {
		addr, err := accountManager.DeriveChangeAddressForAccount(ctx, FeeAccount, 1)
		if err != nil {
			return "", "", err
		}
		outputs = append(outputs, Output{w.nativeAsset, change, addr[0].Address()})
	}

	// TODO: ensure feeAmount <= dummyFeeAmount, otherwise TBD.
	feeAmount, err := txManager.EstimateFees(ctx, inputs, outputs)
	if err != nil {
		return "", "", err
	}
	outputs[len(outputs)-1] = Output{
		asset:   w.nativeAsset,
		value:   outputs[len(outputs)-1].Value() + (dummyFeeAmount - feeAmount),
		address: outputs[len(outputs)-1].Address(),
	}

	outputs = append(outputs, Output{w.nativeAsset, feeAmount, ""})

	pset, err := txManager.CreateTransaction(ctx, inputs, outputs)
	if err != nil {
		return "", "", err
	}

	blindedPset, err := txManager.BlindTransaction(ctx, pset, true)
	if err != nil {
		return "", "", err
	}

	txHex, err := txManager.SignTransaction(ctx, blindedPset, true)
	if err != nil {
		return "", "", err
	}

	txid, err := txManager.BroadcastTransaction(ctx, txHex)
	if err != nil {
		return "", "", err
	}

	return txHex, txid, nil
}

func (w *wallet) FillSwapTransaction(
	ctx context.Context, account string, swapRequest domain.SwapRequest,
) (string, []ports.UtxoKey, map[string][]byte, map[string][]byte, error) {
	inputs := make([]ports.Input, 0)
	utxos, change, err := w.TransactionManager().SelectUnspentsForAccount(
		ctx, account, swapRequest.GetAssetR(), swapRequest.GetAmountR(), nil,
	)
	if err != nil {
		return "", nil, nil, nil, err
	}
	inputs = append(inputs, utxos...)

	addresses, err := w.DeriveAddressForAccount(ctx, account, 1)
	if err != nil {
		return "", nil, nil, nil, err
	}
	outputs := Outputs{
		{swapRequest.GetAssetP(), swapRequest.GetAmountP(), addresses[0].Address},
	}
	if change > 0 {
		addresses, err := w.AccountManager().DeriveChangeAddressForAccount(
			ctx, account, 1,
		)
		if err != nil {
			return "", nil, nil, nil, err
		}
		outputs = append(outputs, Output{
			swapRequest.GetAssetP(), change, addresses[0].Address(),
		})
	}

	dummyFeeAmount := uint64(800)
	feeUtxos, feeChange, err := w.TransactionManager().SelectUnspentsForAccount(
		ctx, FeeAccount, w.nativeAsset, dummyFeeAmount, nil,
	)
	if err != nil {
		return "", nil, nil, nil, err
	}
	inputs = append(inputs, feeUtxos...)

	if feeChange > 0 {
		addresses, err := w.AccountManager().DeriveChangeAddressForAccount(
			ctx, FeeAccount, 1,
		)
		if err != nil {
			return "", nil, nil, nil, err
		}
		outputs = append(outputs, Output{
			w.nativeAsset, feeChange, addresses[0].Address(),
		})
	}

	feeAmount, err := w.TransactionManager().EstimateFees(
		ctx, inputs, outputs.toPortableList(),
	)
	if err != nil {
		return "", nil, nil, nil, err
	}

	// TODO: ensure feeAmount <= dummyFeeAmount, otherwise TBD.

	outputs[len(outputs)-1] = Output{
		asset:   w.nativeAsset,
		value:   outputs[len(outputs)-1].Value() + (dummyFeeAmount - feeAmount),
		address: outputs[len(outputs)-1].Address(),
	}

	outputs = append(outputs, Output{w.nativeAsset, feeAmount, ""})

	updatedPset, inBlindKeys, outBlindKeys, err := w.TransactionManager().
		UpdateTransaction(
			ctx, swapRequest.GetTransaction(), inputs, outputs.toPortableList(),
		)
	if err != nil {
		return "", nil, nil, nil, err
	}

	blindedPset, err := w.TransactionManager().BlindTransaction(
		ctx, updatedPset, true,
	)
	if err != nil {
		return "", nil, nil, nil, err
	}

	signedPset, err := w.TransactionManager().SignTransaction(ctx, blindedPset, false)
	if err != nil {
		return "", nil, nil, nil, err
	}

	for script, key := range swapRequest.GetInputBlindingKey() {
		inBlindKeys[script] = key
	}
	for script, key := range swapRequest.GetOutputBlindingKey() {
		outBlindKeys[script] = key
	}
	return signedPset, inputs, inBlindKeys, outBlindKeys, nil
}

func (w *wallet) RegisterHandlerForUtxoEvent(
	handler UtxoNotificationHandler,
) {
	w.UtxoNotificationHandlers.pushBack(handler)
}

func (w *wallet) RegisterHandlerForTxEvent(
	handler TxNotificationHandler,
) {
	w.TxNotificationHandlers.pushBack(handler)
}

func (w *wallet) listenToUtxoNotifications() {
	for notification := range w.NotificationManager().UtxoChannel() {
		if _, ok := parseUtxoEventType(notification.EventType()); ok {
			toRepeat := make([]UtxoNotificationHandler, 0)
			for i := 0; i < w.UtxoNotificationHandlers.len(); i++ {
				handler := w.UtxoNotificationHandlers.pop()
				done := handler(notification)
				if !done {
					toRepeat = append(toRepeat, handler)
				}
			}
			for _, handler := range toRepeat {
				w.UtxoNotificationHandlers.pushBack(handler)
			}
		}
	}
}

func (w *wallet) listenToTxNotifications() {
	for notification := range w.NotificationManager().TxChannel() {
		if _, ok := parseTxEventType(notification.EventType()); ok {
			toRepeat := make([]TxNotificationHandler, 0)
			for i := 0; i < w.TxNotificationHandlers.len(); i++ {
				handler := w.TxNotificationHandlers.pop()
				done := handler(notification)
				if !done {
					toRepeat = append(toRepeat, handler)
				}
			}
			for _, handler := range toRepeat {
				w.TxNotificationHandlers.pushBack(handler)
			}
		}
	}
}

func (w *wallet) createAccountIfNotExists(
	ctx context.Context, account string,
) error {
	exists, err := w.accountExists(ctx, account)
	if err != nil {
		return err
	}
	if !exists {
		if _, _, err := w.AccountManager().CreateAccount(ctx, account); err != nil {
			return err
		}
	}
	return nil
}

func (w *wallet) accountExists(
	ctx context.Context, account string,
) (bool, error) {
	info, err := w.WalletManager().GetInfo(ctx)
	if err != nil {
		return false, err
	}
	for _, a := range info.Accounts() {
		if a.Name() == account {
			return true, nil
		}
	}
	return false, nil
}

func toAddressAndBlindingKeyList(
	addresses []ports.AddressInfo,
) []AddressAndBlindingKey {
	list := make([]AddressAndBlindingKey, 0, len(addresses))
	for _, addr := range addresses {
		list = append(list, AddressAndBlindingKey{
			Address:     addr.Address(),
			BlindingKey: addr.BlindingPrivKey(),
		})
	}
	return list
}

func portableNetworkToString(net ports.Network) (string, error) {
	if net.IsUnknown() {
		return "", fmt.Errorf(
			"wallet configured with unknown network. " +
				"Must be either mainnet|testnet|regtest",
		)
	}
	if net.IsRegtest() {
		return "regtest", nil
	}
	if net.IsTestnet() {
		return "testnet", nil
	}
	return "mainnet", nil
}

type UtxoNotificationHandler func(ports.UtxoNotification) bool
type utxoEventType int

func parseUtxoEventType(portableType ports.UtxoEventType) (utxoEventType, bool) {
	if portableType.IsUtxoLocked() {
		return utxoEventLocked, true
	}
	if portableType.IsUtxoUnlocked() {
		return utxoEventUnlocked, true
	}
	if portableType.IsUtxoSpent() {
		return utxoEventSpent, true
	}
	return utxoEventUnknown, false
}

func (t utxoEventType) IsUnknown() bool {
	return t == utxoEventUnknown
}

func (t utxoEventType) IsUtxoLocked() bool {
	return t == utxoEventLocked
}

func (t utxoEventType) IsUtxoUnlocked() bool {
	return t == utxoEventUnlocked
}

func (t utxoEventType) IsUtxoSpent() bool {
	return t == utxoEventSpent
}

type utxoNotificationQueue struct {
	lock *sync.Mutex
	list []UtxoNotificationHandler
}

func (q utxoNotificationQueue) len() int {
	return len(q.list)
}

func (q utxoNotificationQueue) pushBack(handler UtxoNotificationHandler) {
	q.lock.Lock()
	defer q.lock.Unlock()

	list := append(q.list, handler)
	copy(q.list, list)
}

func (q utxoNotificationQueue) pop() UtxoNotificationHandler {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.list) <= 0 {
		return nil
	}
	handler := q.list[0]
	copy(q.list, q.list[1:])
	return handler
}

type TxNotificationHandler func(ports.TxNotification) bool
type txEventType int

func parseTxEventType(portableType ports.TxEventType) (txEventType, bool) {
	if portableType.IsTxBroadcasted() {
		return txEventBroadcasted, true
	}
	if portableType.IsTxUnconfirmed() {
		return txEventUnconfirmed, true
	}
	if portableType.IsTxConfirmed() {
		return txEventConfirmed, true
	}
	return txEventUnknown, false
}

func (t txEventType) IsUnknown() bool {
	return t == txEventUnknown
}

func (t txEventType) IsTxBroadcasted() bool {
	return t == txEventBroadcasted
}

func (t txEventType) IsTxUnconfirmed() bool {
	return t == txEventUnconfirmed
}

func (t txEventType) IsTxConfirmed() bool {
	return t == txEventConfirmed
}

type txNotificationQueue struct {
	lock *sync.Mutex
	list []TxNotificationHandler
}

func (q txNotificationQueue) len() int {
	return len(q.list)
}

func (q txNotificationQueue) pushBack(handler TxNotificationHandler) {
	q.lock.Lock()
	defer q.lock.Unlock()

	list := append(q.list, handler)
	copy(q.list, list)
}

func (q txNotificationQueue) pop() TxNotificationHandler {
	q.lock.Lock()
	defer q.lock.Unlock()

	if len(q.list) <= 0 {
		return nil
	}
	handler := q.list[0]
	copy(q.list, q.list[1:])
	return handler
}
