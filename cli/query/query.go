package query

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/nspcc-dev/neo-go/cli/options"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/urfave/cli"
)

// NewCommands returns 'query' command.
func NewCommands() []cli.Command {
	queryTxFlags := append([]cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, v",
			Usage: "Output full tx info and execution logs",
		},
	}, options.RPC...)
	return []cli.Command{{
		Name:  "query",
		Usage: "query",
		Subcommands: []cli.Command{
			{
				Name:   "tx",
				Usage:  "query tx status",
				Action: queryTx,
				Flags:  queryTxFlags,
			},
		},
	}}
}

func queryTx(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) == 0 {
		return cli.NewExitError("Transaction hash is missing", 1)
	}

	txHash, err := util.Uint256DecodeStringLE(strings.TrimPrefix(args[0], "0x"))
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Invalid tx hash: %s", args[0]), 1)
	}

	gctx, cancel := options.GetTimeoutContext(ctx)
	defer cancel()

	c, err := options.GetRPCClient(gctx, ctx)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	tx, err := c.GetRawTransaction(txHash)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	res, err := c.GetApplicationLog(txHash, nil)
	if err != nil && strings.Index(err.Error(), "Unknown transaction") < 0 {
		return cli.NewExitError(err, 1)
	}

	dumpApplicationLog(ctx, res, tx)
	return nil
}

func dumpApplicationLog(ctx *cli.Context, res *result.ApplicationLog, tx *transaction.Transaction) {
	verbose := ctx.Bool("verbose")
	buf := bytes.NewBuffer(nil)
	tw := tabwriter.NewWriter(buf, 0, 4, 4, '\t', 0)
	tw.Write([]byte("Hash:\t" + tx.Hash().StringLE() + "\n"))
	if res == nil {
		tw.Write([]byte("Persisted:\tfalse\n"))
	} else {
		tw.Write([]byte("Persisted:\ttrue\n"))
	}
	if verbose {
		for _, sig := range tx.Signers {
			tw.Write([]byte(fmt.Sprintf("Signer:\t%s (%s)",
				sig.Account.StringLE(),
				sig.Scopes) + "\n"))
		}
		tw.Write([]byte("SystemFee:\t" + fixedn.Fixed8(tx.SystemFee).String() + " GAS\n"))
		tw.Write([]byte("NetworkFee:\t" + fixedn.Fixed8(tx.NetworkFee).String() + " GAS\n"))
		tw.Write([]byte("Nonce:\t" + strconv.FormatUint(uint64(tx.Nonce), 16) + "\n"))
		tw.Write([]byte("ValidUntil:\t" + strconv.FormatUint(uint64(tx.ValidUntilBlock), 10) + "\n"))
		tw.Write([]byte("Script:\t" + base64.StdEncoding.EncodeToString(tx.Script) + "\n"))
		if res != nil {
			for _, e := range res.Executions {
				tw.Write([]byte("VMState:\t" + e.VMState.String() + "\n"))
				if e.VMState != vm.HaltState {
					tw.Write([]byte("Exception:\t" + e.FaultException + "\n"))
				}
			}
		}
	}
	_ = tw.Flush()
	fmt.Fprint(ctx.App.Writer, buf.String())
}
