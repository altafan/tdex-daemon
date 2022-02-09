package application_test

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/tdex-network/tdex-daemon/internal/core/application"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
)

// **** Wallet Managers ****

type walletStatus struct {
	initialized bool
	synced      bool
	unlocked    bool
}

func (s walletStatus) IsInitialized() bool {
	return s.initialized
}
func (s walletStatus) IsSynced() bool {
	return s.synced
}
func (s walletStatus) IsUnlocked() bool {
	return s.unlocked
}

type mockedWalletManager struct {
	mock.Mock
}

func (m *mockedWalletManager) GenSeed(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	var res []string
	if a := args.Get(0); a != nil {
		res = a.([]string)
	}
	return res, args.Error(1)
}

func (m *mockedWalletManager) CreateWallet(
	ctx context.Context, mnemonic []string, passphrase string,
	chMessages chan string,
) error {
	defer close(chMessages)
	args := m.Called(ctx, mnemonic, passphrase, chMessages)
	err := args.Error(0)
	if err == nil {
		chMessages <- "creating wallet"
		time.Sleep(time.Second)
		chMessages <- "wallet creation succeeded"
	}

	return err
}

func (m *mockedWalletManager) RestoreWallet(
	ctx context.Context, mnemonic []string, passphrase string,
	chMessages chan string,
) error {
	defer close(chMessages)
	args := m.Called(ctx, mnemonic, passphrase, chMessages)
	err := args.Error(0)
	if err == nil {
		chMessages <- "restoring wallet"
		time.Sleep(200 * time.Millisecond)
		chMessages <- "restoring accounts"
		time.Sleep(200 * time.Millisecond)
		chMessages <- "restored 2 accounts"
		time.Sleep(200 * time.Millisecond)
		chMessages <- "restoring utxo set for discovered accounts"
		time.Sleep(600 * time.Millisecond)
		chMessages <- "restored utxo set for discovered accounts"
		time.Sleep(600 * time.Millisecond)
		chMessages <- "wallet restoration succeeded"
	}
	return err
}

func (m *mockedWalletManager) Unlock(ctx context.Context, passphrase string) error {
	args := m.Called(ctx, passphrase)
	return args.Error(0)
}

func (m *mockedWalletManager) ChangePassword(
	ctx context.Context, currentPassphrase, newPassphrase string,
) error {
	args := m.Called(ctx, currentPassphrase, newPassphrase)
	return args.Error(0)
}

func (m *mockedWalletManager) Status(
	ctx context.Context,
) (ports.WalletStatus, error) {
	args := m.Called(ctx)
	var res ports.WalletStatus
	if a := args.Get(0); a != nil {
		res = a.(ports.WalletStatus)
	}
	return res, args.Error(1)
}
func (m *mockedWalletManager) GetInfo(
	ctx context.Context,
) (ports.WalletInfo, error) {
	args := m.Called(ctx)
	var res ports.WalletInfo
	if a := args.Get(0); a != nil {
		res = a.(ports.WalletInfo)
	}
	return res, args.Error(1)
}

type accountBalance struct {
	total       uint64
	confirmed   uint64
	unconfirmed uint64
}

func (b accountBalance) Total() uint64 {
	return b.total
}
func (b accountBalance) Confirmed() uint64 {
	return b.confirmed
}
func (b accountBalance) Unconfirmed() uint64 {
	return b.unconfirmed
}

type mockedAccountManager struct {
	mock.Mock
}

func (m *mockedAccountManager) CreateAccount(
	ctx context.Context, name string,
) (uint64, string, error) {
	args := m.Called(ctx, name)
	var res uint64
	var res1 string
	if a := args.Get(0); a != nil {
		res = a.(uint64)
	}
	if a := args.Get(1); a != nil {
		res1 = a.(string)
	}
	return res, res1, args.Error(2)
}

func (m *mockedAccountManager) DeriveAddressesForAccount(
	ctx context.Context, account string, numOfAddresses uint64,
) ([]ports.AddressInfo, error) {
	args := m.Called(ctx, account, numOfAddresses)
	var res []ports.AddressInfo
	if a := args.Get(0); a != nil {
		res = a.([]ports.AddressInfo)
	}
	return res, args.Error(1)
}

func (m *mockedAccountManager) DeriveChangeAddressesForAccount(
	ctx context.Context, account string, numOfAddresses uint64,
) ([]ports.AddressInfo, error) {
	args := m.Called(ctx, account, numOfAddresses)
	var res []ports.AddressInfo
	if a := args.Get(0); a != nil {
		res = a.([]ports.AddressInfo)
	}
	return res, args.Error(1)
}

