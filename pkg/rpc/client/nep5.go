package client

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
)

// TransferTarget represents target address and token amount for transfer.
type TransferTarget struct {
	Token   util.Uint160
	Address util.Uint160
	Amount  int64
}

var (
	// NeoContractHash is a hash of the NEO native contract.
	NeoContractHash, _ = util.Uint160DecodeStringBE("25059ecb4878d3a875f91c51ceded330d4575fde")
	// GasContractHash is a hash of the GAS native contract.
	GasContractHash, _ = util.Uint160DecodeStringBE("bcaf41d684c7d4ad6ee0d99da9707b9d1f0c8e66")
)

// NEP5Decimals invokes `decimals` NEP5 method on a specified contract.
func (c *Client) NEP5Decimals(tokenHash util.Uint160) (int64, error) {
	result, err := c.InvokeFunction(tokenHash, "decimals", []smartcontract.Parameter{}, nil)
	if err != nil {
		return 0, err
	}
	err = getInvocationError(result)
	if err != nil {
		return 0, fmt.Errorf("failed to get NEP5 decimals: %w", err)
	}

	return topIntFromStack(result.Stack)
}

// NEP5Name invokes `name` NEP5 method on a specified contract.
func (c *Client) NEP5Name(tokenHash util.Uint160) (string, error) {
	result, err := c.InvokeFunction(tokenHash, "name", []smartcontract.Parameter{}, nil)
	if err != nil {
		return "", err
	}
	err = getInvocationError(result)
	if err != nil {
		return "", fmt.Errorf("failed to get NEP5 name: %w", err)
	}

	return topStringFromStack(result.Stack)
}

// NEP5Symbol invokes `symbol` NEP5 method on a specified contract.
func (c *Client) NEP5Symbol(tokenHash util.Uint160) (string, error) {
	result, err := c.InvokeFunction(tokenHash, "symbol", []smartcontract.Parameter{}, nil)
	if err != nil {
		return "", err
	}
	err = getInvocationError(result)
	if err != nil {
		return "", fmt.Errorf("failed to get NEP5 symbol: %w", err)
	}

	return topStringFromStack(result.Stack)
}

// NEP5TotalSupply invokes `totalSupply` NEP5 method on a specified contract.
func (c *Client) NEP5TotalSupply(tokenHash util.Uint160) (int64, error) {
	result, err := c.InvokeFunction(tokenHash, "totalSupply", []smartcontract.Parameter{}, nil)
	if err != nil {
		return 0, err
	}
	err = getInvocationError(result)
	if err != nil {
		return 0, fmt.Errorf("failed to get NEP5 total supply: %w", err)
	}

	return topIntFromStack(result.Stack)
}

// NEP5BalanceOf invokes `balanceOf` NEP5 method on a specified contract.
func (c *Client) NEP5BalanceOf(tokenHash, acc util.Uint160) (int64, error) {
	result, err := c.InvokeFunction(tokenHash, "balanceOf", []smartcontract.Parameter{{
		Type:  smartcontract.Hash160Type,
		Value: acc,
	}}, nil)
	if err != nil {
		return 0, err
	}
	err = getInvocationError(result)
	if err != nil {
		return 0, fmt.Errorf("failed to get NEP5 balance: %w", err)
	}

	return topIntFromStack(result.Stack)
}

// NEP5TokenInfo returns full NEP5 token info.
func (c *Client) NEP5TokenInfo(tokenHash util.Uint160) (*wallet.Token, error) {
	name, err := c.NEP5Name(tokenHash)
	if err != nil {
		return nil, err
	}
	symbol, err := c.NEP5Symbol(tokenHash)
	if err != nil {
		return nil, err
	}
	decimals, err := c.NEP5Decimals(tokenHash)
	if err != nil {
		return nil, err
	}
	return wallet.NewToken(tokenHash, name, symbol, decimals), nil
}

// CreateNEP5TransferTx creates an invocation transaction for the 'transfer'
// method of a given contract (token) to move specified amount of NEP5 assets
// (in FixedN format using contract's number of decimals) to given account and
// returns it. The returned transaction is not signed.
func (c *Client) CreateNEP5TransferTx(acc *wallet.Account, to util.Uint160, token util.Uint160, amount int64, gas int64) (*transaction.Transaction, error) {
	return c.CreateNEP5MultiTransferTx(acc, gas, TransferTarget{
		Token:   token,
		Address: to,
		Amount:  amount,
	})
}

