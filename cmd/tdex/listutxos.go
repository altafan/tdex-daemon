package main

import (
	"context"

	pboperator "github.com/tdex-network/tdex-daemon/api-spec/protobuf/gen/operator"
	"github.com/urfave/cli/v2"
)

var listutxos = cli.Command{
	Name:  "listutxos",
	Usage: "list all utxos for a wallet account",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "account_name",
			Usage:    "the name of the wallet account for which listing utxos",
			Required: true,
		},
		&cli.Uint64Flag{
			Name:  "page",
			Usage: "the number of the page to be listed. If omitted, the entire list is returned",
		},
		&cli.Uint64Flag{
			Name:  "page_size",
			Usage: "the size of the page",
			Value: 10,
		},
	},
	Action: listUtxosAction,
}

func listUtxosAction(ctx *cli.Context) error {
	client, cleanup, err := getOperatorClient(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	accountName := ctx.String("account_name")
	pageNumber := ctx.Int64("page")
	pageSize := ctx.Int64("page_size")
	var page *pboperator.Page
	if pageNumber > 0 {
		page = &pboperator.Page{
			PageNumber: pageNumber,
			PageSize:   pageSize,
		}
	}

	resp, err := client.ListUtxos(
		context.Background(), &pboperator.ListUtxosRequest{
			AccountName: accountName,
			Page:        page,
		},
	)
	if err != nil {
		return err
	}

	printRespJSON(resp)

	return nil
}
