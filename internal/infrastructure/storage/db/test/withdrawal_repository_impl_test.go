package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tdex-network/tdex-daemon/internal/infrastructure/storage/db/inmemory"

	"github.com/stretchr/testify/require"
	"github.com/tdex-network/tdex-daemon/internal/core/ports"

	"github.com/tdex-network/tdex-daemon/internal/core/domain"

	dbbadger "github.com/tdex-network/tdex-daemon/internal/infrastructure/storage/db/badger"
)

func TestWithdrawalRepositoryImplementations(t *testing.T) {
	repositories := createWithdrawalRepositories(t)

	for i := range repositories {
		repo := repositories[i]

		t.Run(repo.Name, func(t *testing.T) {
			t.Run("testAddAndListWithdrawals", func(t *testing.T) {
				testAddAndListWithdrawals(t, repo)
			})
		})
	}
}

func testAddAndListWithdrawals(t *testing.T, repo withdrawalRepository) {
	withdrawalRepo := repo.Repository
	withdrawals := make([]domain.Withdrawal, 0)
	for i := 0; i < 10; i++ {
		withdrawals = append(withdrawals, domain.Withdrawal{
			TxID:        fmt.Sprintf("%d", i),
			AccountName: "aaa",
			Outputs: []domain.Output{
				{
					Asset:   "aaa",
					Value:   100,
					Address: "xyz",
				},
				{
					Asset:   "bbb",
					Value:   3000,
					Address: "zyx",
				},
			},
			MillisatPerByte: 10,
			TotAmountPerAsset: map[string]uint64{
				"aaa": 100,
				"bbb": 3000,
			},
			Timestamp: uint64(time.Now().Unix()),
		})
	}

	count, err := withdrawalRepo.AddWithdrawals(
		context.Background(), withdrawals,
	)
	require.NoError(t, err)
	require.Equal(t, 10, count)

	count, err = withdrawalRepo.AddWithdrawals(
		context.Background(), []domain.Withdrawal{
			{
				TxID:        "0",
				AccountName: "aaa",
				Outputs: []domain.Output{
					{
						Asset:   "aaa",
						Value:   200,
						Address: "foo",
					},
					{
						Asset:   "bbb",
						Value:   6000,
						Address: "bar",
					},
				},
				MillisatPerByte: 10,
				TotAmountPerAsset: map[string]uint64{
					"aaa": 200,
					"bbb": 6000,
				},
				Timestamp: uint64(time.Now().Unix()),
			},
		},
	)
	require.NoError(t, err)
	require.Zero(t, count)

	withdrawals, err = withdrawalRepo.ListWithdrawalsForAccount(context.Background(), "aaa")
	require.NoError(t, err)
	require.Len(t, withdrawals, 10)

	withdrawals, err = withdrawalRepo.ListWithdrawalsForAccount(context.Background(), "bbb")
	require.NoError(t, err)
	require.Empty(t, withdrawals)

	withdrawals, err = withdrawalRepo.ListWithdrawalsForAccountAndPage(
		context.Background(), "aaa", domain.Page{Number: 1, Size: 5},
	)
	require.NoError(t, err)
	require.Len(t, withdrawals, 5)

	withdrawals, err = withdrawalRepo.ListWithdrawalsForAccountAndPage(
		context.Background(), "aaa", domain.Page{Number: 2, Size: 5},
	)
	require.NoError(t, err)
	require.Len(t, withdrawals, 5)

	withdrawals, err = withdrawalRepo.ListAllWithdrawals(context.Background())
	require.NoError(t, err)
	require.Len(t, withdrawals, 10)

	withdrawals, err = withdrawalRepo.ListAllWithdrawalsForPage(
		context.Background(), domain.Page{Number: 1, Size: 6},
	)
	require.NoError(t, err)
	require.Len(t, withdrawals, 6)

	withdrawals, err = withdrawalRepo.ListAllWithdrawalsForPage(
		context.Background(), domain.Page{Number: 2, Size: 6},
	)
	require.NoError(t, err)
	require.Len(t, withdrawals, 4)
}

type withdrawalRepository struct {
	Name       string
	DBManager  ports.RepoManager
	Repository domain.WithdrawalRepository
}

func createWithdrawalRepositories(t *testing.T) []withdrawalRepository {
	inmemoryDBManager := inmemory.NewRepoManager()
	badgerDBManager, err := dbbadger.NewRepoManager("", nil)
	require.NoError(t, err)

	badgerDBManager.RegisterHandlerForWithdrawalEvent(
		domain.NewWithdrawalEvent, func(event domain.WithdrawalEvent) {
			require.Equal(t, event.EventType, domain.NewWithdrawalEvent)
			require.NotEmpty(t, event.Withdrawal)
		},
	)

	return []withdrawalRepository{
		{
			Name:       "badger",
			DBManager:  badgerDBManager,
			Repository: badgerDBManager.WithdrawalRepository(),
		},
		{
			Name:       "inmemory",
			DBManager:  inmemoryDBManager,
			Repository: inmemoryDBManager.WithdrawalRepository(),
		},
	}
}
