package domain

import "context"

type WithdrawalEventType int

const (
	NewWithdrawalEvent WithdrawalEventType = iota
)

type WithdrawalEvent struct {
	EventType  WithdrawalEventType
	Withdrawal Withdrawal
}

// WithdrawalRepository is the abstraction to which all concrete implementations
// must sitck with to persist withdrawals.
type WithdrawalRepository interface {
	// AddWithdrawals adds the provided withdrawals to the repository. Those already
	// existing won't be re-added.
	AddWithdrawals(ctx context.Context, withdrawals []Withdrawal) (int, error)
	// ListWithdrawalsForAccount returns the list with the withdrawals related to
	// the given wallet account id.
	ListWithdrawalsForAccount(
		ctx context.Context, accountName string,
	) ([]Withdrawal, error)
	// ListWithdrawalsForAccountAndPage returns a page containing a subset of the
	// list with the withdrawals related to the given wallet account id.
	ListWithdrawalsForAccountAndPage(
		ctx context.Context, accountName string, page Page,
	) ([]Withdrawal, error)
	// ListAllWithdrawals returns all withdrawals related to all wallet accounts
	// stored in the repository.
	ListAllWithdrawals(ctx context.Context) ([]Withdrawal, error)
	// ListAllWithdrawalsForPage returns a page containing a subset of all
	// withdrawals related to all wallet accounts stored in the repository.
	ListAllWithdrawalsForPage(ctx context.Context, page Page) ([]Withdrawal, error)
	// EventChannel returns the channel to receive info about relevant events
	// happening within the repository.
	EventChannel() chan WithdrawalEvent
}
