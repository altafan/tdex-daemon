package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"github.com/tdex-network/tdex-daemon/config"
	"github.com/tdex-network/tdex-daemon/internal/core/application"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	"github.com/tdex-network/tdex-daemon/internal/infrastructure/oceanwallet"
	webhookpubsub "github.com/tdex-network/tdex-daemon/internal/infrastructure/pubsub/webhook"
	dbbadger "github.com/tdex-network/tdex-daemon/internal/infrastructure/storage/db/badger"
	"github.com/tdex-network/tdex-daemon/internal/interfaces"
	grpcinterface "github.com/tdex-network/tdex-daemon/internal/interfaces/grpc"
	"github.com/tdex-network/tdex-daemon/pkg/explorer/esplora"
	boltsecurestore "github.com/tdex-network/tdex-daemon/pkg/securestore/bolt"
	"github.com/tdex-network/tdex-daemon/pkg/stats"

	_ "net/http/pprof" // #nosec
)

var (
	// General config
	logLevel                = config.GetInt(config.LogLevelKey)
	profilerEnabled         = config.GetBool(config.EnableProfilerKey)
	datadir                 = config.GetDatadir()
	dbDir                   = filepath.Join(datadir, config.DbLocation)
	profilerDir             = filepath.Join(datadir, config.ProfilerLocation)
	noMacaroons             = config.GetBool(config.NoMacaroonsKey)
	statsIntervalInSeconds  = config.GetDuration(config.StatsIntervalKey) * time.Second
	tradeTLSKey             = config.GetString(config.TradeTLSKeyKey)
	tradeTLSCert            = config.GetString(config.TradeTLSCertKey)
	operatorTLSExtraIPs     = config.GetStringSlice(config.OperatorExtraIPKey)
	operatorTLSExtraDomains = config.GetStringSlice(config.OperatorExtraDomainKey)
	// App services config
	marketsFee                    = int64(config.GetFloat(config.DefaultFeeKey) * 100)
	marketsBaseAsset              = config.GetString(config.BaseAssetKey)
	marketsQuoteAsset             = config.GetString(config.QuoteAssetKey)
	tradesExpiryDurationInSeconds = config.GetDuration(config.TradeExpiryTimeKey) * time.Second
	tradesSatsPerByte             = config.GetFloat(config.TradeSatsPerByte)
	pricesSlippagePercentage      = decimal.NewFromFloat(config.GetFloat(config.PriceSlippageKey))
	feeThreshold                  = uint64(config.GetInt(config.FeeAccountBalanceThresholdKey))
	tradeSvcPort                  = config.GetInt(config.TradeListeningPortKey)
	operatorSvcPort               = config.GetInt(config.OperatorListeningPortKey)
	httpClientReqTimeout          = config.GetDuration(config.ExplorerRequestTimeoutKey)
)

func main() {
	log.SetLevel(log.Level(logLevel))

	// Profiler is enabled at url http://localhost:8024/debug/pprof/
	if profilerEnabled {
		runtime.SetBlockProfileRate(1)
		go func() {
			http.ListenAndServe(":8024", nil)
		}()
	}

	webhookPubSub, err := newWebhookPubSubService(dbDir, httpClientReqTimeout)
	if err != nil {
		log.Errorf("error while setting up webhook pubsub service: %s", err)
		return
	}

	// TODO: move to config ? or constant ?
	// who is running the ocean server ? the daemon ?
	addrOceanServer := "localhost:50051"
	oceanWallet := oceanwallet.New(addrOceanServer)

	wallet, err := application.NewWallet(oceanWallet)
	if err != nil {
		log.Errorf("error while setting up internal wallet: %s", err)
		return
	}

	// Init services to be used by those of the application layer.
	repoManager, err := dbbadger.NewRepoManager(dbDir, log.New())
	if err != nil {
		log.Errorf("error while opening db: %s", err)
		return
	}
	// TODO: integrate webhooks and repo events

	// Init application services
	tradeSvc := application.NewTradeService(
		repoManager,
		wallet,
		webhookPubSub,
		tradesExpiryDurationInSeconds,
		tradesSatsPerByte,
		pricesSlippagePercentage,
		feeThreshold,
	)
	operatorSvc := application.NewOperatorService(
		repoManager,
		wallet,
		webhookPubSub,
		marketsBaseAsset,
		marketsQuoteAsset,
		marketsFee,
		feeThreshold,
	)
	walletSvc := application.NewWalletService(
		repoManager, wallet, marketsFee,
	)
	walletUnlockerSvc := application.NewWalletUnlockerService(
		nil, webhookPubSub,
	)

	// Init gRPC interfaces.
	opts := grpcinterface.ServiceOpts{
		NoMacaroons:          noMacaroons,
		Datadir:              datadir,
		DBLocation:           config.DbLocation,
		TLSLocation:          config.TLSLocation,
		MacaroonsLocation:    config.MacaroonsLocation,
		OperatorExtraIPs:     operatorTLSExtraIPs,
		OperatorExtraDomains: operatorTLSExtraDomains,
		OperatorAddress:      fmt.Sprintf(":%d", operatorSvcPort),
		TradeAddress:         fmt.Sprintf(":%d", tradeSvcPort),
		TradeTLSKey:          tradeTLSKey,
		TradeTLSCert:         tradeTLSCert,
		WalletSvc:            walletSvc,
		WalletUnlockerSvc:    walletUnlockerSvc,
		OperatorSvc:          operatorSvc,
		TradeSvc:             tradeSvc,
	}
	svc, err := grpcinterface.NewService(opts)
	if err != nil {
		repoManager.Close()

		log.Errorf("error while setting up gRPC service: %s", err)
		return
	}

	log.Info("starting daemon")

	var cancelStats context.CancelFunc
	if log.GetLevel() >= log.DebugLevel {
		var ctx context.Context
		ctx, cancelStats = context.WithCancel(context.Background())
		stats.EnableMemoryStatistics(ctx, statsIntervalInSeconds, profilerDir)
	}

	defer stop(repoManager, webhookPubSub, svc, cancelStats)

	// Start gRPC service interfaces.
	if err := svc.Start(); err != nil {
		log.Errorf("error while starting daemon: %s", err)
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	<-sigChan

	log.Info("shutting down daemon")
}

func stop(
	repoManager ports.RepoManager,
	pubsubSvc ports.SecurePubSub,
	svc interfaces.Service,
	cancelStats context.CancelFunc,
) {
	if profilerEnabled && log.GetLevel() >= log.DebugLevel {
		cancelStats()
		time.Sleep(1 * time.Second)
		log.Debug("stopped profiler")
	}

	svc.Stop()

	pubsubSvc.Store().Close()
	log.Debug("stopped pubsub service")

	repoManager.Close()
	log.Debug("closed connection with database")

	log.Info("disabled all active interfaces. Exiting")
}

func newWebhookPubSubService(
	datadir string, reqTimeout time.Duration,
) (ports.SecurePubSub, error) {
	secureStore, err := boltsecurestore.NewSecureStorage(datadir, "pubsub.db")
	if err != nil {
		return nil, err
	}
	httpClient := esplora.NewHTTPClient(time.Duration(reqTimeout) * time.Second)
	return webhookpubsub.NewWebhookPubSubService(secureStore, httpClient)
}
