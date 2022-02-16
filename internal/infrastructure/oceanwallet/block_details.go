package oceanwallet

import (
	"encoding/hex"

	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
)

type blockDetailsGrpc struct {
	details *oceanv1alpha.BlockDetails
}

func (b *blockDetailsGrpc) Hash() string {
	return hex.EncodeToString(b.details.GetHash())
}

func (b *blockDetailsGrpc) Height() uint32 {
	return uint32(b.details.GetHeight())
}

func (b *blockDetailsGrpc) Timestamp() int64 {
	return b.details.GetTimestamp()
}
