package result

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NEP5Balances is a result for the getnep5balances RPC call.
type NEP5Balances struct {
	Balances []NEP5Balance `json:"balance"`
	Address  string        `json:"address"`
}

// NEP5Balance represents balance for the single token contract.
type NEP5Balance struct {
	Asset       util.Uint160 `json:"assethash"`
	Amount      string       `json:"amount"`
	LastUpdated uint32       `json:"lastupdatedblock"`
}

// nep5Balance is an auxiliary struct for proper Asset marshaling.
type nep5Balance struct {
	Asset       string `json:"assethash"`
	Amount      string `json:"amount"`
	LastUpdated uint32 `json:"lastupdatedblock"`
}

// NEP5Transfers is a result for the getnep5transfers RPC.
type NEP5Transfers struct {
	Sent     []NEP5Transfer `json:"sent"`
	Received []NEP5Transfer `json:"received"`
	Address  string         `json:"address"`
}

// NEP5Transfer represents single NEP5 transfer event.
type NEP5Transfer struct {
	Timestamp   uint64       `json:"timestamp"`
	Asset       util.Uint160 `json:"assethash"`
	Address     string       `json:"transferaddress,omitempty"`
	Amount      string       `json:"amount"`
	Index       uint32       `json:"blockindex"`
	NotifyIndex uint32       `json:"transfernotifyindex"`
	TxHash      util.Uint256 `json:"txhash"`
}

// MarshalJSON implements json.Marshaler interface.
func (b *NEP5Balance) MarshalJSON() ([]byte, error) {
	s := &nep5Balance{
		Asset:       b.Asset.StringLE(),
		Amount:      b.Amount,
		LastUpdated: b.LastUpdated,
	}
	return json.Marshal(s)
}

// UnmarshalJSON implements json.Unmarshaler interface.
func (b *NEP5Balance) UnmarshalJSON(data []byte) error {
	s := new(nep5Balance)
	if err := json.Unmarshal(data, s); err != nil {
		return err
	}
	asset, err := util.Uint160DecodeStringLE(s.Asset)
	if err != nil {
		return err
	}
	b.Amount = s.Amount
	b.Asset = asset
	b.LastUpdated = s.LastUpdated
	return nil
}
