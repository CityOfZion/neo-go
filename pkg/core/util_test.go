package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenesisBlockMainNet(t *testing.T) {
	cfg, err := config.Load("../../config", netmode.MainNet)
	require.NoError(t, err)

	block, err := createGenesisBlock(cfg.ProtocolConfiguration)
	require.NoError(t, err)

	expect := "ecaee33262f1bc7c7c28f2b25b54a5d61d50670871f45c0c6fe755a40cbde4a8"
	assert.Equal(t, expect, block.Hash().StringLE())
}

func TestGetConsensusAddressMainNet(t *testing.T) {
	var (
		consensusAddr   = "NiVihDFvZacZhujTWkBhRz32UDuNRp416f"
		consensusScript = "f7b4d00143932f3b6243cfc06cb4a68f22c739e2"
	)

	cfg, err := config.Load("../../config", netmode.MainNet)
	require.NoError(t, err)

	validators, err := validatorsFromConfig(cfg.ProtocolConfiguration)
	require.NoError(t, err)

	script, err := getNextConsensusAddress(validators)
	require.NoError(t, err)

	assert.Equal(t, consensusScript, script.String())
	assert.Equal(t, consensusAddr, address.Uint160ToString(script))
}
