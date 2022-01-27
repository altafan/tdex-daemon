package ports

import (
	"github.com/tdex-network/tdex-daemon/internal/core/domain"
)

// RepoManager interface defines the methods for swap, price and unspent.
type RepoManager interface {
	MarketRepository() domain.MarketRepository
	TradeRepository() domain.TradeRepository
	DepositRepository() domain.DepositRepository
	WithdrawalRepository() domain.WithdrawalRepository

	Close()
	RegisterHandlerForTradeEvent(
		eventType domain.TradeEventType,
		handler func(event domain.TradeEvent),
	)
	RegisterHandlerForWithdrawalEvent(
		eventType domain.WithdrawalEventType,
		handler func(event domain.WithdrawalEvent),
	)
}
