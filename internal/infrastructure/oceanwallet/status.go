package oceanwallet

import (
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
)

type walletStatusGrpc struct {
	resp *oceanv1alpha.StatusResponse
}

func (s *walletStatusGrpc) IsUnlocked() bool {
	return s.resp.GetUnlocked()
}

func (s *walletStatusGrpc) IsSynced() bool {
	return s.resp.GetSynced()
}

func (s *walletStatusGrpc) IsInitialized() bool {
	return s.resp.GetInitialized()
}
