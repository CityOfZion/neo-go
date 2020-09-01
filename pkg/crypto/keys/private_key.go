package keys

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/rfc6979"
)

// PrivateKey represents a NEO private key and provides a high level API around
// ecdsa.PrivateKey.
type PrivateKey struct {
	ecdsa.PrivateKey
}

// NewPrivateKey creates a new random Secp256k1 private key.
func NewPrivateKey() (*PrivateKey, error) {
	priv, x, y, err := elliptic.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{
		ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: elliptic.P256(),
				X:     x,
				Y:     y,
			},
			D: new(big.Int).SetBytes(priv),
		},
	}, nil
}

// NewPrivateKeyFromHex returns a Secp256k1 PrivateKey created from the
// given hex string.
func NewPrivateKeyFromHex(str string) (*PrivateKey, error) {
	b, err := hex.DecodeString(str)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeyFromBytes(b)
}

// NewPrivateKeyFromBytes returns a NEO Secp256r1 PrivateKey from the given
// byte slice.
func NewPrivateKeyFromBytes(b []byte) (*PrivateKey, error) {
	if len(b) != 32 {
		return nil, fmt.Errorf(
			"invalid byte length: expected %d bytes got %d", 32, len(b),
		)
	}
	var (
		c = elliptic.P256()
		d = new(big.Int).SetBytes(b)
	)

	x, y := c.ScalarBaseMult(d.Bytes())

	return &PrivateKey{
		ecdsa.PrivateKey{
			PublicKey: ecdsa.PublicKey{
				Curve: c,
				X:     x,
				Y:     y,
			},
			D: d,
		},
	}, nil
}

// NewPrivateKeyFromASN1 returns a NEO Secp256k1 PrivateKey from the ASN.1
// serialized key.
func NewPrivateKeyFromASN1(b []byte) (*PrivateKey, error) {
	privkey, err := x509.ParseECPrivateKey(b)
	if err != nil {
		return nil, err
	}
	return NewPrivateKeyFromBytes(privkey.D.Bytes())
}

// PublicKey derives the public key from the private key.
func (p *PrivateKey) PublicKey() *PublicKey {
	result := PublicKey(p.PrivateKey.PublicKey)
	return &result
}

// NewPrivateKeyFromWIF returns a NEO PrivateKey from the given
// WIF (wallet import format).
func NewPrivateKeyFromWIF(wif string) (*PrivateKey, error) {
	w, err := WIFDecode(wif, WIFVersion)
	if err != nil {
		return nil, err
	}
	return w.PrivateKey, nil
}

// WIF returns the (wallet import format) of the PrivateKey.
// Good documentation about this process can be found here:
// https://en.bitcoin.it/wiki/Wallet_import_format
func (p *PrivateKey) WIF() string {
	w, err := WIFEncode(p.Bytes(), WIFVersion, true)
	// The only way WIFEncode() can fail is if we're to give it a key of
	// wrong size, but we have a proper key here, aren't we?
	if err != nil {
		panic(err)
	}
	return w
}

// Address derives the public NEO address that is coupled with the private key, and
// returns it as a string.
func (p *PrivateKey) Address() string {
	pk := p.PublicKey()
	return pk.Address()
}

// GetScriptHash returns verification script hash for public key associated with
// the private key.
func (p *PrivateKey) GetScriptHash() util.Uint160 {
	pk := p.PublicKey()
	return pk.GetScriptHash()
}

// Sign signs arbitrary length data using the private key.
func (p *PrivateKey) Sign(data []byte) []byte {
	var (
		privateKey = &p.PrivateKey
		digest     = sha256.Sum256(data)
	)

	r, s := rfc6979.SignECDSA(privateKey, digest[:], sha256.New)
	return getSignatureSlice(privateKey.Curve, r, s)
}

func getSignatureSlice(curve elliptic.Curve, r, s *big.Int) []byte {
	params := curve.Params()
	curveOrderByteSize := params.P.BitLen() / 8
	rBytes, sBytes := r.Bytes(), s.Bytes()
	signature := make([]byte, curveOrderByteSize*2)
	copy(signature[curveOrderByteSize-len(rBytes):], rBytes)
	copy(signature[curveOrderByteSize*2-len(sBytes):], sBytes)

	return signature
}

// String implements the stringer interface.
func (p *PrivateKey) String() string {
	return hex.EncodeToString(p.Bytes())
}

// Bytes returns the underlying bytes of the PrivateKey.
func (p *PrivateKey) Bytes() []byte {
	bytes := p.D.Bytes()
	result := make([]byte, 32)
	copy(result[32-len(bytes):], bytes)

	return result
}
