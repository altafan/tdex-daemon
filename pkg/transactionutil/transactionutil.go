package transactionutil

import (
	"encoding/hex"
	"fmt"

	"github.com/vulpemventures/go-elements/network"
	"github.com/vulpemventures/go-elements/pset"

	"github.com/tdex-network/tdex-daemon/pkg/bufferutil"
	"github.com/vulpemventures/go-elements/confidential"
	"github.com/vulpemventures/go-elements/elementsutil"
	"github.com/vulpemventures/go-elements/transaction"
)

type UnblindedResult struct {
	AssetHash    string
	Value        uint64
	AssetBlinder []byte
	ValueBlinder []byte
}

func UnblindOutput(
	utxo *transaction.TxOutput,
	blindKey []byte,
) (*UnblindedResult, bool) {
	if utxo.IsConfidential() {
		revealed, err := confidential.UnblindOutputWithKey(utxo, blindKey)
		if err != nil {
			return nil, false
		}
		return &UnblindedResult{
			AssetHash:    hex.EncodeToString(elementsutil.ReverseBytes(revealed.Asset)),
			Value:        revealed.Value,
			AssetBlinder: revealed.AssetBlindingFactor,
			ValueBlinder: revealed.ValueBlindingFactor,
		}, true
	}

	return &UnblindedResult{
		AssetHash:    bufferutil.AssetHashFromBytes(utxo.Asset),
		Value:        bufferutil.ValueFromBytes(utxo.Value),
		AssetBlinder: make([]byte, 32),
		ValueBlinder: make([]byte, 32),
	}, true
}

func NewFeeOutput(feeAmount uint64, net *network.Network) []*transaction.TxOutput {
	feeAsset, _ := bufferutil.AssetHashToBytes(net.AssetID)
	feeValue, _ := bufferutil.ValueToBytes(feeAmount)
	feeScript := make([]byte, 0)
	return []*transaction.TxOutput{
		transaction.NewTxOutput(feeAsset, feeValue, feeScript),
	}
}

func GetTxIdFromPset(psetBase64 string) (string, error) {
	p, err := pset.NewPsetFromBase64(psetBase64)
	if err != nil {
		return "", err
	}

	return p.UnsignedTx.TxHash().String(), nil
}

func GetTxIdFromHex(txHex string) (string, error) {
	tx, err := transaction.NewTxFromHex(txHex)
	if err != nil {
		return "", err
	}
	return tx.TxHash().String(), nil
}

// FinalizeAndExtractTransaction attempts to finalize the provided partial
// transaction and eventually extracts the final transaction and returns
// it in hex string format, along with its transaction id
func FinalizeAndExtractTransaction(psetBase64 string) (string, string, error) {
	ptx, err := pset.NewPsetFromBase64(psetBase64)
	if err != nil {
		return "", "", fmt.Errorf("invalid pset: %s", err)
	}

	ok, err := ptx.ValidateAllSignatures()
	if err != nil {
		return "", "", err
	}
	if !ok {
		return "", "", fmt.Errorf("invalid pset: failed to verify all signatures")
	}

	if err := pset.FinalizeAll(ptx); err != nil {
		return "", "", err
	}

	tx, err := pset.Extract(ptx)
	if err != nil {
		return "", "", err
	}
	txHex, err := tx.ToHex()
	if err != nil {
		return "", "", err
	}
	return txHex, tx.TxHash().String(), nil
}
