package wallet

import (
	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
)

type walletStatusGrpc struct {
	resp *oceanv1alpha.StatusResponse
}

var _ ports.WalletStatus = (*walletStatusGrpc)(nil)

func (s *walletStatusGrpc) IsUnlocked() bool {
	return s.resp.Unlocked
}

func (s *walletStatusGrpc) IsSynced() bool {
	return s.resp.Synced
}

func (s *walletStatusGrpc) IsInitialized() bool {
	return s.resp.Initialized
}
