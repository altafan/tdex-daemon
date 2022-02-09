package wallet

import (
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	"github.com/tdex-network/tdex-daemon/internal/infrastructure/oceanv1alpha"
)

type balanceGrpc struct {
	infos *oceanv1alpha.BalanceInfo
}

var _ ports.Balance = (*balanceGrpc)(nil)

func (b *balanceGrpc) Total() uint64 {
	return b.infos.TotalBalance
}

func (b *balanceGrpc) Unconfirmed() uint64 {
	return b.infos.UnconfirmedBalance
}

func (b *balanceGrpc) Confirmed() uint64 {
	return b.infos.ConfirmedBalance
}
