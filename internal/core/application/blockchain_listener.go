package application

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/tdex-network/tdex-daemon/config"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	"github.com/tdex-network/tdex-daemon/pkg/crawler"
	"github.com/tdex-network/tdex-daemon/pkg/explorer"
)

type BlockchainListener interface {
	ObserveBlockchain()
}

type blockchainListener struct {
	unspentRepository domain.UnspentRepository
	marketRepository  domain.MarketRepository
	vaultRepository   domain.VaultRepository
	crawlerSvc        crawler.Service
	explorerSvc       explorer.Service
	dbManager         ports.DbManager
}

func NewBlockchainListener(
	unspentRepository domain.UnspentRepository,
	marketRepository domain.MarketRepository,
	vaultRepository domain.VaultRepository,
	crawlerSvc crawler.Service,
	explorerSvc explorer.Service,
	dbManager ports.DbManager,
) BlockchainListener {
	return &blockchainListener{
		unspentRepository: unspentRepository,
		marketRepository:  marketRepository,
		vaultRepository:   vaultRepository,
		crawlerSvc:        crawlerSvc,
		explorerSvc:       explorerSvc,
		dbManager:         dbManager,
	}
}

func (b *blockchainListener) ObserveBlockchain() {
	go b.crawlerSvc.Start()
	go b.handleBlockChainEvents()
}

func (b *blockchainListener) handleBlockChainEvents() {

events:
	for event := range b.crawlerSvc.GetEventChannel() {
		tx := b.startTx()
		ctx := context.WithValue(context.Background(), "tx", tx)
		switch event.Type() {
		case crawler.FeeAccountDeposit:
			e := event.(crawler.AddressEvent)
			unspents := make([]domain.Unspent, 0)
			if len(e.Utxos) > 0 {

			utxoLoop:
				for _, utxo := range e.Utxos {
					isTrxConfirmed, err := b.explorerSvc.IsTransactionConfirmed(
						utxo.Hash(),
					)
					if err != nil {
						tx.Discard()
						log.Warn(err)
						continue utxoLoop
					}
					u := domain.NewUnspent(
						utxo.Hash(),
						utxo.Asset(),
						e.Address,
						utxo.Index(),
						utxo.Value(),
						false,
						false,
						nil, //TODO should this be populated
						nil,
						isTrxConfirmed,
					)
					unspents = append(unspents, u)
				}
				err := b.unspentRepository.AddUnspents(
					ctx,
					unspents,
				)
				if err != nil {
					tx.Discard()
					log.Warn(err)
					continue events
				}

				markets, err := b.marketRepository.GetTradableMarkets(
					ctx,
				)
				if err != nil {
					tx.Discard()
					log.Warn(err)
					continue events
				}

				addresses, _, err := b.vaultRepository.
					GetAllDerivedAddressesAndBlindingKeysForAccount(
						ctx,
						domain.FeeAccount,
					)
				if err != nil {
					tx.Discard()
					log.Warn(err)
					continue events
				}

				var feeAccountBalance uint64
				for _, a := range addresses {
					feeAccountBalance += b.unspentRepository.GetBalance(
						ctx,
						a,
						config.GetString(config.BaseAssetKey),
					)
				}

				if feeAccountBalance < uint64(
					config.GetInt(config.FeeAccountBalanceThresholdKey),
				) {
					log.Debug("fee account balance too low - Trades and" +
						" deposits will be disabled")
					for _, m := range markets {
						err := b.marketRepository.CloseMarket(
							ctx,
							m.QuoteAssetHash(),
						)
						if err != nil {
							tx.Discard()
							log.Warn(err)
							continue events
						}
					}
					continue events
				}

				for _, m := range markets {
					err := b.marketRepository.OpenMarket(
						ctx,
						m.QuoteAssetHash(),
					)
					if err != nil {
						tx.Discard()
						log.Warn(err)
						continue events
					}
					log.Debug(fmt.Sprintf(
						"market %v, opened",
						m.AccountIndex(),
					))
				}
			}

		case crawler.MarketAccountDeposit:
			e := event.(crawler.AddressEvent)
			unspents := make([]domain.Unspent, 0)
			if len(e.Utxos) > 0 {
			utxoLoop1:
				for _, utxo := range e.Utxos {
					isTrxConfirmed, err := b.explorerSvc.IsTransactionConfirmed(
						utxo.Hash(),
					)
					if err != nil {
						tx.Discard()
						log.Warn(err)
						continue utxoLoop1
					}
					u := domain.NewUnspent(
						utxo.Hash(),
						utxo.Asset(),
						e.Address,
						utxo.Index(),
						utxo.Value(),
						false,
						false,
						nil, //TODO should this be populated
						nil,
						isTrxConfirmed,
					)
					unspents = append(unspents, u)
				}
				err := b.unspentRepository.AddUnspents(
					ctx,
					unspents,
				)
				if err != nil {
					tx.Discard()
					log.Warn(err)
					continue events
				}

				fundingTxs := make([]domain.OutpointWithAsset, 0)
				for _, u := range e.Utxos {
					tx := domain.OutpointWithAsset{
						Asset: u.Asset(),
						Txid:  u.Hash(),
						Vout:  int(u.Index()),
					}
					fundingTxs = append(fundingTxs, tx)
				}

				m, err := b.marketRepository.GetOrCreateMarket(
					ctx,
					e.AccountIndex,
				)
				if err != nil {
					tx.Discard()
					log.Error(err)
					continue events
				}

				if err := b.marketRepository.UpdateMarket(
					ctx,
					m.AccountIndex(),
					func(m *domain.Market) (*domain.Market, error) {

						if m.IsFunded() {
							return m, nil
						}

						if err := m.FundMarket(fundingTxs); err != nil {
							tx.Discard()
							return nil, err
						}

						log.Info("deposit: funding market with quote asset ", m.QuoteAssetHash())

						return m, nil
					}); err != nil {
					tx.Discard()
					log.Warn(err)
					continue events
				}

			}

		case crawler.TransactionConfirmed:
			//TODO
		}
		b.commitTx(tx)
	}
}

func (b *blockchainListener) startTx() ports.Transaction {
	return b.dbManager.NewTransaction()
}

func (b *blockchainListener) commitTx(tx ports.Transaction) {
	err := tx.Commit()
	if err != nil {
		log.Error(err)
	}
}