// CreateNEP5MultiTransferTx creates an invocation transaction for performing NEP5 transfers
// from a single sender to multiple recipients.
func (c *Client) CreateNEP5MultiTransferTx(acc *wallet.Account, gas int64, recipients ...TransferTarget) (*transaction.Transaction, error) {
	from, err := address.StringToUint160(acc.Address)
	if err != nil {
		return nil, fmt.Errorf("bad account address: %w", err)
	}
	w := io.NewBufBinWriter()
	for i := range recipients {
		emit.AppCallWithOperationAndArgs(w.BinWriter, recipients[i].Token, "transfer", from,
			recipients[i].Address, recipients[i].Amount)
		emit.Opcodes(w.BinWriter, opcode.ASSERT)
	}
	return c.CreateTxFromScript(w.Bytes(), acc, -1, gas, transaction.Signer{
		Account: acc.Contract.ScriptHash(),
		Scopes:  transaction.CalledByEntry,
	})
}

// CreateTxFromScript creates transaction and properly sets cosigners and NetworkFee.
// If sysFee <= 0, it is determined via result of `invokescript` RPC. You should
// initialize network magic with Init before calling CreateTxFromScript.
func (c *Client) CreateTxFromScript(script []byte, acc *wallet.Account, sysFee, netFee int64,
	cosigners ...transaction.Signer) (*transaction.Transaction, error) {
	from, err := address.StringToUint160(acc.Address)
	if err != nil {
		return nil, fmt.Errorf("bad account address: %v", err)
	}

	signers := getSigners(from, cosigners)
	if sysFee < 0 {
		result, err := c.InvokeScript(script, signers)
		if err != nil {
			return nil, fmt.Errorf("can't add system fee to transaction: %w", err)
		}
		if result.State != "HALT" {
			return nil, fmt.Errorf("can't add system fee to transaction: bad vm state: %s due to an error: %s", result.State, result.FaultException)
		}
		sysFee = result.GasConsumed
	}

	if !c.initDone {
		return nil, errNetworkNotInitialized
	}
	tx := transaction.New(c.GetNetwork(), script, sysFee)
	tx.Signers = signers

	tx.ValidUntilBlock, err = c.CalculateValidUntilBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to add validUntilBlock to transaction: %w", err)
	}

	err = c.AddNetworkFee(tx, netFee, acc)
	if err != nil {
		return nil, fmt.Errorf("failed to add network fee: %w", err)
	}

	return tx, nil
}

// TransferNEP5 creates an invocation transaction that invokes 'transfer' method
// on a given token to move specified amount of NEP5 assets (in FixedN format
// using contract's number of decimals) to given account and sends it to the
// network returning just a hash of it.
func (c *Client) TransferNEP5(acc *wallet.Account, to util.Uint160, token util.Uint160, amount int64, gas int64) (util.Uint256, error) {
	tx, err := c.CreateNEP5TransferTx(acc, to, token, amount, gas)
	if err != nil {
		return util.Uint256{}, err
	}

	if err := acc.SignTx(tx); err != nil {
		return util.Uint256{}, fmt.Errorf("can't sign tx: %w", err)
	}

	return c.SendRawTransaction(tx)
}

// MultiTransferNEP5 is similar to TransferNEP5, buf allows to have multiple recipients.
func (c *Client) MultiTransferNEP5(acc *wallet.Account, gas int64, recipients ...TransferTarget) (util.Uint256, error) {
	tx, err := c.CreateNEP5MultiTransferTx(acc, gas, recipients...)
	if err != nil {
		return util.Uint256{}, err
	}

	if err := acc.SignTx(tx); err != nil {
		return util.Uint256{}, fmt.Errorf("can't sign tx: %w", err)
	}

	return c.SendRawTransaction(tx)
}

func topIntFromStack(st []stackitem.Item) (int64, error) {
	index := len(st) - 1 // top stack element is last in the array
	bi, err := st[index].TryInteger()
	if err != nil {
		return 0, err
	}
	return bi.Int64(), nil
}

func topStringFromStack(st []stackitem.Item) (string, error) {
	index := len(st) - 1 // top stack element is last in the array
	bs, err := st[index].TryBytes()
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// getInvocationError returns an error in case of bad VM state or empty stack.
func getInvocationError(result *result.Invoke) error {
	if result.State != "HALT" {
		return fmt.Errorf("invocation failed: %s", result.FaultException)
	}
	if len(result.Stack) == 0 {
		return errors.New("result stack is empty")
	}
	return nil
}
