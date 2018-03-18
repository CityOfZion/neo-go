package transaction

import (
	"encoding/binary"
	"io"

	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// # 发行总量，共有2种模式：
// # 1. 限量模式：当Amount为正数时，表示当前资产的最大总量为Amount，且不可修改（股权在未来可能会支持扩股或增发，会考虑需要公司签名或一定比例的股东签名认可）。
// # 2. 不限量模式：当Amount等于-1时，表示当前资产可以由创建者无限量发行。这种模式的自由度最大，但是公信力最低，不建议使用。
// # 在使用过程中，根据资产类型的不同，能够使用的总量模式也不同，具体规则如下：
// # 1. 对于股权，只能使用限量模式；
// # 2. 对于货币，只能使用不限量模式；
// # 3. 对于点券，可以使用任意模式；
//
// In English:
// # Total number of releases, there are 2 modes:
// # 1. Limited amount: When Amount is positive, it means that the maximum amount of current assets is Amount
// and can not be modified (the equity may support the expansion or issuance in the future, will consider the
// need for company signature or a certain percentage of shareholder signature recognition ).
// # 2. Unlimited mode: When Amount is equal to -1, it means that the current asset can be issued by the
// creator unlimited. This mode of freedom is the largest, but the credibility of the lowest, not recommended.
// # In the use of the process, according to the different types of assets, can use the total amount of
// different models, the specific rules are as follows:
// # 1. For equity, use only limited models;
// # 2. For currencies, use only unlimited models;
// # 3. For point coupons, you can use any pattern;

// RegisterTX represents a register transaction.
type RegisterTX struct {
	// The type of the asset being registered.
	AssetType AssetType

	// Name of the asset being registered.
	Name []byte

	// Amount registered
	// Unlimited mode -0.00000001
	Amount util.Fixed8

	// Decimals
	Precision uint8

	// Public key of the owner
	Owner *crypto.PublicKey

	Admin util.Uint160
}

// DecodeBinary implements the Payload interface.
func (tx *RegisterTX) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &tx.AssetType); err != nil {
		return err
	}
	lenName := util.ReadVarUint(r)
	tx.Name = make([]byte, lenName)
	if err := binary.Read(r, binary.LittleEndian, &tx.Name); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &tx.Amount); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &tx.Precision); err != nil {
		return err
	}

	tx.Owner = &crypto.PublicKey{}
	if err := tx.Owner.DecodeBinary(r); err != nil {
		return err
	}

	return binary.Read(r, binary.LittleEndian, &tx.Admin)
}

// EncodeBinary implements the Payload interface.
func (tx *RegisterTX) EncodeBinary(w io.Writer) error {
	return nil
}
