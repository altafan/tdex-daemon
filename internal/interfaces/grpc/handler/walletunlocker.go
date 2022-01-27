package grpchandler

import (
	"context"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"os"

	"github.com/tdex-network/tdex-daemon/internal/core/application"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/tdex-network/tdex-daemon/api-spec/protobuf/gen/walletunlocker"
)

type walletUnlockerHandler struct {
	pb.UnimplementedWalletUnlockerServer
	walletUnlockerSvc application.WalletUnlockerService
	adminMacaroonPath string
}

func NewWalletUnlockerHandler(
	walletUnlockerSvc application.WalletUnlockerService,
	adminMacPath string,
) pb.WalletUnlockerServer {
	return newWalletUnlockerHandler(walletUnlockerSvc, adminMacPath)
}

func newWalletUnlockerHandler(
	walletUnlockerSvc application.WalletUnlockerService,
	adminMacPath string,
) *walletUnlockerHandler {
	return &walletUnlockerHandler{
		walletUnlockerSvc: walletUnlockerSvc,
		adminMacaroonPath: adminMacPath,
	}
}

func (w *walletUnlockerHandler) GenSeed(
	ctx context.Context, req *pb.GenSeedRequest,
) (*pb.GenSeedReply, error) {
	return w.genSeed(ctx, req)
}

func (w *walletUnlockerHandler) InitWallet(
	req *pb.InitWalletRequest, stream pb.WalletUnlocker_InitWalletServer,
) error {
	return w.initWallet(req, stream)
}

func (w *walletUnlockerHandler) UnlockWallet(
	ctx context.Context, req *pb.UnlockWalletRequest,
) (*pb.UnlockWalletReply, error) {
	return w.unlockWallet(ctx, req)
}

func (w *walletUnlockerHandler) ChangePassword(
	ctx context.Context, req *pb.ChangePasswordRequest,
) (*pb.ChangePasswordReply, error) {
	return w.changePassword(ctx, req)
}

func (w *walletUnlockerHandler) IsReady(
	ctx context.Context, req *pb.IsReadyRequest,
) (*pb.IsReadyReply, error) {
	return w.isReady(ctx, req)
}

func (w *walletUnlockerHandler) genSeed(
	ctx context.Context, req *pb.GenSeedRequest,
) (*pb.GenSeedReply, error) {
	mnemonic, err := w.walletUnlockerSvc.GenSeed(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.GenSeedReply{SeedMnemonic: mnemonic}, nil
}

func (w *walletUnlockerHandler) initWallet(
	req *pb.InitWalletRequest, stream pb.WalletUnlocker_InitWalletServer,
) error {
	mnemonic := req.GetSeedMnemonic()
	if err := validateMnemonic(mnemonic); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	password := req.GetWalletPassword()
	if err := validatePassword(password); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}

	chReplies := make(chan application.InitWalletReply)
	go w.walletUnlockerSvc.InitWallet(
		stream.Context(), mnemonic, string(password), req.GetRestore(), chReplies,
	)

	isAlreadyInitialized := false
	for reply := range chReplies {
		if err := reply.Err; err != nil {
			return err
		}
		if reply == application.WalletInitializedReply {
			isAlreadyInitialized = true
		}
		if err := stream.Send(
			&pb.InitWalletReply{Message: reply.Message},
		); err != nil {
			return err
		}
	}

	// Inject admin.macaroon to InitWalletReply only if the app service has
	// actually initialized the internal wallet.
	// If the reply channel didn't contain any message before closing, it means
	// that the app service skipped the operation because the wallet was already
	// initialized.
	if !isAlreadyInitialized && w.adminMacaroonPath != "" {
		var mac []byte
		var err error
		// Retry reading the admin.macaroon file until it's found in the datadir.
		for {
			mac, err = ioutil.ReadFile(w.adminMacaroonPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			break
		}
		macStr := hex.EncodeToString(mac)
		if err := stream.Send(&pb.InitWalletReply{
			Message: macStr,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (w *walletUnlockerHandler) unlockWallet(
	ctx context.Context, req *pb.UnlockWalletRequest,
) (*pb.UnlockWalletReply, error) {
	password := req.GetWalletPassword()
	if err := validatePassword(password); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := w.walletUnlockerSvc.UnlockWallet(ctx, string(password)); err != nil {
		return nil, err
	}

	return &pb.UnlockWalletReply{}, nil
}

func (w *walletUnlockerHandler) changePassword(
	ctx context.Context, req *pb.ChangePasswordRequest,
) (*pb.ChangePasswordReply, error) {
	currentPwd := req.GetCurrentPassword()
	if err := validatePassword(currentPwd); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	newPwd := req.GetNewPassword()
	if err := validatePassword(newPwd); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if err := w.walletUnlockerSvc.ChangePassword(
		ctx,
		string(currentPwd),
		string(newPwd),
	); err != nil {
		return nil, err
	}

	return &pb.ChangePasswordReply{}, nil
}

func (w *walletUnlockerHandler) isReady(
	ctx context.Context, _ *pb.IsReadyRequest,
) (*pb.IsReadyReply, error) {
	status, err := w.walletUnlockerSvc.Status(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.IsReadyReply{
		Initialized: status.IsInitialized(),
		Unlocked:    status.IsUnlocked(),
		Synced:      status.IsSynced(),
	}, nil
}

func validateMnemonic(mnemonic []string) error {
	if len(mnemonic) <= 0 {
		return errors.New("mnemonic is null")
	}
	return nil
}

func validatePassword(password []byte) error {
	if len(password) <= 0 {
		return errors.New("password is null")
	}
	return nil
}
