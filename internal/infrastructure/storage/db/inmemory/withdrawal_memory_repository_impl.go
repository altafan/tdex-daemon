package inmemory

import (
	"context"

	"github.com/tdex-network/tdex-daemon/internal/core/domain"
)

type withdrawalRepositoryImpl struct {
	store *withdrawalInmemoryStore
}

// NewWithdrawalRepositoryImpl returns a new empty DepositRepositoryImpl
func NewWithdrawalRepositoryImpl(store *withdrawalInmemoryStore) domain.WithdrawalRepository {
	return &withdrawalRepositoryImpl{store}
}

func (w withdrawalRepositoryImpl) AddWithdrawals(
	_ context.Context,
	withdrawals []domain.Withdrawal,
) (int, error) {
	w.store.locker.Lock()
	defer w.store.locker.Unlock()

	count := 0
	for _, withdrawal := range withdrawals {
		if _, ok := w.store.withdrawals[withdrawal.TxID]; !ok {
			w.store.withdrawals[withdrawal.TxID] = withdrawal
			count++
		}
	}
	return count, nil
}

func (w withdrawalRepositoryImpl) ListWithdrawalsForAccount(
	_ context.Context, accountName string,
) ([]domain.Withdrawal, error) {
	w.store.locker.RLock()
	defer w.store.locker.RUnlock()

	result := make([]domain.Withdrawal, 0)
	for _, v := range w.store.withdrawals {
		if v.AccountName == accountName {
			result = append(result, v)
		}
	}

	return result, nil
}
func (w withdrawalRepositoryImpl) ListWithdrawalsForAccountAndPage(
	_ context.Context, accountName string, page domain.Page,
) ([]domain.Withdrawal, error) {
	w.store.locker.RLock()
	defer w.store.locker.RUnlock()

	result := make([]domain.Withdrawal, 0)
	startIndex := page.Number*page.Size - page.Size + 1
	endIndex := page.Number * page.Size
	index := 1
	for _, v := range w.store.withdrawals {
		if v.AccountName == accountName {
			if index >= startIndex && index <= endIndex {
				result = append(result, v)
			}
			index++
		}
	}

	return result, nil
}

func (w withdrawalRepositoryImpl) ListAllWithdrawals(
	_ context.Context,
) ([]domain.Withdrawal, error) {
	withdrawals := make([]domain.Withdrawal, 0, len(w.store.withdrawals))
	for _, v := range w.store.withdrawals {
		withdrawals = append(withdrawals, v)
	}
	return withdrawals, nil
}

func (w withdrawalRepositoryImpl) ListAllWithdrawalsForPage(
	_ context.Context, page domain.Page,
) ([]domain.Withdrawal, error) {
	withdrawals := make([]domain.Withdrawal, 0)
	startIndex := page.Number*page.Size - page.Size + 1
	endIndex := page.Number * page.Size
	index := 1
	for _, v := range w.store.withdrawals {
		if index >= startIndex && index <= endIndex {
			withdrawals = append(withdrawals, v)
		}
		index++
	}
	return withdrawals, nil
}

func (w withdrawalRepositoryImpl) EventChannel() chan domain.WithdrawalEvent {
	return nil
}
