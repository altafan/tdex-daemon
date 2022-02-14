package wallet

import (
	"context"
	"strings"

	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
	"google.golang.org/grpc"
)

type walletManagerGrpc struct {
	client oceanv1alpha.WalletServiceClient
}

var _ ports.WalletManager = (*walletManagerGrpc)(nil)

func newWalletManagerGrpc(conn *grpc.ClientConn) ports.WalletManager {
	return &walletManagerGrpc{
		client: oceanv1alpha.NewWalletServiceClient(conn),
	}
}

func (wm *walletManagerGrpc) GenSeed(ctx context.Context) (mnemonic []string, err error) {
	req := &oceanv1alpha.GenSeedRequest{}
	resp, err := wm.client.GenSeed(ctx, req)
	if err != nil {
		return nil, err
	}

	return strings.Split(resp.Mnemonic, " "), nil
}

func (wm *walletManagerGrpc) CreateWallet(ctx context.Context, mnemonic []string, passphrase string, chMessages chan string) (err error) {
	req := &oceanv1alpha.CreateWalletRequest{
		Mnemonic: strings.Join(mnemonic, " "),
		Password: passphrase,
	}

	_, err = wm.client.CreateWallet(ctx, req)
	return err
}

func (wm *walletManagerGrpc) RestoreWallet(ctx context.Context, mnemonic []string, passphrase string, chMessages chan string) error {
	req := &oceanv1alpha.RestoreWalletRequest{
		Mnemonic: strings.Join(mnemonic, " "),
		Password: passphrase,
	}

	_, err := wm.client.RestoreWallet(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (wm *walletManagerGrpc) Unlock(ctx context.Context, passphrase string) error {
	req := &oceanv1alpha.UnlockRequest{
		Password: passphrase,
	}

	_, err := wm.client.Unlock(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (wm *walletManagerGrpc) ChangePassword(ctx context.Context, oldPassphrase string, newPassphrase string) error {
	req := &oceanv1alpha.ChangePasswordRequest{
		CurrentPassword: oldPassphrase,
		NewPassword:     newPassphrase,
	}

	_, err := wm.client.ChangePassword(ctx, req)
	if err != nil {
		return err
	}

	return nil
}

func (wm *walletManagerGrpc) Status(ctx context.Context) (ports.WalletStatus, error) {
	req := &oceanv1alpha.StatusRequest{}
	resp, err := wm.client.Status(ctx, req)
	if err != nil {
		return nil, err
	}

	return &walletStatusGrpc{resp}, nil
}

func (wm *walletManagerGrpc) GetInfo(ctx context.Context) (ports.WalletInfo, error) {
	req := &oceanv1alpha.GetInfoRequest{}
	resp, err := wm.client.GetInfo(ctx, req)
	if err != nil {
		return nil, err
	}

	return &walletInfosGrpc{resp}, nil
}
