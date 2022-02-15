package oceanwallet

import (
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
)

type balanceGrpc struct {
	infos *oceanv1alpha.BalanceInfo
}

var _ ports.Balance = (*balanceGrpc)(nil)

func (b *balanceGrpc) Total() uint64 {
	return b.infos.GetTotalBalance()
}

func (b *balanceGrpc) Unconfirmed() uint64 {
	return b.infos.GetConfirmedBalance()
}

func (b *balanceGrpc) Confirmed() uint64 {
	return b.infos.GetConfirmedBalance()
}
