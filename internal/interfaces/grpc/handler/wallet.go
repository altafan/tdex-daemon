package grpchandler

import (
	"context"
	"errors"

	"github.com/tdex-network/tdex-daemon/internal/core/application"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/tdex-network/tdex-daemon/api-spec/protobuf/gen/wallet"
)

type walletHandler struct {
	pb.UnimplementedWalletServer
	walletSvc application.WalletService
}

func NewWalletHandler(
	walletSvc application.WalletService,
) pb.WalletServer {
	return newWalletHandler(walletSvc)
}

func newWalletHandler(
	walletSvc application.WalletService,
) *walletHandler {
	return &walletHandler{
		walletSvc: walletSvc,
	}
}

func (w walletHandler) WalletAddress(
	ctx context.Context, req *pb.WalletAddressRequest,
) (*pb.WalletAddressReply, error) {
	return w.walletAddress(ctx, req)
}

func (w walletHandler) WalletBalance(
	ctx context.Context, req *pb.WalletBalanceRequest,
) (*pb.WalletBalanceReply, error) {
	return w.walletBalance(ctx, req)
}

func (w walletHandler) SendToMany(
	ctx context.Context, req *pb.SendToManyRequest,
) (*pb.SendToManyReply, error) {
	return w.sendToMany(ctx, req)
}

func (w walletHandler) walletAddress(
	ctx context.Context, req *pb.WalletAddressRequest,
) (*pb.WalletAddressReply, error) {
	info, err := w.walletSvc.GenerateAddressAndBlindingKey(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.WalletAddressReply{
		Address:  info.Address,
		Blinding: info.BlindingKey,
	}, nil
}

func (w walletHandler) walletBalance(
	ctx context.Context, req *pb.WalletBalanceRequest,
) (*pb.WalletBalanceReply, error) {
	b, err := w.walletSvc.GetBalance(ctx)
	if err != nil {
		return nil, err
	}

	balance := make(map[string]*pb.BalanceInfo)
	for k, v := range b {
		balance[k] = &pb.BalanceInfo{
			TotalBalance:       v.Total(),
			ConfirmedBalance:   v.Confirmed(),
			UnconfirmedBalance: v.Unconfirmed(),
		}
	}

	return &pb.WalletBalanceReply{Balance: balance}, nil
}

func (w walletHandler) sendToMany(
	ctx context.Context,
	req *pb.SendToManyRequest,
) (*pb.SendToManyReply, error) {
	outs := req.GetOutputs()
	if err := validateOutputs(outs); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	msatPerByte := req.GetMillisatPerByte()
	if err := validateMillisatPerByte(msatPerByte); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	outputs := make(application.Outputs, 0)
	for _, v := range outs {
		outputs = append(outputs, application.NewOutput(
			v.GetAddress(), v.GetAsset(), uint64(v.GetValue()),
		))
	}

	rawTx, txid, err := w.walletSvc.SendToMany(ctx, outputs, uint64(msatPerByte))
	if err != nil {
		return nil, err
	}

	return &pb.SendToManyReply{RawTx: rawTx, Txid: txid}, nil
}

func validateOutputs(outputs []*pb.TxOut) error {
	if len(outputs) <= 0 {
		return errors.New("output list is empty")
	}
	for _, o := range outputs {
		if o == nil ||
			len(o.GetAsset()) <= 0 ||
			o.GetValue() <= 0 ||
			len(o.GetAddress()) <= 0 {
			return errors.New("output list is malformed")
		}
	}
	return nil
}

func validateMillisatPerByte(satPerByte int64) error {
	if satPerByte < domain.MinMilliSatPerByte {
		return errors.New("milli sats per byte is too low")
	}
	return nil
}
