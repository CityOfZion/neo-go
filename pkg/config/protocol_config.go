package config

import (
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
)

// ProtocolConfiguration represents the protocol config.
type (
	ProtocolConfiguration struct {
		Magic       netmode.Magic `yaml:"Magic"`
		MemPoolSize int           `yaml:"MemPoolSize"`
		// SaveStorageBatch enables storage batch saving before every persist.
		SaveStorageBatch bool     `yaml:"SaveStorageBatch"`
		SecondsPerBlock  int      `yaml:"SecondsPerBlock"`
		SeedList         []string `yaml:"SeedList"`
		StandbyCommittee []string `yaml:"StandbyCommittee"`
		ValidatorsCount  int      `yaml:"ValidatorsCount"`
		// Whether to verify received blocks.
		VerifyBlocks bool `yaml:"VerifyBlocks"`
		// Whether to verify transactions in received blocks.
		VerifyTransactions bool `yaml:"VerifyTransactions"`
	}
)
