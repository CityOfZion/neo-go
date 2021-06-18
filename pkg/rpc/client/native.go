package client

// Various non-policy things from native contracts.

import (
	"errors"
	"fmt"

	nns "github.com/nspcc-dev/neo-go/examples/nft-nd-nns"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// GetOraclePrice invokes `getPrice` method on a native Oracle contract.
func (c *Client) GetOraclePrice() (int64, error) {
	oracleHash, err := c.GetNativeContractHash(nativenames.Notary)
	if err != nil {
		return 0, fmt.Errorf("failed to get native Oracle hash: %w", err)
	}
	return c.invokeNativeGetMethod(oracleHash, "getPrice")
}

// GetNNSPrice invokes `getPrice` method on a NeoNameService contract with the specified hash.
func (c *Client) GetNNSPrice(nnsHash util.Uint160) (int64, error) {
	return c.invokeNativeGetMethod(nnsHash, "getPrice")
}

// GetGasPerBlock invokes `getGasPerBlock` method on a native NEO contract.
func (c *Client) GetGasPerBlock() (int64, error) {
	neoHash, err := c.GetNativeContractHash(nativenames.Neo)
	if err != nil {
		return 0, fmt.Errorf("failed to get native NEO hash: %w", err)
	}
	return c.invokeNativeGetMethod(neoHash, "getGasPerBlock")
}

// GetDesignatedByRole invokes `getDesignatedByRole` method on a native RoleManagement contract.
func (c *Client) GetDesignatedByRole(role noderoles.Role, index uint32) (keys.PublicKeys, error) {
	rmHash, err := c.GetNativeContractHash(nativenames.Designation)
	if err != nil {
		return nil, fmt.Errorf("failed to get native RoleManagement hash: %w", err)
	}
	result, err := c.InvokeFunction(rmHash, "getDesignatedByRole", []smartcontract.Parameter{
		{
			Type:  smartcontract.IntegerType,
			Value: int64(role),
		},
		{
			Type:  smartcontract.IntegerType,
			Value: int64(index),
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	err = GetInvocationError(result)
	if err != nil {
		return nil, fmt.Errorf("`getDesignatedByRole`: %w", err)
	}
	return topPublicKeysFromStack(result.Stack)
}

// NNSResolve invokes `resolve` method on a NameService contract with the specified hash.
func (c *Client) NNSResolve(nnsHash util.Uint160, name string, typ nns.RecordType) (string, error) {
	if typ == nns.CNAME {
		return "", errors.New("can't resolve CNAME record type")
	}
	result, err := c.InvokeFunction(nnsHash, "resolve", []smartcontract.Parameter{
		{
			Type:  smartcontract.StringType,
			Value: name,
		},
		{
			Type:  smartcontract.IntegerType,
			Value: int64(typ),
		},
	}, nil)
	if err != nil {
		return "", err
	}
	err = GetInvocationError(result)
	if err != nil {
		return "", fmt.Errorf("`resolve`: %w", err)
	}
	return topStringFromStack(result.Stack)
}

// NNSIsAvailable invokes `isAvailable` method on a NeoNameService contract with the specified hash.
func (c *Client) NNSIsAvailable(nnsHash util.Uint160, name string) (bool, error) {
	result, err := c.InvokeFunction(nnsHash, "isAvailable", []smartcontract.Parameter{
		{
			Type:  smartcontract.StringType,
			Value: name,
		},
	}, nil)
	if err != nil {
		return false, err
	}
	err = GetInvocationError(result)
	if err != nil {
		return false, fmt.Errorf("`isAvailable`: %w", err)
	}
	return TopBoolFromStack(result.Stack)
}
