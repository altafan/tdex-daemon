package oceanwallet

import (
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	"github.com/vulpemventures/go-elements/transaction"
)

type txDetailsHex struct {
	hex  string
	hash string
}

func newTxDetails(hex string) (ports.TxDetails, error) {
	tx, err := transaction.NewTxFromHex(hex)
	if err != nil {
		return nil, err
	}

	return &txDetailsHex{hex: hex, hash: tx.TxHash().String()}, nil
}

func (t *txDetailsHex) Hex() string {
	return t.hex
}

func (t *txDetailsHex) Hash() string {
	return t.hash
}
