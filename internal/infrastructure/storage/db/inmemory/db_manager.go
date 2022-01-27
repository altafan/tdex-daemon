package inmemory

import (
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
)

type marketInmemoryStore struct {
	markets             map[uint64]domain.Market
	accountsByAssetsKey map[string]uint64
	accountsByName      map[string]uint64
	locker              *sync.Mutex
}

type tradeInmemoryStore struct {
	trades               map[uuid.UUID]domain.Trade
	tradesBySwapAcceptID map[string]uuid.UUID
	tradesByMarket       map[string][]uuid.UUID
	locker               *sync.Mutex
}

type depositInmemoryStore struct {
	deposits map[domain.DepositKey]domain.Deposit
	locker   *sync.RWMutex
}

type withdrawalInmemoryStore struct {
	withdrawals map[string]domain.Withdrawal
	locker      *sync.RWMutex
}

type RepoManager struct {
	marketStore     *marketInmemoryStore
	tradeStore      *tradeInmemoryStore
	depositStore    *depositInmemoryStore
	withdrawalStore *withdrawalInmemoryStore

	marketRepository      domain.MarketRepository
	tradeRepository       domain.TradeRepository
	depositRepository     domain.DepositRepository
	withdrawalsRepository domain.WithdrawalRepository
}

type InmemoryTx struct {
	db      *RepoManager
	success bool
}

func (tx *InmemoryTx) Commit() error {
	if tx.db == nil {
		return errors.New("the transaction has no associated database.")
	}
	tx.success = true
	return nil
}

func (tx *InmemoryTx) Discard() {
	tx.success = false
}

func NewRepoManager() ports.RepoManager {
	marketStore := &marketInmemoryStore{
		markets:             map[uint64]domain.Market{},
		accountsByAssetsKey: map[string]uint64{},
		accountsByName:      map[string]uint64{},
		locker:              &sync.Mutex{},
	}
	tradeStore := &tradeInmemoryStore{
		trades:               map[uuid.UUID]domain.Trade{},
		tradesBySwapAcceptID: map[string]uuid.UUID{},
		tradesByMarket:       map[string][]uuid.UUID{},
		locker:               &sync.Mutex{},
	}
	depositStore := &depositInmemoryStore{
		deposits: map[domain.DepositKey]domain.Deposit{},
		locker:   &sync.RWMutex{},
	}
	withdrawalStore := &withdrawalInmemoryStore{
		withdrawals: map[string]domain.Withdrawal{},
		locker:      &sync.RWMutex{},
	}

	marketRepo := NewMarketRepositoryImpl(marketStore)
	tradeRepo := NewTradeRepositoryImpl(tradeStore)
	depositRepo := NewDepositRepositoryImpl(depositStore)
	withdrawalRepo := NewWithdrawalRepositoryImpl(withdrawalStore)

	return &RepoManager{
		marketStore:           marketStore,
		tradeStore:            tradeStore,
		depositStore:          depositStore,
		withdrawalStore:       withdrawalStore,
		marketRepository:      marketRepo,
		tradeRepository:       tradeRepo,
		depositRepository:     depositRepo,
		withdrawalsRepository: withdrawalRepo,
	}
}

func (d *RepoManager) MarketRepository() domain.MarketRepository {
	return d.marketRepository
}

func (d *RepoManager) TradeRepository() domain.TradeRepository {
	return d.tradeRepository
}

func (d *RepoManager) DepositRepository() domain.DepositRepository {
	return d.depositRepository
}

func (d *RepoManager) WithdrawalRepository() domain.WithdrawalRepository {
	return d.withdrawalsRepository
}

func (d *RepoManager) Close() {}

func (db *RepoManager) RegisterHandlerForTradeEvent(
	eventType domain.TradeEventType,
	handler func(event domain.TradeEvent),
) {
}

func (db *RepoManager) RegisterHandlerForWithdrawalEvent(
	eventType domain.WithdrawalEventType,
	handler func(event domain.WithdrawalEvent),
) {
}
