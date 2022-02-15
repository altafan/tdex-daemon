package oceanwallet

import (
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type oceanGrpcWallet struct {
	w ports.WalletManager
	t ports.TransactionManager
	n ports.NotificationManager
	a ports.AccountManager
}

// NewOceanGrpcWallet creates a new OceanWallet instance.
// it uses an ocean grpc client to communicate with the ocean server.
func New(addr string) (ports.OceanWallet, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
