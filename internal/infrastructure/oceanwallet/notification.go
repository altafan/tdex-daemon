package oceanwallet

import (
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
)

type txEventType struct {
	t oceanv1alpha.TxEventType
}

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

type txNotificationGrpc struct {
	resp *oceanv1alpha.TransactionNotificationsResponse
}

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

func (n *utxoNotificationGrpc) AccountIndex() uint64 {
	return uint64(n.resp.GetAccountKey().GetId())
}

func (n *utxoNotificationGrpc) Utxo() ports.UtxoKey {
	return &utxoGrpc{n.resp.GetUtxo()}
}

func (n *utxoNotificationGrpc) EventType() ports.UtxoEventType {
	return &utxoEventType{n.resp.GetEventType()}
}
