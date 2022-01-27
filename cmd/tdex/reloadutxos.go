package main

import (
	"github.com/urfave/cli/v2"
)

var reloadtxos = cli.Command{
	Name:   "reloadutxos",
	Usage:  "reload all utxos",
	Action: reloadUtxos,
}

func reloadUtxos(ctx *cli.Context) error {
	printDeprecatedWarn("")

	return nil
}
