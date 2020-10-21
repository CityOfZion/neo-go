package wallet

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/cli/paramcontext"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/urfave/cli"
)

func newMultisigCommands() []cli.Command {
	signFlags := []cli.Flag{
		walletPathFlag,
		outFlag,
		inFlag,
		cli.StringFlag{
			Name:  "address, a",
			Usage: "Address to use",
		},
	}
	signFlags = append(signFlags, options.RPC...)
	return []cli.Command{
		{
			Name:      "sign",
			Usage:     "sign a transaction",
			UsageText: "multisig sign --wallet <path> --address <address> --in <file.in> --out <file.out>",
			Action:    signMultisig,
			Flags:     signFlags,
		},
	}
}

func signMultisig(ctx *cli.Context) error {
	wall, err := openWallet(ctx.String("wallet"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	defer wall.Close()

	c, err := paramcontext.Read(ctx.String("in"))
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	addr := ctx.String("address")
	sh, err := address.StringToUint160(addr)
	if err != nil {
		return cli.NewExitError(fmt.Errorf("invalid address: %w", err), 1)
	}
	acc, err := getDecryptedAccount(ctx, wall, sh)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	tx, ok := c.Verifiable.(*transaction.Transaction)
	if !ok {
		return cli.NewExitError("verifiable item is not a transaction", 1)
	}

	priv := acc.PrivateKey()
	sign := priv.Sign(tx.GetSignedPart())
	if err := c.AddSignature(acc.Contract, priv.PublicKey(), sign); err != nil {
		return cli.NewExitError(fmt.Errorf("can't add signature: %w", err), 1)
	}
	if out := ctx.String("out"); out != "" {
		if err := paramcontext.Save(c, out); err != nil {
			return cli.NewExitError(err, 1)
		}
	}
	if len(ctx.String(options.RPCEndpointFlag)) != 0 {
		w, err := c.GetWitness(acc.Contract)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		tx.Scripts = append(tx.Scripts, *w)

		gctx, cancel := options.GetTimeoutContext(ctx)
		defer cancel()

		c, err := options.GetRPCClient(gctx, ctx)
		if err != nil {
			return err
		}
		res, err := c.SendRawTransaction(tx)
		if err != nil {
			return cli.NewExitError(err, 1)
		}
		fmt.Fprintln(ctx.App.Writer, res.StringLE())
		return nil
	}

	fmt.Fprintln(ctx.App.Writer, tx.Hash().StringLE())
	return nil
}
