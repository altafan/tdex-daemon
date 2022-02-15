package ports

import "context"

type OceanWallet interface {
	WalletManager() WalletManager
	AccountManager() AccountManager
	TransactionManager() TransactionManager
	NotificationManager() NotificationManager
}

type WalletManager interface {
	GenSeed(ctx context.Context) (mnemonic []string, err error)
	CreateWallet(
		ctx context.Context, mnemonic []string, passphrase string,
		chMessages chan string,
	) error
	RestoreWallet(
		ctx context.Context, mnemonic []string, passphrase string,
		chMessages chan string,
	) error
	Unlock(ctx context.Context, passphrase string) error
	ChangePassword(
		ctx context.Context, currentPassphrase, newPassphrase string,
	) error
	Status(ctx context.Context) (status WalletStatus, err error)
	GetInfo(
		ctx context.Context,
	) (info WalletInfo, err error)
}

type AccountManager interface {
	CreateAccount(
		ctx context.Context, name string,
	) (accountIndex uint64, xpub string, err error)
	DeriveAddressesForAccount(
		ctx context.Context, account string, numOfAddresses uint64,
	) ([]string, error)
	DeriveChangeAddressesForAccount(
		ctx context.Context, account string, numOfAddresses uint64,
	) ([]string, error)
	ListAddressesForAccount(
		ctx context.Context, account string,
	) ([]string, error)
	BalanceForAccount(
		ctx context.Context, account string,
	) (balancePerAsset map[string]Balance, err error)
	DeleteAccount(ctx context.Context, account string) error
	ListUtxosForAccount(
		ctx context.Context, account string,
	) (spendableUtxos, lockedUtxos []Utxo, err error)
}

type TransactionManager interface {
	SelectUnspentsForAccount(
		ctx context.Context, account string,
		targetAsset string, targetAmount uint64, strategy Strategy,
	) (utxos []UtxoKey, change uint64, err error)
	EstimateFees(
		ctx context.Context, inputs []Input, outputs []Output,
	) (feeAmount uint64, err error)
	TransferFromAccount(
		ctx context.Context, account string, outputs []Output,
		millisatPerByte uint64,
	) (txHex string, err error)
	CreateTransaction(
		ctx context.Context, inputs []Input, outputs []Output,
	) (psetBase64 string, err error)
	UpdateTransaction(
		ctx context.Context, psetBase64 string, inputs []Input, outputs []Output,
	) (
		updatedPset string,
		inBlindKeysByScript, outBlindKeysByScrpt map[string][]byte,
		err error,
	)
	BlindTransaction(
		ctx context.Context, psetBase64 string, lastBlinder bool,
	) (blindedPsetBase64 string, err error)
	SignTransaction(
		ctx context.Context, psetBase64 string, extractRawTx bool,
	) (signedPsetBase64 string, err error)
	BroadcastTransaction(
		ctx context.Context, txHex string,
	) (txid string, err error)
}

type NotificationManager interface {
	TxChannel() (chan TxNotification, error)
	UtxoChannel() (chan UtxoNotification, error)
}

type WalletInfo interface {
	Network() Network
	NativeAsset() string
	RootPath() string
	MasterBlindingKey() string
	Accounts() []WalletAccount
}

type WalletAccount interface {
	Index() uint64
	Name() string
	DerivationPath() string
	Xpub() string
}

type Network interface {
	IsUnknown() bool
	IsMainnet() bool
	IsTestnet() bool
	IsRegtest() bool
}

type WalletStatus interface {
	IsInitialized() bool
	IsSynced() bool
	IsUnlocked() bool
}

type InitWalletMsg interface {
	Error() error
	Message() string
}

type Balance interface {
	Unconfirmed() uint64
	Confirmed() uint64
	Total() uint64
}

type TxNotification interface {
	TxID() string
	EventType() TxEventType
	BlockDetails() BlockDetails
}

type UtxoNotification interface {
	Utxo() UtxoKey
	EventType() UtxoEventType
}

type TxEventType interface {
	IsUnknown() bool
	IsTxBroadcasted() bool
	IsTxUnconfirmed() bool
	IsTxConfirmed() bool
}

type UtxoEventType interface {
	IsUnknown() bool
	IsUtxoLocked() bool
	IsUtxoUnlocked() bool
	IsUtxoSpent() bool
}

type BlockDetails interface {
	Hash() string
	Height() uint32
	Timestamp() int64
}

type TxDetails interface {
	Hash() string
	Hex() string
}

type Strategy interface {
	IsUnknown() bool
	IsBranchBound() bool
	IsFragment() bool
}

type UtxoKey interface {
	TxID() string
	Index() uint32
}

type Utxo interface {
	Key() UtxoKey
	Asset() string
	Value() uint64
	Script() []byte
	IsConfirmed() bool
	IsLocked() bool
}

type Input = UtxoKey
type Output interface {
	Asset() string
	Value() uint64
	Address() string
}
