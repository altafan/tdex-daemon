package oceanwallet

import (
	"context"

	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
	"google.golang.org/grpc"
)

type transactionManagerGrpc struct {
	client oceanv1alpha.TransactionServiceClient
}

func newTransactionManagerGrpc(conn *grpc.ClientConn) ports.TransactionManager {
	client := oceanv1alpha.NewTransactionServiceClient(conn)
	return &transactionManagerGrpc{
		client: client,
	}
}

func strategyToProto(s ports.Strategy) oceanv1alpha.SelectUtxosRequest_Strategy {
	if s.IsBranchBound() {
		return oceanv1alpha.SelectUtxosRequest_STRATEGY_BRANCH_BOUND
	}

	if s.IsFragment() {
		return oceanv1alpha.SelectUtxosRequest_STRATEGY_FRAGMENT
	}

	return oceanv1alpha.SelectUtxosRequest_STRATEGY_UNSPECIFIED
}

func (tm *transactionManagerGrpc) GetTransaction(ctx context.Context, txid string) (ports.TxDetails, ports.BlockDetails, error) {
	req := &oceanv1alpha.GetTransactionRequest{
		Txid: txid,
	}

	resp, err := tm.client.GetTransaction(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	txDetails, err := newTxDetails(resp.GetTxHex())
	if err != nil {
		return nil, nil, err
	}

	return txDetails, &blockDetailsGrpc{details: resp.GetBlockDetails()}, nil
}

func (tm *transactionManagerGrpc) SelectUnspentsForAccount(
	ctx context.Context, account string,
	targetAsset string, targetAmount uint64, strategy ports.Strategy,
) (utxos []ports.UtxoKey, change uint64, err error) {
	req := &oceanv1alpha.SelectUtxosRequest{
		AccountKey:   &oceanv1alpha.AccountKey{Name: account},
		TargetAsset:  targetAsset,
		TargetAmount: targetAmount,
		Strategy:     strategyToProto(strategy),
	}

	resp, err := tm.client.SelectUtxos(ctx, req)
	if err != nil {
		return nil, 0, err
	}

	utxos = make([]ports.UtxoKey, len(resp.GetUtxos().GetUtxos()))
	for i, u := range resp.GetUtxos().GetUtxos() {
		utxos[i] = newUtxoGrpc(u)
	}

	return utxos, resp.GetChange(), nil
}

func (tm *transactionManagerGrpc) EstimateFees(
	ctx context.Context, inputs []ports.Input, outputs []ports.Output,
) (uint64, error) {
	req := &oceanv1alpha.EstimateFeesRequest{
		Inputs:  make([]*oceanv1alpha.Input, len(inputs)),
		Outputs: make([]*oceanv1alpha.Output, len(outputs)),
	}

	for i, in := range inputs {
		req.Inputs[i] = &oceanv1alpha.Input{
			Txid:  in.TxID(),
			Index: in.Index(),
		}
	}

	for i, out := range outputs {
		req.Outputs[i] = &oceanv1alpha.Output{
			Asset:   out.Asset(),
			Amount:  out.Value(),
			Address: out.Address(),
		}
	}

	resp, err := tm.client.EstimateFees(ctx, req)
	if err != nil {
		return 0, err
	}

	return resp.GetFeeAmount(), nil
}

func (tm *transactionManagerGrpc) TransferFromAccount(
	ctx context.Context, account string, outputs []ports.Output,
	millisatPerByte uint64,
) (string, error) {
	req := &oceanv1alpha.TransferRequest{
		AccountKey: &oceanv1alpha.AccountKey{Name: account},
		Receivers:  make([]*oceanv1alpha.Output, len(outputs)),
	}

	for i, out := range outputs {
		req.Receivers[i] = &oceanv1alpha.Output{
			Asset:   out.Asset(),
			Amount:  out.Value(),
			Address: out.Address(),
		}
	}

	resp, err := tm.client.Transfer(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.GetTxHex(), nil
}

func (tm *transactionManagerGrpc) CreateTransaction(
	ctx context.Context, inputs []ports.Input, outputs []ports.Output,
) (string, error) {
	req := &oceanv1alpha.CreatePsetRequest{
		Inputs:  make([]*oceanv1alpha.Input, len(inputs)),
		Outputs: make([]*oceanv1alpha.Output, len(outputs)),
	}

	for i, in := range inputs {
		req.Inputs[i] = &oceanv1alpha.Input{
			Txid:  in.TxID(),
			Index: in.Index(),
		}
	}

	for i, out := range outputs {
		req.Outputs[i] = &oceanv1alpha.Output{
			Asset:   out.Asset(),
			Amount:  out.Value(),
			Address: out.Address(),
		}
	}

	resp, err := tm.client.CreatePset(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.GetPset(), nil
}

func (tm *transactionManagerGrpc) UpdateTransaction(ctx context.Context, psetBase64 string, inputs []ports.Input, outputs []ports.Output) (
	updatedPset string,
	inBlindKeysByScript, outBlindKeysByScrpt map[string][]byte,
	err error,
) {
	req := &oceanv1alpha.UpdatePsetRequest{
		Pset:    psetBase64,
		Inputs:  make([]*oceanv1alpha.Input, len(inputs)),
		Outputs: make([]*oceanv1alpha.Output, len(outputs)),
	}

	for i, in := range inputs {
		req.Inputs[i] = &oceanv1alpha.Input{
			Txid:  in.TxID(),
			Index: in.Index(),
		}
	}

	for i, out := range outputs {
		req.Outputs[i] = &oceanv1alpha.Output{
			Asset:   out.Asset(),
			Amount:  out.Value(),
			Address: out.Address(),
		}
	}

	resp, err := tm.client.UpdatePset(ctx, req)
	if err != nil {
		return "", nil, nil, err
	}

	// TODO blinding keys maps
	return resp.GetPset(), nil, nil, nil
}

func (tm *transactionManagerGrpc) BlindTransaction(ctx context.Context, psetBase64 string, lastBlinder bool) (string, error) {
	req := &oceanv1alpha.BlindPsetRequest{
		Pset:        psetBase64,
		LastBlinder: lastBlinder,
	}

	resp, err := tm.client.BlindPset(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.GetPset(), nil
}

func (tm *transactionManagerGrpc) SignTransaction(ctx context.Context, psetBase64 string, extractRawTx bool) (string, error) {
	req := &oceanv1alpha.SignPsetRequest{
		Pset: psetBase64,
	}

	resp, err := tm.client.SignPset(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.GetPset(), nil
}

func (tm *transactionManagerGrpc) BroadcastTransaction(ctx context.Context, txHex string) (string, error) {
	req := &oceanv1alpha.BroadcastTransactionRequest{
		TxHex: txHex,
	}

	resp, err := tm.client.BroadcastTransaction(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.GetTxid(), nil
}
