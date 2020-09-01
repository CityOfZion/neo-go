package crypto

import (
	"crypto/elliptic"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/crypto"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// ECDSAVerifyPrice is a gas price of a single verification.
const ECDSAVerifyPrice = 1000000

// ECDSASecp256r1Verify checks ECDSA signature using Secp256r1 elliptic curve.
func ECDSASecp256r1Verify(ic *interop.Context) error {
	return ecdsaVerify(ic, elliptic.P256())
}

// ECDSASecp256k1Verify checks ECDSA signature using Secp256k1 elliptic curve
func ECDSASecp256k1Verify(ic *interop.Context) error {
	return ecdsaVerify(ic, btcec.S256())
}

// ecdsaVerify is internal representation of ECDSASecp256k1Verify and
// ECDSASecp256r1Verify.
func ecdsaVerify(ic *interop.Context, curve elliptic.Curve) error {
	msg := getMessage(ic, ic.VM.Estack().Pop().Item())
	hashToCheck := hash.Sha256(msg).BytesBE()
	keyb := ic.VM.Estack().Pop().Bytes()
	signature := ic.VM.Estack().Pop().Bytes()
	pkey, err := keys.NewPublicKeyFromBytes(keyb, curve)
	if err != nil {
		return err
	}
	res := pkey.Verify(signature, hashToCheck)
	ic.VM.Estack().PushVal(res)
	return nil
}

// ECDSASecp256r1CheckMultisig checks multiple ECDSA signatures at once using
// Secp256r1 elliptic curve.
func ECDSASecp256r1CheckMultisig(ic *interop.Context) error {
	return ecdsaCheckMultisig(ic, elliptic.P256())
}

// ECDSASecp256k1CheckMultisig checks multiple ECDSA signatures at once using
// Secp256k1 elliptic curve.
func ECDSASecp256k1CheckMultisig(ic *interop.Context) error {
	return ecdsaCheckMultisig(ic, btcec.S256())
}

// ecdsaCheckMultisig is internal representation of ECDSASecp256r1CheckMultisig and
// ECDSASecp256k1CheckMultisig
func ecdsaCheckMultisig(ic *interop.Context, curve elliptic.Curve) error {
	msg := getMessage(ic, ic.VM.Estack().Pop().Item())
	hashToCheck := hash.Sha256(msg).BytesBE()
	pkeys, err := ic.VM.Estack().PopSigElements()
	if err != nil {
		return fmt.Errorf("wrong parameters: %w", err)
	}
	if !ic.VM.AddGas(ECDSAVerifyPrice * int64(len(pkeys))) {
		return errors.New("gas limit exceeded")
	}
	sigs, err := ic.VM.Estack().PopSigElements()
	if err != nil {
		return fmt.Errorf("wrong parameters: %w", err)
	}
	// It's ok to have more keys than there are signatures (it would
	// just mean that some keys didn't sign), but not the other way around.
	if len(pkeys) < len(sigs) {
		return errors.New("more signatures than there are keys")
	}
	sigok := vm.CheckMultisigPar(ic.VM, curve, hashToCheck, pkeys, sigs)
	ic.VM.Estack().PushVal(sigok)
	return nil
}

func getMessage(ic *interop.Context, item stackitem.Item) []byte {
	var msg []byte
	switch val := item.(type) {
	case *stackitem.Interop:
		msg = val.Value().(crypto.Verifiable).GetSignedPart()
	case stackitem.Null:
		msg = ic.Container.GetSignedPart()
	default:
		var err error
		if msg, err = val.TryBytes(); err != nil {
			return nil
		}
	}
	return msg
}
