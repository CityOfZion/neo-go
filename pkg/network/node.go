package network

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network/payload"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/go-kit/kit/log"
)

const (
	protoVersion = 0
)

var protoTickInterval = 5 * time.Second

// Node represents the local node.
type Node struct {
	// Config fields may not be modified while the server is running.
	Config

	logger   log.Logger
	server   *Server
	services uint64
	bc       *core.Blockchain
	protoIn  chan messageTuple
}

// messageTuple respresents a tuple that holds the message being
// send along with its peer.
type messageTuple struct {
	peer Peer
	msg  *Message
}

func newNode(s *Server, cfg Config) *Node {
	var startHash util.Uint256
	if cfg.Net == ModePrivNet {
		startHash = core.GenesisHashPrivNet()
	}
	if cfg.Net == ModeTestNet {
		startHash = core.GenesisHashTestNet()
	}
	if cfg.Net == ModeMainNet {
		startHash = core.GenesisHashMainNet()
	}

	bc := core.NewBlockchain(
		core.NewMemoryStore(),
		startHash,
	)

	logger := log.NewLogfmtLogger(os.Stderr)
	logger = log.With(logger, "component", "node")

	n := &Node{
		Config:  cfg,
		protoIn: make(chan messageTuple),
		server:  s,
		bc:      bc,
		logger:  logger,
	}
	go n.handleMessages()

	return n
}

func (n *Node) version() *payload.Version {
	return payload.NewVersion(n.server.id, n.ListenTCP, n.UserAgent, 1, n.Relay)
}

func (n *Node) startProtocol(p Peer) {
	ticker := time.NewTicker(protoTickInterval).C

	for {
		select {
		case <-p.Done():
			n.logger.Log("event", "peer done")
			return
		case <-ticker:
			select {
			case <-p.Done():
				return
			}
			// Try to sync with the peer if his block height is higher then ours.
			if p.Version().StartHeight > n.bc.HeaderHeight() {
				n.askMoreHeaders(p)
			}
			// Only ask for more peers if the server has the capacity for it.
			if n.server.hasCapacity() {
				msg := NewMessage(n.Net, CMDGetAddr, nil)
				n.send(msg, p)
			}
		}
	}
}

// When a peer sends out his version we reply with verack after validating
// the version.
func (n *Node) handleVersionCmd(version *payload.Version, p Peer) error {
	msg := NewMessage(n.Net, CMDVerack, nil)
	n.send(msg, p)
	return nil
}

// handleInvCmd handles the forwarded inventory received from the peer.
// We will use the getdata message to get more details about the received
// inventory.
// note: if the server has Relay on false, inventory messages are not received.
func (n *Node) handleInvCmd(inv *payload.Inventory, p Peer) error {
	if !inv.Type.Valid() {
		return fmt.Errorf("invalid inventory type received: %s", inv.Type)
	}
	if len(inv.Hashes) == 0 {
		return errors.New("inventory has no hashes")
	}
	payload := payload.NewInventory(inv.Type, inv.Hashes)
	n.send(NewMessage(n.Net, CMDGetData, payload), p)
	return nil
}

// handleBlockCmd processes the received block received from its peer.
func (n *Node) handleBlockCmd(block *core.Block, peer Peer) error {
	n.logger.Log(
		"event", "block received",
		"index", block.Index,
		"hash", block.Hash(),
		"tx", len(block.Transactions),
	)

	return n.bc.AddBlock(block)
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
			n.logger.Log("msg", "failed processing headers", "err", err)
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
	n.send(NewMessage(n.Net, CMDGetHeaders, payload), p)
}

func (n *Node) send(msg *Message, p Peer) {
	n.logger.Log(
		"event", "message send",
		"to", p.Endpoint(),
		"msg", msg.CommandType(),
	)

	p.Send(msg)
}

// blockhain implements the Noder interface.
func (n *Node) blockchain() *core.Blockchain { return n.bc }

// handleProto implements the protoHandler interface.
func (n *Node) handleProto(msg *Message, p Peer) {
	n.logger.Log(
		"event", "message received",
		"from", p.Endpoint(),
		"msg", msg.CommandType(),
	)

	n.protoIn <- messageTuple{
		msg:  msg,
		peer: p,
	}
}

func (n *Node) handleMessages() {
	for {
		t := <-n.protoIn

		var (
			msg = t.msg
			p   = t.peer
			err error
		)

		switch msg.CommandType() {
		case CMDVersion:
			version := msg.Payload.(*payload.Version)
			err = n.handleVersionCmd(version, p)
		case CMDAddr:
			addressList := msg.Payload.(*payload.AddressList)
			err = n.handleAddrCmd(addressList, p)
		case CMDInv:
			inventory := msg.Payload.(*payload.Inventory)
			err = n.handleInvCmd(inventory, p)
		case CMDBlock:
			block := msg.Payload.(*core.Block)
			err = n.handleBlockCmd(block, p)
		case CMDHeaders:
			headers := msg.Payload.(*payload.Headers)
			err = n.handleHeadersCmd(headers, p)
		case CMDTX:
			//			tx := msg.Payload.(*transaction.Transaction)
			//n.logger.Log("tx", fmt.Sprintf("%+v", tx))
		case CMDVerack:
			// Only start the protocol if we got the version and verack
			// received.
			if p.Version() != nil {
				n.logger.Log("event", "start protocol")
				go n.startProtocol(p)
			}
		case CMDUnknown:
			err = errors.New("received non-protocol messgae")
		}

		if err != nil {
			n.logger.Log(
				"msg", "failed processing message",
				"command", msg.CommandType,
				"err", err,
			)
		}
	}
}
