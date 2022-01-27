package application

import (
	"context"
	"encoding/hex"

	"github.com/tdex-network/tdex-daemon/internal/core/ports"
)

type WalletService interface {
	GenerateAddressAndBlindingKey(
		ctx context.Context,
	) (addressAndBlindingKey *AddressAndBlindingKey, err error)
	GetBalance(
		ctx context.Context,
	) (balancePerAsset map[string]ports.Balance, err error)
	SendToMany(
		ctx context.Context, outputs Outputs, millisatPerByte uint64,
	) (txHex []byte, txid []byte, err error)
}

type walletService struct {
	repoManager ports.RepoManager
	wallet      Wallet
	marketFee   int64

	pwChan chan PassphraseMsg
}

func NewWalletService(
	repoManager ports.RepoManager, wallet Wallet, marketFee int64,
) WalletService {
	return newWalletService(
		repoManager, wallet, marketFee,
	)
}

func newWalletService(
	repoManager ports.RepoManager, wallet Wallet, marketFee int64,
) *walletService {
	return &walletService{
		repoManager: repoManager,
		wallet:      wallet,
		marketFee:   marketFee,
		pwChan:      make(chan PassphraseMsg, 1),
	}
}

func (w *walletService) GenerateAddressAndBlindingKey(
	ctx context.Context,
) (*AddressAndBlindingKey, error) {
	addresses, err := w.wallet.DeriveAddressForAccount(ctx, PersonalAccount, 1)
	if err != nil {
		return nil, err
	}
	addr := addresses[0]
	return &addr, nil
}

func (w *walletService) GetBalance(
	ctx context.Context,
) (map[string]ports.Balance, error) {
	return w.wallet.AccountManager().BalanceForAccount(ctx, PersonalAccount)
}

type SendToManyRequest struct {
	Outputs         Outputs
	MillisatPerByte int64
}

func (w *walletService) SendToMany(
	ctx context.Context, outs Outputs, millisatPerByte uint64,
) ([]byte, []byte, error) {
	txHex, txid, err := w.wallet.SendToManyWithFeeTopup(
		ctx, PersonalAccount, outs, millisatPerByte,
	)
	if err != nil {
		return nil, nil, err
	}

	rawTx, _ := hex.DecodeString(txHex)
	rawTxid, _ := hex.DecodeString(txid)
	return rawTx, rawTxid, nil
}
