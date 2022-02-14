package wallet

import (
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
)

type networkGrpc struct {
	n oceanv1alpha.GetInfoResponse_Network
}

var _ ports.Network = (*networkGrpc)(nil)

func (net *networkGrpc) IsMainnet() bool {
	return net.n == oceanv1alpha.GetInfoResponse_NETWORK_MAINNET
}

func (net *networkGrpc) IsTestnet() bool {
	return net.n == oceanv1alpha.GetInfoResponse_NETWORK_TESTNET
}

func (net *networkGrpc) IsRegtest() bool {
	return net.n == oceanv1alpha.GetInfoResponse_NETWORK_REGTEST
}

func (net *networkGrpc) IsUnknown() bool {
	return !net.IsMainnet() && !net.IsTestnet() && !net.IsRegtest()
}

type walletInfosGrpc struct {
	resp *oceanv1alpha.GetInfoResponse
}

var _ ports.WalletInfo = (*walletInfosGrpc)(nil)

func (w *walletInfosGrpc) Network() ports.Network {
	return &networkGrpc{w.resp.Network}
}

func (w *walletInfosGrpc) NativeAsset() string {
	return w.resp.NativeAsset
}

func (w *walletInfosGrpc) RootPath() string {
	return w.resp.RootPath
}

func (w *walletInfosGrpc) MasterBlindingKey() string {
	return w.resp.MasterBlindingKey
}

func (w *walletInfosGrpc) Accounts() []ports.WalletAccount {
	var accounts []ports.WalletAccount
	for _, a := range w.resp.Accounts {
		accounts = append(accounts, &accountGrpc{accountInfo: a})
	}
	return accounts
}
