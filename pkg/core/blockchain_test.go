package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestAddHeaders(t *testing.T) {
	bc := newTestChain(t)
	h1 := newBlock(1).Header()
	h2 := newBlock(2).Header()
	h3 := newBlock(3).Header()

	if err := bc.AddHeaders(h1, h2, h3); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, bc.blockCache.Len())
	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(1), bc.storedHeaderCount)
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	// Add them again, they should not be added.
	if err := bc.AddHeaders(h3, h2, h1); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(1), bc.storedHeaderCount)
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())
}

func TestAddBlock(t *testing.T) {
	bc := newTestChain(t)
	blocks := []*Block{
		newBlock(1),
		newBlock(2),
		newBlock(3),
	}

	for i := 0; i < len(blocks); i++ {
		if err := bc.AddBlock(blocks[i]); err != nil {
			t.Fatal(err)
		}
	}

	lastBlock := blocks[len(blocks)-1]
	assert.Equal(t, 3, bc.blockCache.Len())
	assert.Equal(t, lastBlock.Index, bc.HeaderHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())
	assert.Equal(t, uint32(1), bc.storedHeaderCount)

	if err := bc.persist(); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, lastBlock.Index, bc.BlockHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())
	assert.Equal(t, 0, bc.blockCache.Len())
}

func TestGetHeader(t *testing.T) {
	bc := newTestChain(t)
	block := newBlock(1)
	err := bc.AddBlock(block)
	assert.Nil(t, err)

	hash := block.Hash()
	header, err := bc.getHeader(hash)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, block.Header(), header)

	block = newBlock(2)
	hash = block.Hash()
	_, err = bc.getHeader(block.Hash())
	assert.NotNil(t, err)
}

func TestGetBlock(t *testing.T) {
	bc := newTestChain(t)
	blocks := makeBlocks(100)

	for i := 0; i < len(blocks); i++ {
		if err := bc.AddBlock(blocks[i]); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < len(blocks); i++ {
		block, err := bc.GetBlock(blocks[i].Hash())
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, blocks[i].Index, block.Index)
		assert.Equal(t, blocks[i].Hash(), block.Hash())
	}
}

func TestHasBlock(t *testing.T) {
	bc := newTestChain(t)
	blocks := makeBlocks(50)

	for i := 0; i < len(blocks); i++ {
		if err := bc.AddBlock(blocks[i]); err != nil {
			t.Fatal(err)
		}
	}
	assert.Nil(t, bc.persist())

	for i := 0; i < len(blocks); i++ {
		assert.True(t, bc.HasBlock(blocks[i].Hash()))
	}

	newBlock := newBlock(51)
	assert.False(t, bc.HasBlock(newBlock.Hash()))
}

func newTestChain(t *testing.T) *Blockchain {
	startHash, _ := util.Uint256DecodeString("a")
	chain, err := NewBlockchain(storage.NewMemoryStore(), startHash)
	if err != nil {
		t.Fatal(err)
	}
	return chain
}
