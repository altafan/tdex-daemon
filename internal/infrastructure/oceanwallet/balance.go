package oceanwallet

import (
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
)

type balanceGrpc struct {
	infos *oceanv1alpha.BalanceInfo
}

func (b *balanceGrpc) Total() uint64 {
	return b.infos.GetTotalBalance()
}

func (b *balanceGrpc) Unconfirmed() uint64 {
	return b.infos.GetConfirmedBalance()
}

func (b *balanceGrpc) Confirmed() uint64 {
	return b.infos.GetConfirmedBalance()
}
