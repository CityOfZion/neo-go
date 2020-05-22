package network

import (
	"net"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendVersion(t *testing.T) {
	var (
		s = newTestServer(t)
		p = newLocalPeer(t, s)
	)
	s.Port = 3000
	s.UserAgent = "/test/"

	p.messageHandler = func(t *testing.T, msg *Message) {
		assert.Equal(t, CMDVersion, msg.Command)
		assert.IsType(t, msg.Payload, &payload.Version{})
		version := msg.Payload.(*payload.Version)
		assert.NotZero(t, version.Nonce)
		assert.Equal(t, uint16(3000), version.Port)
		assert.Equal(t, uint64(1), version.Services)
		assert.Equal(t, uint32(0), version.Version)
		assert.Equal(t, []byte("/test/"), version.UserAgent)
		assert.Equal(t, uint32(0), version.StartHeight)
	}

	require.NoError(t, p.SendVersion())
}

// Server should reply with a verack after receiving a valid version.
func TestVerackAfterHandleVersionCmd(t *testing.T) {
	var (
		s = newTestServer(t)
		p = newLocalPeer(t, s)
	)
	na, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:3000")
	p.netaddr = *na

	// Should have a verack
	p.messageHandler = func(t *testing.T, msg *Message) {
		assert.Equal(t, CMDVerack, msg.Command)
	}
	version := payload.NewVersion(0, 1337, 3000, "/NEO-GO/", 0, true)

	require.NoError(t, s.handleVersionCmd(p, version))
}

// Server should not reply with a verack after receiving a
// invalid version and disconnects the peer.
func TestServerNotSendsVerack(t *testing.T) {
	var (
		s  = newTestServer(t)
		p  = newLocalPeer(t, s)
		p2 = newLocalPeer(t, s)
	)
	s.id = 1
	s.Net = 56753
	finished := make(chan struct{})
	go func() {
		s.run()
		close(finished)
	}()
	defer func() {
		// close via quit as server was started via `run()`, not `Start()`
		close(s.quit)
		<-finished
	}()

	na, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:3000")
	p.netaddr = *na
	p2.netaddr = *na
	s.register <- p

	// identical id's
	version := payload.NewVersion(56753, 1, 3000, "/NEO-GO/", 0, true)
	err := s.handleVersionCmd(p, version)
	assert.NotNil(t, err)
	assert.Equal(t, errIdenticalID, err)

	// Different IDs, but also different magics
	version.Nonce = 2
	version.Magic = 56752
	err = s.handleVersionCmd(p, version)
	assert.NotNil(t, err)
	assert.Equal(t, errInvalidNetwork, err)

	// Different IDs and same network, make handshake pass.
	version.Magic = 56753
	require.NoError(t, s.handleVersionCmd(p, version))
	require.NoError(t, p.HandleVersionAck())
	require.Equal(t, true, p.Handshaked())

	// Second handshake from the same peer should fail.
	s.register <- p2
	err = s.handleVersionCmd(p2, version)
	assert.NotNil(t, err)
	require.Equal(t, errAlreadyConnected, err)
}

func TestRequestHeaders(t *testing.T) {
	var (
		s = newTestServer(t)
		p = newLocalPeer(t, s)
	)
	p.messageHandler = func(t *testing.T, msg *Message) {
		assert.IsType(t, &payload.GetBlocks{}, msg.Payload)
		assert.Equal(t, CMDGetHeaders, msg.Command)
	}
	s.requestHeaders(p)
}
