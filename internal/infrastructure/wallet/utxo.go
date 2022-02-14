package wallet

import (
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
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
	return u.grpcUtxo.Asset
}

func (u *utxoGrpc) Value() uint64 {
	return u.grpcUtxo.Value
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
