package wallet

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/CityOfZion/neo-go/pkg/wallet"
	"github.com/urfave/cli"
)

var (
	errNoPath          = errors.New("Target path where the wallet should be stored is mandatory and should be passed using (--path, -p) flags.")
	errPathMandatory   = errors.New("While opening wallet (--path) is mandatory and must be passed as a flag.")
	errNoPass          = errors.New("Passphrase is mandatory while trying to open a wallet using (--pass) flag..")
	errPhraseMissmatch = errors.New("The entered passphrases do not match. Maybe you have misspelled them?")
	errFileNotFound    = errors.New("File specified in --path doesn't exist")
)

// NewComand creates a new Wallet command.
func NewCommand() cli.Command {
	return cli.Command{
		Name:  "wallet",
		Usage: "create, open and manage a NEO wallet",
		Subcommands: []cli.Command{
			{
				Name:   "create",
				Usage:  "create a new wallet",
				Action: createWallet,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "path, p",
						Usage: "Target location of the wallet file.",
					},
					cli.BoolFlag{
						Name:  "account, a",
						Usage: "Create a new account",
					},
				},
			},
			{
				Name:   "open",
				Usage:  "open a existing NEO wallet",
				Action: openWallet,
				Flags: []cli.Flag{
					cli.StringFlag{
						Name:  "path, p",
						Usage: "Target location of the wallet file.",
					},
					cli.StringFlag{
						Name:  "passphrase, pass",
						Usage: "Passphrase to unlock an account in the wallet.",
					},
				},
			},
		},
	}
}

func openWallet(ctx *cli.Context) error {
	path := ctx.String("path")
	if len(path) == 0 {
		return cli.NewExitError(errPathMandatory, 1)
	}

	pass := ctx.String("pass")
	if len(pass) == 0 {
		return cli.NewExitError(errNoPass, 1)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cli.NewExitError(errFileNotFound, 1)
	}

	w, err := wallet.NewWalletFromFile(path)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	if err := w.Unlock(pass); err != nil {
		return cli.NewExitError(err, 1)
	}

	fmt.Println("Wallet opened successfully.")
	return nil
}

func createWallet(ctx *cli.Context) error {
	path := ctx.String("path")
	if len(path) == 0 {
		return cli.NewExitError(errNoPath, 1)
	}
	wall, err := wallet.NewWallet(path)
	if err != nil {
		return cli.NewExitError(err, 1)
	}
	if err := wall.Save(); err != nil {
		return cli.NewExitError(err, 1)
	}

	if ctx.Bool("account") {
		if err := createAccount(ctx, wall); err != nil {
			return cli.NewExitError(err, 1)
		}
	}

	dumpWallet(wall)
	fmt.Printf("wallet succesfully created, file location is %s\n", wall.Path())
	return nil
}

func createAccount(ctx *cli.Context, wall *wallet.Wallet) error {
	var (
		rawName,
		rawPhrase,
		rawPhraseCheck []byte
	)
	buf := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the name of the account > ")
	rawName, _ = buf.ReadBytes('\n')
	fmt.Print("Enter passphrase > ")
	rawPhrase, _ = buf.ReadBytes('\n')
	fmt.Print("Confirm passphrase > ")
	rawPhraseCheck, _ = buf.ReadBytes('\n')

	// Clean data
	var (
		name        = strings.TrimRight(string(rawName), "\n")
		phrase      = strings.TrimRight(string(rawPhrase), "\n")
		phraseCheck = strings.TrimRight(string(rawPhraseCheck), "\n")
	)

	if phrase != phraseCheck {
		return errPhraseMissmatch
	}

	return wall.CreateAccount(name, phrase)
}

func dumpWallet(wall *wallet.Wallet) {
	b, _ := wall.JSON()
	fmt.Println("")
	fmt.Println(string(b))
	fmt.Println("")
}
