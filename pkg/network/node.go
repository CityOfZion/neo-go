package network

import (
	"errors"
	"fmt"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
)

const (
	protoVersion = 0
)

var protoTickInterval = 5 * time.Second

// Node represents the local node.
type Node struct {
	// Config fields may not be modified while the server is running.
	Config

	server   *Server
	services uint64
	msgCh    chan *MessageTuple
	bc       *core.Blockchain
}

// MessageTuple respresents a tuple that holds the message being
// send along with the peer.
type MessageTuple struct {
	peer Peer
	msg  *Message
}

func newNode(s *Server, cfg Config) *Node {
	var startHash util.Uint256
	if cfg.Net == ModePrivNet {
		startHash = core.GenesisHashPrivNet()
	}

	bc := core.NewBlockchain(
		core.NewMemoryStore(),
		s.logger,
		startHash,
	)

	return &Node{
		Config: cfg,
		msgCh:  make(chan *MessageTuple),
		server: s,
		bc:     bc,
	}
}

func (n *Node) version() *payload.Version {
	return payload.NewVersion(n.server.id, n.ListenTCP, n.UserAgent, 1, n.Relay)
}

func (n *Node) startProtocol(peer Peer) {
	ticker := time.NewTicker(protoTickInterval).C

	for {
		select {
		case <-ticker:
			// Try to sync with the peer if his block height is higher then ours.
			if peer.Version().StartHeight > n.bc.HeaderHeight() {
				n.askMoreHeaders(peer)
			}
			// Only ask for more peers if the server has the capacity for it.
			if n.server.hasCapacity() {
				msg := NewMessage(n.Net, CMDGetAddr, nil)
				peer.Send(msg)
			}
		case <-peer.Done():
			return
		}
	}
}

// When a peer sends out his version we reply with verack after validating
// the version.
func (n *Node) handleVersionCmd(version *payload.Version, peer Peer) error {
	msg := NewMessage(n.Net, CMDVerack, nil)
	peer.Send(msg)
	return nil
}

// handleInvCmd handles the forwarded inventory received from the peer.
// We will use the getdata message to get more details about the received
// inventory.
// note: if the server has Relay on false, inventory messages are not received.
func (n *Node) handleInvCmd(inv *payload.Inventory, peer Peer) error {
	if !inv.Type.Valid() {
		return fmt.Errorf("invalid inventory type received: %s", inv.Type)
	}
	if len(inv.Hashes) == 0 {
		return errors.New("inventory has no hashes")
	}
	payload := payload.NewInventory(inv.Type, inv.Hashes)
	peer.Send(NewMessage(n.Net, CMDGetData, payload))
	return nil
}

// handleBlockCmd processes the received block received from its peer.
func (n *Node) handleBlockCmd(block *core.Block, peer Peer) error {
	hash, err := block.Hash()
	if err != nil {
		return err
	}
	n.server.logger.Printf("received block: %s height: %d numTX: %d", hash, block.Index, len(block.Transactions))
	return nil
}

// After a node sends out the getaddr message its receives a list of known peers
// in the network. handleAddrCmd processes that payload.
func (n *Node) handleAddrCmd(addressList *payload.AddressList, peer Peer) error {
	addrs := make([]string, len(addressList.Addrs))
	for i := 0; i < len(addrs); i++ {
		addrs[i] = addressList.Addrs[i].Address.String()
	}
	n.server.connectToPeers(addrs...)
	return nil
}

// The handleHeadersCmd will process the received headers from its peer.
// We call this in a routine cause we may block Peers Send() for to long.
func (n *Node) handleHeadersCmd(headers *payload.Headers, peer Peer) error {
	go func(headers []*core.Header) {
		if err := n.bc.AddHeaders(headers...); err != nil {
			n.server.logger.Printf("processing headers failed: %s", err)
			return
		}
		// The peer will respond with a maximum of 2000 headers in one batch.
		// We will ask one more batch here if needed. Eventually we will get synced
		// due to the startProtocol routine that will ask headers every protoTick.
		if n.bc.HeaderHeight() < peer.Version().StartHeight {
			n.askMoreHeaders(peer)
		}
	}(headers.Hdrs)

	return nil
}

// askMoreHeaders will send a getheaders message to the peer.
func (n *Node) askMoreHeaders(p Peer) {
	start := []util.Uint256{n.bc.CurrentHeaderHash()}
	payload := payload.NewGetBlocks(start, util.Uint256{})
	p.Send(NewMessage(n.Net, CMDGetHeaders, payload))
}

// blockhain implements the Noder interface.
func (n *Node) blockchain() *core.Blockchain { return n.bc }