func (m *mockedAccountManager) ListAddressesForAccount(
	ctx context.Context, account string,
) ([]ports.AddressInfo, error) {
	args := m.Called(ctx, account)
	var res []ports.AddressInfo
	if a := args.Get(0); a != nil {
		res = a.([]ports.AddressInfo)
	}
	return res, args.Error(1)
}

func (m *mockedAccountManager) BalanceForAccount(
	ctx context.Context, account string,
) (map[string]ports.Balance, error) {
	args := m.Called(ctx, account)
	var res map[string]ports.Balance
	if a := args.Get(0); a != nil {
		res = a.(map[string]ports.Balance)
	}
	return res, args.Error(1)
}

func (m *mockedAccountManager) DeleteAccount(
	ctx context.Context, account string,
) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

func (m *mockedAccountManager) ListUtxosForAccount(
	ctx context.Context, account string,
) ([]ports.Utxo, []ports.Utxo, error) {
	args := m.Called(ctx, account)
	var res, res1 []ports.Utxo
	if a := args.Get(0); a != nil {
		res = a.([]ports.Utxo)
	}
	if a := args.Get(1); a != nil {
		res1 = a.([]ports.Utxo)

	}
	return res, res1, args.Error(2)
}

type mockedTransactionManager struct {
	mock.Mock
}

func (m *mockedTransactionManager) SelectUnspentsForAccount(
	ctx context.Context, account string,
	targetAsset string, targetAmount uint64, strategy ports.Strategy,
) ([]ports.UtxoKey, uint64, error) {
	args := m.Called(ctx, account, targetAsset, targetAmount, strategy)
	var res []ports.UtxoKey
	var res1 uint64
	if a := args.Get(0); a != nil {
		res = a.([]ports.UtxoKey)
	}
	if a := args.Get(1); a != nil {
		res1 = a.(uint64)

	}
	return res, res1, args.Error(2)
}

func (m *mockedTransactionManager) EstimateFees(
	ctx context.Context, inputs []ports.Input, outputs []ports.Output,
) (uint64, error) {
	args := m.Called(ctx, inputs, outputs)
	var res uint64
	if a := args.Get(0); a != nil {
		res = a.(uint64)
	}
	return res, args.Error(1)
}

func (m *mockedTransactionManager) TransferFromAccount(
	ctx context.Context, account string, outputs []ports.Output,
	millisatPerByte uint64,
) (string, error) {
	args := m.Called(ctx, account, outputs, millisatPerByte)
	var res string
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	return res, args.Error(1)
}

func (m *mockedTransactionManager) CreateTransaction(
	ctx context.Context, inputs []ports.Input, outputs []ports.Output,
) (string, error) {
	args := m.Called(ctx, inputs, outputs)
	var res string
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	return res, args.Error(1)
}

func (m *mockedTransactionManager) UpdateTransaction(
	ctx context.Context, psetBase64 string, inputs []ports.Input, outputs []ports.Output,
) (string, map[string][]byte, map[string][]byte, error) {
	args := m.Called(ctx, psetBase64, inputs, outputs)
	var res string
	var res1, res2 map[string][]byte
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	if a := args.Get(1); a != nil {
		res1 = a.(map[string][]byte)
	}
	if a := args.Get(2); a != nil {
		res2 = a.(map[string][]byte)
	}
	return res, res1, res2, args.Error(3)
}

func (m *mockedTransactionManager) BlindTransaction(
	ctx context.Context, psetBase64 string, lastBlinder bool,
) (string, error) {
	args := m.Called(ctx, psetBase64, lastBlinder)
	var res string
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	return res, args.Error(1)
}

func (m *mockedTransactionManager) SignTransaction(
	ctx context.Context, psetBase64 string, extractRawTx bool,
) (string, error) {
	args := m.Called(ctx, psetBase64, extractRawTx)
	var res string
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	return res, args.Error(1)
}

func (m *mockedTransactionManager) BroadcastTransaction(
	ctx context.Context, txHex string,
) (string, error) {
	args := m.Called(ctx, txHex)
	var res string
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	return res, args.Error(1)
}

type mockedNotificationManager struct {
	mock.Mock
}

func (m *mockedNotificationManager) TxChannel() chan ports.TxNotification {
	args := m.Called()
	var res chan ports.TxNotification
	if a := args.Get(0); a != nil {
		res = a.(chan ports.TxNotification)
	}
	return res
}

func (m *mockedNotificationManager) UtxoChannel() chan ports.UtxoNotification {
	args := m.Called()
	var res chan ports.UtxoNotification
	if a := args.Get(0); a != nil {
		res = a.(chan ports.UtxoNotification)
	}
	return res
}

// **** Wallet ****
type mockedWallet struct {
	mock.Mock
	walletManager       *mockedWalletManager
	accountManager      *mockedAccountManager
	txManager           *mockedTransactionManager
	notificationManager *mockedNotificationManager
}

