package wallet

import (
	"context"

	"github.com/tdex-network/tdex-daemon/internal/core/ports"
	oceanv1alpha "github.com/vulpemventures/ocean/api-spec/protobuf/gen/go/ocean/v1alpha"
	"google.golang.org/grpc"
)

type notificationsManagerGrpc struct {
	client oceanv1alpha.NotificationServiceClient
}

var _ ports.NotificationManager = (*notificationsManagerGrpc)(nil)

func newNotificationsManagerGrpc(conn *grpc.ClientConn) ports.NotificationManager {
	return &notificationsManagerGrpc{
		client: oceanv1alpha.NewNotificationServiceClient(conn),
	}
}

func (nm *notificationsManagerGrpc) TxChannel() (chan ports.TxNotification, error) {
	req := &oceanv1alpha.TransactionNotificationsRequest{}
	stream, err := nm.client.TransactionNotifications(context.Background(), req)
	if err != nil {
		return nil, err
	}

	txChan := make(chan ports.TxNotification)

	go func() {
		for {
			resp, err := stream.Recv()
			if err != nil {
				return
			}

			txChan <- &txNotificationGrpc{resp}
		}
	}()

	return txChan, nil
}

func (nm *notificationsManagerGrpc) UtxoChannel() (chan ports.UtxoNotification, error) {
	req := &oceanv1alpha.UtxosNotificationsRequest{}
	stream, err := nm.client.UtxosNotifications(context.Background(), req)
	if err != nil {
		return nil, err
	}

	utxoChan := make(chan ports.UtxoNotification)

	go func() {
		for {
			resp, err := stream.Recv()
			if err != nil {
				return
			}

			utxoChan <- &utxoNotificationGrpc{resp}
		}
	}()

	return utxoChan, nil
}
