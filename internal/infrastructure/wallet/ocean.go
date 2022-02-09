package wallet

import (
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	"google.golang.org/grpc"
)

type oceanGrpcWallet struct {
	w ports.WalletManager
	t ports.TransactionManager
	n ports.NotificationManager
	a ports.AccountManager
}

var _ ports.OceanWallet = (*oceanGrpcWallet)(nil)

func NewOceanWallet(addr string) (ports.OceanWallet, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return &oceanGrpcWallet{
		w: newWalletManagerGrpc(conn),
		t: newTransactionManagerGrpc(conn),
		n: newNotificationsManagerGrpc(conn),
		a: newAccountManagerGrpc(conn),
	}, nil
}

func (o *oceanGrpcWallet) WalletManager() ports.WalletManager {
	return o.w
}

func (o *oceanGrpcWallet) TransactionManager() ports.TransactionManager {
	return o.t
}

func (o *oceanGrpcWallet) NotificationManager() ports.NotificationManager {
	return o.n
}

func (o *oceanGrpcWallet) AccountManager() ports.AccountManager {
	return o.a
}
