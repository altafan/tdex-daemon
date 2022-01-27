package application_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tdex-network/tdex-daemon/internal/core/application"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	dbbadger "github.com/tdex-network/tdex-daemon/internal/infrastructure/storage/db/badger"
)

var (
	restore    = true
	passphrase = "passphrase"
	mnemonic   = []string{
		"curious", "alien", "peanut", "protect", "capable", "charge", "recipe", "hub",
		"volume", "deal", "math", "make", "suggest", "bleak", "seat", "swim",
		"into", "save", "hint", "wood", "pioneer", "ball", "decline", "universe",
	}
	ctx = context.Background()
)

func TestMain(m *testing.M) {
	mockedPsetParser := &mockPsetParser{}
	mockedPsetParser.On("GetTxID", mock.AnythingOfType("string")).Return(randomHex(32), nil)
	mockedPsetParser.On("GetTxHex", mock.AnythingOfType("string")).Return(randomHex(1000), nil)
	domain.PsetParserManager = mockedPsetParser

	mockedSwapRequest := newMockedSwapRequest()
	mockedSwapAccept := newMockedSwapAccept()
	mockedSwapComplete := newMockedSwapComplete()
	mockedSwapFail := newMockedSwapFail()

	mockedSwapParser := &mockSwapParser{}
	mockedSwapParser.
		On("SerializeRequest", mock.Anything).Return(randomBytes(100), nil)
	mockedSwapParser.
		On("SerializeAccept", mock.Anything).Return(mockedSwapAccept.GetId(), randomBytes(100), nil)
	mockedSwapParser.
		On("SerializeComplete", mock.Anything, mock.Anything).Return(mockedSwapComplete.GetId(), randomBytes(100), nil)
	mockedSwapParser.
		On("SerializeFail", mock.Anything, mock.Anything, mock.Anything).Return(mockedSwapFail.GetId(), nil)
	mockedSwapParser.
		On("DeserializeRequest", mock.Anything).Return(mockedSwapRequest, nil)
	mockedSwapParser.
		On("DeserializeAccept", mock.Anything).Return(mockedSwapAccept, nil)
	mockedSwapParser.
		On("DeserializeComplete", mock.Anything).Return(mockedSwapComplete, nil)
	mockedSwapParser.
		On("DeserializeFail", mock.Anything).Return(mockedSwapFail, nil)
	domain.SwapParserManager = mockedSwapParser

	os.Exit(m.Run())
}

func TestInitWallet(t *testing.T) {
	t.Run("wallet_from_scratch", func(t *testing.T) {
		walletSvc := newWalletUnlockerService()
		require.NotNil(t, walletSvc)

		walletSvc.RegisterPassphraseChanHandler(func(msg application.PassphraseMsg) {
			require.True(t, (msg.Method == application.InitWallet || msg.Method == application.UnlockWallet))
			require.Equal(t, passphrase, msg.CurrentPwd)
		})

		walletStatus, err := walletSvc.Status(ctx)
		require.NoError(t, err)
		require.False(t, walletStatus.IsInitialized())
		require.False(t, walletStatus.IsSynced())
		require.False(t, walletStatus.IsUnlocked())

		seed, err := walletSvc.GenSeed(ctx)
		require.NoError(t, err)
		require.Equal(t, 24, len(seed))

		chReplies := make(chan application.InitWalletReply)
		go walletSvc.InitWallet(ctx, mnemonic, passphrase, !restore, chReplies)

		replies, err := listenToReplies(chReplies)
		require.NoError(t, err)
		require.NotZero(t, replies)

		walletStatus, err = walletSvc.Status(ctx)
		require.NoError(t, err)
		require.True(t, walletStatus.IsInitialized())
		require.True(t, walletStatus.IsSynced())
		require.False(t, walletStatus.IsUnlocked())

		err = walletSvc.UnlockWallet(ctx, passphrase)
		require.NoError(t, err)

		walletStatus, err = walletSvc.Status(ctx)
		require.NoError(t, err)
		require.True(t, walletStatus.IsInitialized())
		require.True(t, walletStatus.IsSynced())
		require.True(t, walletStatus.IsUnlocked())
	})

	t.Run("wallet_from_restart", func(t *testing.T) {
		// walletSvc with no workers registered for passphrase and ready channels.
		walletSvc := newWalletUnlockerServiceRestart()
		require.NotNil(t, walletSvc)

		walletStatus, err := walletSvc.Status(ctx)
		require.NoError(t, err)
		require.True(t, walletStatus.IsInitialized())
		require.True(t, walletStatus.IsSynced())
		require.False(t, walletStatus.IsUnlocked())

		err = walletSvc.UnlockWallet(ctx, "wrongpassphrase")
		require.Error(t, err)

		walletStatus, err = walletSvc.Status(ctx)
		require.NoError(t, err)
		require.True(t, walletStatus.IsInitialized())
		require.True(t, walletStatus.IsSynced())
		require.False(t, walletStatus.IsUnlocked())

		err = walletSvc.UnlockWallet(ctx, passphrase)
		require.NoError(t, err)

		walletStatus, err = walletSvc.Status(ctx)
		require.NoError(t, err)
		require.True(t, walletStatus.IsInitialized())
		require.True(t, walletStatus.IsSynced())
		require.True(t, walletStatus.IsUnlocked())
	})

	t.Run("wallet_from_restore", func(t *testing.T) {
		walletSvc := newWalletUnlockerServiceRestore()
		require.NotNil(t, walletSvc)

		walletSvc.RegisterPassphraseChanHandler(func(msg application.PassphraseMsg) {
			require.True(t, (msg.Method == application.InitWallet || msg.Method == application.UnlockWallet))
			require.Equal(t, passphrase, msg.CurrentPwd)
		})

		walletStatus, err := walletSvc.Status(ctx)
		require.NoError(t, err)
		require.False(t, walletStatus.IsInitialized())
		require.False(t, walletStatus.IsSynced())
		require.False(t, walletStatus.IsUnlocked())

		chReplies := make(chan application.InitWalletReply)
		go walletSvc.InitWallet(ctx, mnemonic, passphrase, restore, chReplies)

		replies, err := listenToReplies(chReplies)
		require.NoError(t, err)
		require.Greater(t, len(replies), 0)

		// initialized but not yet synced
		walletStatus, err = walletSvc.Status(ctx)
		require.NoError(t, err)
		require.True(t, walletStatus.IsInitialized())
		require.False(t, walletStatus.IsSynced())
		require.False(t, walletStatus.IsUnlocked())

		err = walletSvc.UnlockWallet(ctx, passphrase)
		require.NoError(t, err)

		// initialized, unlocked but still not synced.
		walletStatus, err = walletSvc.Status(ctx)
		require.NoError(t, err)
		require.True(t, walletStatus.IsInitialized())
		require.False(t, walletStatus.IsSynced())
		require.True(t, walletStatus.IsUnlocked())

		// initialized, unlocked and finally synced.
		walletStatus, err = walletSvc.Status(ctx)
		require.NoError(t, err)
		require.True(t, walletStatus.IsInitialized())
		require.True(t, walletStatus.IsSynced())
		require.True(t, walletStatus.IsUnlocked())
	})
}

