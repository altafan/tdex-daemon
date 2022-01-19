package ports

import "context"

type Wallet interface {
	WalletManager() WalletManager
	AccountManager() AccountManager
	TransactionManager() TransactionManager
	NotificationManager() NotificationManager
}

type WalletManager interface {
	GenSeed(ctx context.Context) (mnemonic string, err error)
	CreateWallet(ctx context.Context, mnemonic, password string) error
	RestoreWallet(ctx context.Context, mnemonic, password string) error
	Unlock(ctx context.Context, password string) error
	ChangePassword(ctx context.Context, oldPassword, newPassword string) error
	Status(ctx context.Context) (code uint64, desc string, err error)
	GetInfo(
		ctx context.Context,
	) (rootPath, masterBlindingKey string, accounts []WalletAccount, err error)
}

type AccountManager interface {
	CreateAccount(
		ctx context.Context, name string,
	) (index uint64, xpub string, err error)
	DeriveAddressesForAccount(
		ctx context.Context, accountIndex, numOfAddresses uint64,
	) ([]AddressInfo, error)
	ListAddressesForAccount(
		ctx context.Context, accountIndex uint64,
	) ([]AddressInfo, error)
	BalanceForAccount(
		ctx context.Context, accountIndex uint64,
	) (balancePerAsset map[string]Balance, err error)
	DeleteAccount(ctx context.Context, accountIndex uint64) error
}

type TransactionManager interface {
	SelectUnspentsForAccount(
		ctx context.Context, accountIndex uint64,
		targetAsset string, targetAmount uint64, strategy Strategy,
	)
	TransferFromAccount(
		ctx context.Context, accountIndex uint64, receivers []Receiver,
	) (txHex string, err error)
	SignTransaction(
		ctx context.Context, psetBase64 string,
	) (signedPsetBase64 string, err error)
	BroadcastTransaction(txHex string) (txid string, err error)
}

type NotificationManager interface {
	TxChannel() chan TxNotification
	UtxoChannel() chan UtxoNotification
}

type WalletAccount interface {
	Index() uint64
	LastInternalIndex() uint64
	LastExternalIndex() uint64
	DerivationPath() string
	Xpub() string
}

type AddressInfo interface {
	Address() string
	DerivationPath() string
	OutputScript() string
	BlindingPrivKey() string
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
	AccountIndex() uint64
	Utxo() Utxo
}

type TxEventType interface {
	IsUknown() bool
	IsTxBroadcasted() bool
	IsTxUnconfirmed() bool
	IsTxConfirmed() bool
}

type BlockDetails interface {
	Hash() string
	Height() uint32
	Timestamp() int64
}

type Strategy interface {
	IsUknown() bool
	IsBranchBound() bool
	IsFragment() bool
}

type Utxo interface {
	TxID() string
	Index() uint32
	Asset() string
	Value() uint64
	AssetCommitment() string
	ValueCommitment() string
	AssetBlinder() []byte
	ValueBlinder() []byte
	Script() []byte
	Nonce() []byte
	RangeProof() []byte
	SurjectionProof() []byte
	IsConfidential() bool
	IsConfirmed() bool
	IsRevealed() bool
	IsNew() bool
	IsSpent() bool
}

type Receiver interface {
	Address() string
	Asset() string
	Amount() uint64
}
