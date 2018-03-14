package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// BlockBase holds the base info of a block
type BlockBase struct {
	// Version of the block.
	Version uint32
	// hash of the previous block.
	PrevHash util.Uint256
	// Root hash of a transaction list.
	MerkleRoot util.Uint256
	// The time stamp of each block must be later than previous block's time stamp.
	// Generally the difference of two block's time stamp is about 15 seconds and imprecision is allowed.
	// The height of the block must be exactly equal to the height of the previous block plus 1.
	Timestamp uint32
	// index/height of the block
	Index uint32
	// Random number also called nonce
	ConsensusData uint64
	// Contract addresss of the next miner
	NextConsensus util.Uint160
	// Padding that is fixed to 1
	_ uint8
	// Script used to validate the block
	Script *transaction.Witness

	// hash of this block, created when binary encoded.
	hash util.Uint256
}

// Verify verifies the integrity of the BlockBase.
func (b *BlockBase) Verify() bool {
	// TODO: Need a persisted blockchain for this.
	return true
}

// DecodeBinary implements the payload interface.
func (b *BlockBase) DecodeBinary(r io.Reader) error {
	if err := binary.Read(r, binary.LittleEndian, &b.Version); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.PrevHash); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.MerkleRoot); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.Timestamp); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.Index); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.ConsensusData); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.NextConsensus); err != nil {
		return err
	}

	var padding uint8
	if err := binary.Read(r, binary.LittleEndian, &padding); err != nil {
		return err
	}
	if padding != 1 {
		return fmt.Errorf("format error: padding must equal 1 got %d", padding)
	}

	b.Script = &transaction.Witness{}
	if err := b.Script.DecodeBinary(r); err != nil {
		return err
	}

	// Make the hash of the block here so we dont need to do this
	// again.
	hash, err := b.createHash()
	if err != nil {
		return err
	}
	b.hash = hash
	return nil
}

// Hash return the hash of the block.
func (b *BlockBase) Hash() util.Uint256 {
	return b.hash
}

// createHash creates the hash of the block.
// When calculating the hash value of the block, instead of calculating the entire block,
// only first seven fields in the block head will be calculated, which are
// version, PrevBlock, MerkleRoot, timestamp, and height, the nonce, NextMiner.
// Since MerkleRoot already contains the hash value of all transactions,
// the modification of transaction will influence the hash value of the block.
func (b *BlockBase) createHash() (hash util.Uint256, err error) {
	buf := new(bytes.Buffer)
	if err = b.encodeHashableFields(buf); err != nil {
		return hash, err
	}

	// Double hash the encoded fields.
	hash = sha256.Sum256(buf.Bytes())
	hash = sha256.Sum256(hash.Bytes())
	return hash, nil
}

// encodeHashableFields will only encode the fields used for hashing.
// see Hash() for more information about the fields.
func (b *BlockBase) encodeHashableFields(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, &b.Version); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, &b.PrevHash); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, &b.MerkleRoot); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, &b.Timestamp); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, &b.Index); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, &b.ConsensusData); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, &b.NextConsensus)
}

// EncodeBinary implements the Payload interface
func (b *BlockBase) EncodeBinary(w io.Writer) error {
	if err := b.encodeHashableFields(w); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint8(1)); err != nil {
		return err
	}
	return b.Script.EncodeBinary(w)
}
