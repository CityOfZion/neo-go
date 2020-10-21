package main

import (
	"encoding/hex"
	"math/big"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/stretchr/testify/require"
)

// Test signing of multisig transactions.
// 1. Transfer funds to a created multisig address.
// 2. Transfer from multisig to another account.
func TestSignMultisigTx(t *testing.T) {
	e := newExecutor(t, true)
	defer e.Close(t)

	privs, pubs := generateKeys(t, 3)
	script, err := smartcontract.CreateMultiSigRedeemScript(2, pubs)
	require.NoError(t, err)
	multisigHash := hash.Hash160(script)
	multisigAddr := address.Uint160ToString(multisigHash)

	// Create 2 wallets participating in multisig.
	tmpDir := os.TempDir()
	wallet1Path := path.Join(tmpDir, "multiWallet1.json")
	defer os.Remove(wallet1Path)
	wallet2Path := path.Join(tmpDir, "multiWallet2.json")
	defer os.Remove(wallet2Path)

	addAccount := func(w string, wif string) {
		e.In.WriteString("acc\rpass\rpass\r")
		e.Run(t, "neo-go", "wallet", "init", "--wallet", w)
		e.Run(t, "neo-go", "wallet", "import-multisig",
			"--wallet", w,
			"--wif", wif,
			"--min", "2",
			hex.EncodeToString(pubs[0].Bytes()),
			hex.EncodeToString(pubs[1].Bytes()),
			hex.EncodeToString(pubs[2].Bytes()))
	}
	addAccount(wallet1Path, privs[0].WIF())
	addAccount(wallet2Path, privs[1].WIF())

	// Transfer funds to the multisig.
	e.In.WriteString("one\r")
	e.Run(t, "neo-go", "wallet", "nep5", "multitransfer",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", validatorWallet,
		"--from", validatorAddr,
		"neo:"+multisigAddr+":4",
		"gas:"+multisigAddr+":1")
	e.checkTxPersisted(t)

	// Sign and transfer funds to another account.
	priv, err := keys.NewPrivateKey()
	require.NoError(t, err)

	txPath := path.Join(tmpDir, "multisigtx.json")
	defer os.Remove(txPath)
	e.In.WriteString("pass\r")
	e.Run(t, "neo-go", "wallet", "nep5", "transfer",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", wallet1Path, "--from", multisigAddr,
		"--to", priv.Address(), "--token", "neo", "--amount", "1",
		"--out", txPath)

	e.In.WriteString("pass\r")
	e.Run(t, "neo-go", "wallet", "multisig", "sign",
		"--rpc-endpoint", "http://"+e.RPC.Addr,
		"--wallet", wallet2Path, "--address", multisigAddr,
		"--in", txPath, "--out", txPath)
	e.checkTxPersisted(t)

	b, _ := e.Chain.GetGoverningTokenBalance(priv.GetScriptHash())
	require.Equal(t, big.NewInt(1), b)
	b, _ = e.Chain.GetGoverningTokenBalance(multisigHash)
	require.Equal(t, big.NewInt(3), b)

	t.Run("via invokefunction", func(t *testing.T) {
		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "contract", "invokefunction",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", wallet1Path, "--address", multisigAddr,
			"--out", txPath,
			client.NeoContractHash.StringLE(), "transfer",
			"bytes:"+multisigHash.StringBE(),
			"bytes:"+priv.GetScriptHash().StringBE(),
			"int:1",
			"--", strings.Join([]string{multisigHash.StringLE(), ":", "Global"}, ""))

		e.In.WriteString("pass\r")
		e.Run(t, "neo-go", "wallet", "multisig", "sign",
			"--rpc-endpoint", "http://"+e.RPC.Addr,
			"--wallet", wallet2Path, "--address", multisigAddr,
			"--in", txPath, "--out", txPath)
		e.checkTxPersisted(t)

		b, _ := e.Chain.GetGoverningTokenBalance(priv.GetScriptHash())
		require.Equal(t, big.NewInt(2), b)
		b, _ = e.Chain.GetGoverningTokenBalance(multisigHash)
		require.Equal(t, big.NewInt(2), b)
	})
}
