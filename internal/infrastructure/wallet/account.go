package wallet

import (
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
)

type accountGrpc struct {
	accountInfo *oceanv1alpha.AccountInfo
}

var _ ports.WalletAccount = (*accountGrpc)(nil)

func (a *accountGrpc) Index() uint64 {
	return a.accountInfo.AccountKey.Id
}

func (a *accountGrpc) Name() string {
	return a.accountInfo.AccountKey.Name
}

func (a *accountGrpc) DerivationPath() string {
	return a.accountInfo.DerivationPath
}

func (a *accountGrpc) Xpub() string {
	return a.accountInfo.Xpub
}