func newMockedWallet() application.Wallet {
	walletManager := &mockedWalletManager{}
	accountManager := &mockedAccountManager{}
	txManager := &mockedTransactionManager{}
	notificationManager := &mockedNotificationManager{}
	return &mockedWallet{
		walletManager:       walletManager,
		accountManager:      accountManager,
		txManager:           txManager,
		notificationManager: notificationManager,
	}
}

func (m *mockedWallet) WalletManager() ports.WalletManager {
	return m.walletManager
}
func (m *mockedWallet) AccountManager() ports.AccountManager {
	return m.accountManager
}
func (m *mockedWallet) TransactionManager() ports.TransactionManager {
	return m.txManager
}
func (m *mockedWallet) NotificationManager() ports.NotificationManager {
	return m.notificationManager
}

func (m *mockedWallet) Network() string {
	args := m.Called()
	var res string
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	return res
}

func (m *mockedWallet) NativeAsset() string {
	args := m.Called()
	var res string
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	return res
}

func (m *mockedWallet) DeriveAddressForAccount(
	ctx context.Context, account string, numOfAddresses uint64,
) ([]application.AddressAndBlindingKey, error) {
	args := m.Called(ctx, account, numOfAddresses)
	var res []application.AddressAndBlindingKey
	if a := args.Get(0); a != nil {
		res = a.([]application.AddressAndBlindingKey)
	}
	return res, args.Error(1)
}

func (m *mockedWallet) ListAddressesForAccount(
	ctx context.Context, account string,
) ([]application.AddressAndBlindingKey, error) {
	args := m.Called(ctx, account)
	var res []application.AddressAndBlindingKey
	if a := args.Get(0); a != nil {
		res = a.([]application.AddressAndBlindingKey)
	}
	return res, args.Error(1)
}

func (m *mockedWallet) BalanceForAccount(
	ctx context.Context, account string,
) (map[string]ports.Balance, error) {
	args := m.Called(ctx, account)
	var res map[string]ports.Balance
	if a := args.Get(0); a != nil {
		res = a.(map[string]ports.Balance)
	}
	return res, args.Error(1)
}

func (m *mockedWallet) SendToManyWithFeeTopup(
	ctx context.Context,
	account string, outs application.Outputs, millisatPerByte uint64,
) (string, string, error) {
	args := m.Called(ctx, account, outs, millisatPerByte)
	var res, res1 string
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	if a := args.Get(1); a != nil {
		res1 = a.(string)
	}
	return res, res1, args.Error(2)
}

func (m *mockedWallet) FillSwapTransaction(
	ctx context.Context, account string, swapRequest domain.SwapRequest,
) (string, []ports.UtxoKey, map[string][]byte, map[string][]byte, error) {
	args := m.Called(ctx, account, swapRequest)
	var res string
	var res1 []ports.UtxoKey
	var res2, res3 map[string][]byte
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	if a := args.Get(1); a != nil {
		res1 = a.([]ports.UtxoKey)
	}
	if a := args.Get(2); a != nil {
		res2 = a.(map[string][]byte)
	}
	if a := args.Get(3); a != nil {
		res3 = a.(map[string][]byte)
	}
	return res, res1, res2, res3, args.Error(4)
}

func (m *mockedWallet) RegisterHandlerForUtxoEvent(_ application.UtxoNotificationHandler) {}
func (m *mockedWallet) RegisterHandlerForTxEvent(_ application.TxNotificationHandler)     {}

// **** PsetParser ****

type mockPsetParser struct {
	mock.Mock
}

func (m *mockPsetParser) GetTxID(psetBase64 string) (string, error) {
	args := m.Called(psetBase64)
	var res string
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	return res, args.Error(1)
}

func (m *mockPsetParser) GetTxHex(psetBase64 string) (string, error) {
	args := m.Called(psetBase64)
	var res string
	if a := args.Get(0); a != nil {
		res = a.(string)
	}
	return res, args.Error(1)
}

// **** SwapParser ****

type mockSwapParser struct {
	mock.Mock
}

func (m *mockSwapParser) SerializeRequest(req domain.SwapRequest) ([]byte, *domain.SwapError) {
	args := m.Called(req)

	var res []byte
	if a := args.Get(0); a != nil {
		res = a.([]byte)
	}

	var err *domain.SwapError
	if a := args.Get(1); a != nil {
		err = a.(*domain.SwapError)
	}
	return res, err
}

func (m *mockSwapParser) SerializeAccept(acc domain.AcceptArgs) (string, []byte, *domain.SwapError) {
	args := m.Called(acc)

	var sres string
	if a := args.Get(0); a != nil {
		sres = a.(string)
	}

	var bres []byte
	if a := args.Get(1); a != nil {
		bres = a.([]byte)
	}

	var err *domain.SwapError
	if args.Get(2) != nil {
		err = args.Get(2).(*domain.SwapError)
	}

	return sres, bres, err
}

