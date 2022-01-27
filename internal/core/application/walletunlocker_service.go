package application

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
)

var (
	WalletInitializedReply = InitWalletReply{
		Message: "wallet is already initialized",
	}
)

type WalletUnlockerService interface {
	GenSeed(ctx context.Context) ([]string, error)
	InitWallet(
		ctx context.Context,
		mnemonic []string,
		passphrase string,
		restore bool,
		chRes chan InitWalletReply,
	)
	UnlockWallet(
		ctx context.Context,
		passphrase string,
	) error
	ChangePassword(
		ctx context.Context,
		currentPassphrase string,
		newPassphrase string,
	) error
	Status(ctx context.Context) (ports.WalletStatus, error)
	RegisterPassphraseChanHandler(func(PassphraseMsg))
	RegisterReadyChanHandler(func())
}

type walletUnlockerService struct {
	wallet        Wallet
	pubsubService ports.SecurePubSub

	lock              *sync.Mutex
	pwChan            pwChan
	readyChan         readyChan
	pwChanHandlers    []func(PassphraseMsg)
	readyChanHandlers []func()
}

func NewWalletUnlockerService(
	wallet Wallet, pubsubService ports.SecurePubSub,
) WalletUnlockerService {
	return newWalletUnlockerService(wallet, pubsubService)
}

func newWalletUnlockerService(
	wallet Wallet, pubsubService ports.SecurePubSub,
) *walletUnlockerService {
	w := &walletUnlockerService{
		wallet:            wallet,
		pubsubService:     pubsubService,
		lock:              &sync.Mutex{},
		pwChan:            newPwChan(),
		readyChan:         newReadyChan(),
		pwChanHandlers:    make([]func(PassphraseMsg), 0),
		readyChanHandlers: make([]func(), 0),
	}

	go w.handlePwChanHandlers()
	go w.handleReadyChanHandlers()

	return w
}

func (w *walletUnlockerService) GenSeed(
	ctx context.Context,
) ([]string, error) {
	return w.wallet.WalletManager().GenSeed(ctx)
}

func (w *walletUnlockerService) Status(
	ctx context.Context,
) (ports.WalletStatus, error) {
	return w.wallet.WalletManager().Status(ctx)
}

func (w *walletUnlockerService) InitWallet(
	ctx context.Context, mnemonic []string, passphrase string, restore bool,
	chRes chan InitWalletReply,
) {
	defer close(chRes)

	status, err := w.wallet.WalletManager().Status(ctx)
	if err != nil {
		chRes <- InitWalletReply{Err: err}
		return
	}
	if status.IsInitialized() {
		chRes <- WalletInitializedReply
		return
	}

	initMethod := w.wallet.WalletManager().CreateWallet
	if restore {
		initMethod = w.wallet.WalletManager().RestoreWallet
	}

	chMessages, chErr := make(chan string), make(chan error)
	go func() {
		if err := initMethod(ctx, mnemonic, passphrase, chMessages); err != nil {
			chErr <- err
		}
	}()

	forwardMessages := func() {
		for {
			select {
			case message, ok := <-chMessages:
				if !ok {
					return
				}
				chRes <- InitWalletReply{Message: message}
			case err = <-chErr:
				chRes <- InitWalletReply{Err: err}
				return
			}
		}
	}

	forwardMessages()

	go func() {
		if err == nil {
			if w.pubsubService != nil {
				if err := w.pubsubService.Store().Init(
					passphrase,
				); err != nil {
					log.WithError(err).Warn(
						"an error occured while initializing pubsub service. " +
							"Pubsub not available for the current session.",
					)
				}
			}
			w.pwChan.send(PassphraseMsg{
				Method:     InitWallet,
				CurrentPwd: passphrase,
			})
			w.readyChan.send(true)
		}
	}()
}

func (w *walletUnlockerService) UnlockWallet(
	ctx context.Context, passphrase string,
) error {
	if err := w.wallet.WalletManager().Unlock(ctx, passphrase); err != nil {
		return err
	}

	// Once the wallet is unlocked, this app service doesn't need to communicate
	// the password to upper levels anymore and the channel can be closed.
	defer w.pwChan.close()

	if w.pubsubService != nil {
		go func() {
			// For backward compatibility, check if the pubsub store has been
			// initialized by wallet.Init, otherwise it is initialized before being
			// unlocked here.
			if w.pubsubService.Store().IsLocked() {
				if err := w.pubsubService.Store().Init(
					passphrase,
				); err != nil {
					log.WithError(err).Warn(
						"an error occured while initializing pubsub service. " +
							"Pubsub not available for the current session.",
					)
					return
				}
			}
			if err := w.pubsubService.Store().Unlock(
				passphrase,
			); err != nil {
				log.WithError(err).Warn(
					"an error occured while unlocking pubsub internal store",
				)
			}
		}()
	}

	w.pwChan.send(PassphraseMsg{
		Method:     UnlockWallet,
		CurrentPwd: passphrase,
	})

	return nil
}

func (w *walletUnlockerService) ChangePassword(
	ctx context.Context, currentPassphrase, newPassphrase string,
) error {
	if err := w.wallet.WalletManager().ChangePassword(
		ctx, currentPassphrase, newPassphrase,
	); err != nil {
		return err
	}

	if w.pubsubService != nil {
		go func() {
			if err := w.pubsubService.Store().ChangePassword(
				currentPassphrase, newPassphrase,
			); err != nil {
				log.WithError(err).Warn(
					"an error occured while updating pubsub service password",
				)
			}
		}()
	}

	w.pwChan.send(PassphraseMsg{
		Method:     ChangePassphrase,
		CurrentPwd: currentPassphrase,
		NewPwd:     newPassphrase,
	})

	return nil
}

func (w *walletUnlockerService) RegisterPassphraseChanHandler(worker func(PassphraseMsg)) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.pwChanHandlers = append(w.pwChanHandlers, worker)
}

func (w *walletUnlockerService) RegisterReadyChanHandler(worker func()) {
	w.lock.Lock()
	defer w.lock.Unlock()
	w.readyChanHandlers = append(w.readyChanHandlers, worker)
}

func (w *walletUnlockerService) handlePwChanHandlers() {
	for msg := range w.pwChan.channel {
		for _, worker := range w.pwChanHandlers {
			worker(msg)
		}
	}
}

func (w *walletUnlockerService) handleReadyChanHandlers() {
	for isReady := range w.readyChan.channel {
		if isReady {
			for _, worker := range w.readyChanHandlers {
				worker()
			}
		}
	}
}
