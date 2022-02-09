package wallet

import (
	"context"
	"errors"

	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	"github.com/tdex-network/tdex-daemon/internal/infrastructure/oceanv1alpha"
	"google.golang.org/grpc"
)

type accountManagerGrpc struct {
	client oceanv1alpha.AccountServiceClient
}

var _ ports.AccountManager = (*accountManagerGrpc)(nil)

func newAccountManagerGrpc(conn *grpc.ClientConn) ports.AccountManager {
	return &accountManagerGrpc{
		client: oceanv1alpha.NewAccountServiceClient(conn),
	}
}

func (am *accountManagerGrpc) CreateAccount(ctx context.Context, name string) (uint64, string, error) {
	req := &oceanv1alpha.CreateAccountRequest{
		Name: name,
	}

	resp, err := am.client.CreateAccount(ctx, req)
	if err != nil {
		return 0, "", err
	}

	return resp.AccountIndex, resp.Xpub, nil
}

func (am *accountManagerGrpc) DeriveAddressesForAccount(ctx context.Context, account string, num uint64) ([]string, error) {
	req := &oceanv1alpha.DeriveAddressRequest{
		AccountKey:     &oceanv1alpha.AccountKey{Name: account},
		NumOfAddresses: num,
	}

	resp, err := am.client.DeriveAddress(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Addresses, nil
}

func (am *accountManagerGrpc) DeriveChangeAddressesForAccount(ctx context.Context, account string, num uint64) ([]string, error) {
	req := &oceanv1alpha.DeriveChangeAddressRequest{
		AccountKey:     &oceanv1alpha.AccountKey{Name: account},
		NumOfAddresses: num,
	}

	resp, err := am.client.DeriveChangeAddress(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Addresses, nil
}

func (am *accountManagerGrpc) ListAddressesForAccount(ctx context.Context, account string) ([]string, error) {
	req := &oceanv1alpha.ListAddressesRequest{
		AccountKey: &oceanv1alpha.AccountKey{Name: account},
	}

	resp, err := am.client.ListAddresses(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Addresses, nil
}

func (am *accountManagerGrpc) BalanceForAccount(
	ctx context.Context, account string,
) (map[string]ports.Balance, error) {
	req := &oceanv1alpha.BalanceRequest{
		AccountKey: &oceanv1alpha.AccountKey{Name: account},
	}

	resp, err := am.client.Balance(ctx, req)
	if err != nil {
		return nil, err
	}

	balances := make(map[string]ports.Balance)
	for asset, b := range resp.Balance {
		balances[asset] = &balanceGrpc{b}
	}

	return balances, nil
}

func (am *accountManagerGrpc) DeleteAccount(ctx context.Context, account string) error {
	return errors.New("not implemented")
}

func (am *accountManagerGrpc) ListUtxosForAccount(ctx context.Context, account string) ([]ports.Utxo, []ports.Utxo, error) {
	req := &oceanv1alpha.ListUtxosRequest{
		AccountKey: &oceanv1alpha.AccountKey{Name: account},
	}

	resp, err := am.client.ListUtxos(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	spendableUtxos := make([]ports.Utxo, 0, len(resp.SpendableUtxos))
	lockedUtxos := make([]ports.Utxo, 0, len(resp.LockedUtxos))

	for _, utxos := range resp.SpendableUtxos {
		for _, utxo := range utxos.Utxos {
			spendableUtxos = append(spendableUtxos, &utxoGrpc{utxo})
		}
	}

	for _, utxos := range resp.LockedUtxos {
		for _, utxo := range utxos.Utxos {
			lockedUtxos = append(lockedUtxos, &utxoGrpc{utxo})
		}
	}

	return spendableUtxos, lockedUtxos, nil
}
