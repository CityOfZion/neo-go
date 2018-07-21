package wire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type Messager interface {
	EncodePayload(w io.Writer) error
	DecodePayload(r io.Reader) error
	PayloadLength() uint32
	Checksum() uint32
	Command() CommandType
}

var (
	errChecksumMismatch = errors.New("checksum mismatch")
)

// CommandType represents the type of a message command.
type CommandType string

// Valid protocol commands used to send between nodes.
// use this to get
const (
	CMDVersion    CommandType = "version"
	CMDVerack     CommandType = "verack"
	CMDGetAddr    CommandType = "getaddr"
	CMDAddr       CommandType = "addr"
	CMDGetHeaders CommandType = "getheaders"
	CMDHeaders    CommandType = "headers"
	CMDGetBlocks  CommandType = "getblocks"
	CMDInv        CommandType = "inv"
	CMDGetData    CommandType = "getdata"
	CMDBlock      CommandType = "block"
	CMDTX         CommandType = "tx"
	CMDConsensus  CommandType = "consensus"
	CMDUnknown    CommandType = "unknown"
)

func WriteMessage(w io.Writer, magic Magic, message Messager) error {
	if err := binary.Write(w, binary.LittleEndian, magic); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, cmdToByteArray(message.Command())); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, message.PayloadLength()); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, message.Checksum()); err != nil {
		return err
	}
	if err := message.EncodePayload(w); err != nil {
		return err
	}
	return nil
}

func ReadMessage(r io.Reader, magic Magic) (Messager, error) {
	var header MessageHeader
	if err := header.DecodeMessageHeader(r); err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	n, err := io.CopyN(buf, r, int64(header.Length))
	if err != nil {
		return nil, err
	}

	if uint32(n) != header.Length {
		return nil, fmt.Errorf("expected to have read exactly %d bytes got %d", header.Length, n)
	}

	// Compare the checksum of the payload.
	if !compareChecksum(header.Checksum, buf.Bytes()) {
		return nil, errChecksumMismatch
	}

	switch header.Command {
	case CMDVersion:
		v := &VersionMessage{}
		return v, v.DecodePayload(r)
	}
	return nil, nil

}

func cmdToByteArray(cmd CommandType) [cmdSize]byte {
	cmdLen := len(cmd)
	if cmdLen > cmdSize {
		panic("exceeded command max length of size 12")
	}

	// The command can have max 12 bytes, rest is filled with 0.
	b := [cmdSize]byte{}
	for i := 0; i < cmdLen; i++ {
		b[i] = cmd[i]
	}

	return b
}

func sumSHA256(b []byte) []byte {
	h := sha256.New()
	h.Write(b)
	return h.Sum(nil)
}

func compareChecksum(have uint32, b []byte) bool {
	sum := sumSHA256(sumSHA256(b))[:4]
	want := binary.LittleEndian.Uint32(sum)
	return have == want
}
