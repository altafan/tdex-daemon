package wallet

import (
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	"github.com/tdex-network/tdex-daemon/internal/infrastructure/oceanv1alpha"
	"github.com/vulpemventures/go-elements/elementsutil"
)

type utxoGrpc struct {
	grpcUtxo *oceanv1alpha.Utxo
}

func newUtxoGrpc(grpcUtxo *oceanv1alpha.Utxo) *utxoGrpc {
	return &utxoGrpc{
		grpcUtxo: grpcUtxo,
	}
}

var _ ports.UtxoKey = (*utxoGrpc)(nil)
var _ ports.Utxo = (*utxoGrpc)(nil)

func (u *utxoGrpc) TxID() string {
	return u.grpcUtxo.Txid
}

func (u *utxoGrpc) Index() uint32 {
	return uint32(u.grpcUtxo.Index)
}

func (u *utxoGrpc) Key() ports.UtxoKey {
	return u
}

func (u *utxoGrpc) Asset() string {
	return string(u.grpcUtxo.Asset) // TODO change after ocean changes
}

func (u *utxoGrpc) Value() uint64 {
	v, e := elementsutil.ElementsToSatoshiValue(u.grpcUtxo.Value)
	if e != nil {
		panic(e)
	}

	return v
}

func (u *utxoGrpc) Script() []byte {
	return u.grpcUtxo.Script
}

func (u *utxoGrpc) IsConfirmed() bool {
	return u.grpcUtxo.IsConfirmed
}

func (u *utxoGrpc) IsLocked() bool {
	return u.grpcUtxo.IsLocked
}