func newWalletUnlockerService() application.WalletUnlockerService {
	_, wallet := newServices()
	walletManager := wallet.(*mockedWallet).walletManager
	walletManager.On("GenSeed", mock.Anything).Return(mnemonic, nil)
	walletManager.On("Unlock", mock.Anything, mock.Anything).Return(nil)
	walletManager.On("CreateWallet", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	walletManager.On("Status", mock.Anything).Return(walletStatus{false, false, false}, nil).Twice()
	walletManager.On("Status", mock.Anything).Return(walletStatus{true, true, false}, nil).Once()
	walletManager.On("Status", mock.Anything).Return(walletStatus{true, true, true}, nil).Once()
	return application.NewWalletUnlockerService(wallet, nil)
}

func newWalletUnlockerServiceRestart() application.WalletUnlockerService {
	_, wallet := newServices()
	walletManager := wallet.(*mockedWallet).walletManager
	walletManager.On("Unlock", mock.Anything, mock.Anything).Return(errors.New("invalid passphrase")).Once()
	walletManager.On("Unlock", mock.Anything, mock.Anything).Return(nil).Once()
	walletManager.On("Status", mock.Anything).Return(walletStatus{true, true, false}, nil).Twice()
	walletManager.On("Status", mock.Anything).Return(walletStatus{true, true, true}, nil).Once()
	return application.NewWalletUnlockerService(wallet, nil)
}

func newWalletUnlockerServiceRestore() application.WalletUnlockerService {
	_, wallet := newServices()
	walletManager := wallet.(*mockedWallet).walletManager
	walletManager.On("Unlock", mock.Anything, mock.Anything).Return(nil)
	walletManager.On("RestoreWallet", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	walletManager.On("Status", mock.Anything).Return(walletStatus{false, false, false}, nil).Twice()
	walletManager.On("Status", mock.Anything).Return(walletStatus{true, false, false}, nil).Once()
	walletManager.On("Status", mock.Anything).Return(walletStatus{true, false, true}, nil).Once()
	walletManager.On("Status", mock.Anything).Return(walletStatus{true, true, true}, nil).Once()
	return application.NewWalletUnlockerService(wallet, nil)
}

func newServices() (ports.RepoManager, application.Wallet) {
	repoManager, _ := dbbadger.NewRepoManager("", nil)
	wallet := newMockedWallet()
	return repoManager, wallet
}

func listenToReplies(
	chReplies chan application.InitWalletReply,
) ([]string, error) {
	replies := make([]string, 0)
	for reply := range chReplies {
		if err := reply.Err; err != nil {
			return nil, err
		}
		replies = append(replies, reply.Message)
	}
	return replies, nil
}

func randomId() string {
	return uuid.New().String()
}

func randomHex(len int) string {
	return hex.EncodeToString(randomBytes(len))
}

func randomValue() uint64 {
	return uint64(randomIntInRange(1000000, 10000000000))
}

func randomBytes(len int) []byte {
	b := make([]byte, len)
	rand.Read(b)
	return b
}

func randomIntInRange(min, max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(int(n.Int64())) + min
}

func randomBase64() string {
	return base64.StdEncoding.EncodeToString(randomBytes(100))
}