func (m *mockSwapParser) SerializeComplete(accMsg []byte, tx string) (string, []byte, *domain.SwapError) {
	args := m.Called(accMsg, tx)

	var sres string
	if a := args.Get(0); a != nil {
		sres = a.(string)
	}

	var bres []byte
	if a := args.Get(1); a != nil {
		bres = a.([]byte)
	}

	var err *domain.SwapError
	if args.Get(2) != nil {
		err = args.Get(2).(*domain.SwapError)
	}

	return sres, bres, err
}

func (m *mockSwapParser) SerializeFail(id string, errCode int, errMsg string) (string, []byte) {
	args := m.Called(id, errCode, errMsg)

	var sres string
	if a := args.Get(0); a != nil {
		sres = a.(string)
	}

	var bres []byte
	if a := args.Get(1); a != nil {
		bres = a.([]byte)
	}

	return sres, bres
}

func (m *mockSwapParser) DeserializeRequest(msg []byte) (domain.SwapRequest, error) {
	args := m.Called(msg)
	var res domain.SwapRequest
	if a := args.Get(0); a != nil {
		res = a.(domain.SwapRequest)
	}

	return res, args.Error(1)
}

func (m *mockSwapParser) DeserializeAccept(msg []byte) (domain.SwapAccept, error) {
	args := m.Called(msg)
	var res domain.SwapAccept
	if a := args.Get(0); a != nil {
		res = a.(domain.SwapAccept)
	}
	return res, args.Error(1)
}

func (m *mockSwapParser) DeserializeComplete(msg []byte) (domain.SwapComplete, error) {
	args := m.Called(msg)
	var res domain.SwapComplete
	if a := args.Get(0); a != nil {
		res = a.(domain.SwapComplete)
	}
	return res, args.Error(1)
}

func (m *mockSwapParser) DeserializeFail(msg []byte) (domain.SwapFail, error) {
	args := m.Called(msg)
	var res domain.SwapFail
	if a := args.Get(0); a != nil {
		res = a.(domain.SwapFail)
	}
	return res, args.Error(1)
}

// **** SwapRequest ****

type mockSwapRequest struct {
	id string
}

func newMockedSwapRequest() *mockSwapRequest {
	return &mockSwapRequest{randomId()}
}

func (m *mockSwapRequest) GetId() string {
	return m.id
}

func (m *mockSwapRequest) GetAssetP() string {
	return randomHex(32)
}

func (m *mockSwapRequest) GetAmountP() uint64 {
	return randomValue()
}

func (m *mockSwapRequest) GetAssetR() string {
	return randomHex(32)
}

func (m *mockSwapRequest) GetAmountR() uint64 {
	return randomValue()
}

func (m *mockSwapRequest) GetTransaction() string {
	return randomBase64()
}

func (m *mockSwapRequest) GetInputBlindingKey() map[string][]byte {
	return nil
}

func (m *mockSwapRequest) GetOutputBlindingKey() map[string][]byte {
	return nil
}

// **** SwapAccept ****

type mockSwapAccept struct {
	id string
}

func newMockedSwapAccept() *mockSwapAccept {
	return &mockSwapAccept{randomId()}
}

func (m *mockSwapAccept) GetId() string {
	return m.id
}

func (m *mockSwapAccept) GetRequestId() string {
	return randomId()
}

func (m *mockSwapAccept) GetTransaction() string {
	return randomBase64()
}

func (m *mockSwapAccept) GetInputBlindingKey() map[string][]byte {
	return nil
}

func (m *mockSwapAccept) GetOutputBlindingKey() map[string][]byte {
	return nil
}

// **** SwapComplete ****

type mockSwapComplete struct {
	id string
}

func newMockedSwapComplete() *mockSwapComplete {
	return &mockSwapComplete{randomId()}
}

func (m *mockSwapComplete) GetId() string {
	return m.id
}

func (m *mockSwapComplete) GetAcceptId() string {
	return randomId()
}

func (m *mockSwapComplete) GetTransaction() string {
	return randomBase64()
}

// **** SwapFail ****

type mockSwapFail struct {
	id string
}

func newMockedSwapFail() *mockSwapFail {
	return &mockSwapFail{randomId()}
}

func (m *mockSwapFail) GetId() string {
	return randomId()
}

func (m *mockSwapFail) GetMessageId() string {
	return randomId()
}

func (m *mockSwapFail) GetFailureCode() uint32 {
	return 1
}

func (m *mockSwapFail) GetFailureMessage() string {
	return "mocked error"
}
