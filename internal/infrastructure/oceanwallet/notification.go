package oceanwallet

import (
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
)

type txEventType struct {
	t oceanv1alpha.TxEventType
}

var _ ports.TxEventType = (*txEventType)(nil)

func (t *txEventType) IsTxBroadcasted() bool {
	return t.t == oceanv1alpha.TxEventType_TX_EVENT_TYPE_BROADCASTED
}

func (t *txEventType) IsTxUnconfirmed() bool {
	return t.t == oceanv1alpha.TxEventType_TX_EVENT_TYPE_UNCONFIRMED
}

func (t *txEventType) IsTxConfirmed() bool {
	return t.t == oceanv1alpha.TxEventType_TX_EVENT_TYPE_CONFIRMED
}

func (t *txEventType) IsUnknown() bool {
	return t.t == oceanv1alpha.TxEventType_TX_EVENT_TYPE_UNSPECIFIED
}

type blockDetailsGrpc struct {
	details *oceanv1alpha.BlockDetails
}

var _ ports.BlockDetails = (*blockDetailsGrpc)(nil)

func (b *blockDetailsGrpc) Hash() string {
	h, err := chainhash.NewHash(b.details.GetHash())
	if err != nil {
		panic(err)
	}
	return h.String()
}

func (b *blockDetailsGrpc) Height() uint32 {
	return uint32(b.details.GetHeight())
}

func (b *blockDetailsGrpc) Timestamp() int64 {
	return b.details.GetTimestamp()
}

type txNotificationGrpc struct {
	resp *oceanv1alpha.TransactionNotificationsResponse
}

var _ ports.TxNotification = (*txNotificationGrpc)(nil)

func (n *txNotificationGrpc) TxID() string {
	return n.resp.GetTxid()
}

func (n *txNotificationGrpc) EventType() ports.TxEventType {
	return &txEventType{n.resp.GetEventType()}
}

func (n *txNotificationGrpc) BlockDetails() ports.BlockDetails {
	return &blockDetailsGrpc{n.resp.GetBlockDetails()}
}

type utxoEventType struct {
	t oceanv1alpha.UtxoEventType
}

var _ ports.UtxoEventType = (*utxoEventType)(nil)

func (t *utxoEventType) IsUtxoLocked() bool {
	return t.t == oceanv1alpha.UtxoEventType_UTXO_EVENT_TYPE_LOCKED
}

func (t *utxoEventType) IsUtxoSpent() bool {
	return t.t == oceanv1alpha.UtxoEventType_UTXO_EVENT_TYPE_SPENT
}

func (t *utxoEventType) IsUtxoUnlocked() bool {
	return t.t == oceanv1alpha.UtxoEventType_UTXO_EVENT_TYPE_UNLOCKED
}

func (t *utxoEventType) IsUnknown() bool {
	return t.t == oceanv1alpha.UtxoEventType_UTXO_EVENT_TYPE_UNSPECIFIED
}

type utxoNotificationGrpc struct {
	resp *oceanv1alpha.UtxosNotificationsResponse
}

var _ ports.UtxoNotification = (*utxoNotificationGrpc)(nil)

func (n *utxoNotificationGrpc) Utxo() ports.UtxoKey {
	return &utxoGrpc{n.resp.GetUtxo()}
}

func (n *utxoNotificationGrpc) EventType() ports.UtxoEventType {
	return &utxoEventType{n.resp.GetEventType()}
}
