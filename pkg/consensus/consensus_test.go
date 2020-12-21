package consensus

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/nspcc-dev/dbft/block"
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/dbft/timer"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewService(t *testing.T) {
	srv := newTestService(t)
	tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 100000)
	tx.ValidUntilBlock = 1
	addSender(t, tx)
	signTx(t, srv.Chain, tx)
	require.NoError(t, srv.Chain.PoolTx(tx))

	var txx []block.Transaction
	require.NotPanics(t, func() { txx = srv.getVerifiedTx() })
	require.Len(t, txx, 1)
	require.Equal(t, tx, txx[0])
	srv.Chain.Close()
}

func initServiceNextConsensus(t *testing.T, newAcc *wallet.Account, offset uint32) (*service, *wallet.Account) {
	acc, err := wallet.NewAccountFromWIF(testchain.WIF(testchain.IDToOrder(0)))
	require.NoError(t, err)
	priv := acc.PrivateKey()
	require.NoError(t, acc.ConvertMultisig(1, keys.PublicKeys{priv.PublicKey()}))

	priv1 := testchain.PrivateKeyByID(1)
	priv2 := testchain.PrivateKeyByID(2)
	bc := newSingleTestChain(t, func(c *config.Config) {
		c.ProtocolConfiguration.StandbyCommittee = append(c.ProtocolConfiguration.StandbyCommittee,
			hex.EncodeToString(priv1.PublicKey().Bytes()),
			hex.EncodeToString(priv2.PublicKey().Bytes()))
	})
	newPriv := newAcc.PrivateKey()

	privateKeys := []*keys.PrivateKey{newPriv, priv1, priv2}

	// Transfer funds to new validator.
	w := io.NewBufBinWriter()
	for i, priv := range privateKeys {
		neoToSend := int64(native.NEOTotalSupply/4 + native.NEOTotalSupply/12*(2-i))
		emit.AppCallWithOperationAndArgs(w.BinWriter, bc.GoverningTokenHash(), "transfer",
			acc.Contract.ScriptHash().BytesBE(), priv.GetScriptHash().BytesBE(), neoToSend, nil)
		emit.Opcodes(w.BinWriter, opcode.ASSERT)
		emit.AppCallWithOperationAndArgs(w.BinWriter, bc.UtilityTokenHash(), "transfer",
			acc.Contract.ScriptHash().BytesBE(), priv.GetScriptHash().BytesBE(), int64(1_000_000_000), nil)
		emit.Opcodes(w.BinWriter, opcode.ASSERT)
	}
	require.NoError(t, w.Err)

	tx := transaction.New(netmode.UnitTestNet, w.Bytes(), 63_000_000)
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	tx.NetworkFee = 10_000_000
	tx.Signers = []transaction.Signer{{Scopes: transaction.Global, Account: acc.Contract.ScriptHash()}}
	require.NoError(t, acc.SignTx(tx))
	require.NoError(t, bc.PoolTx(tx))

	srv := newTestServiceWithChain(t, bc)
	srv.dbft.Start()

	t.Log("registed new candidates")
	for i, priv := range privateKeys {
		w := io.NewBufBinWriter()
		emit.AppCallWithOperationAndArgs(w.BinWriter, bc.GoverningTokenHash(), "registerCandidate", priv.PublicKey().Bytes())
		require.NoError(t, w.Err)

		tx = transaction.New(netmode.UnitTestNet, w.Bytes(), 21_000_000)
		tx.ValidUntilBlock = bc.BlockHeight() + 1
		tx.NetworkFee = 20_000_000
		tx.Signers = []transaction.Signer{{Scopes: transaction.Global, Account: priv.GetScriptHash()}}
		require.NoError(t, wallet.NewAccountFromPrivateKey(priv).SignTx(tx))
		require.NoError(t, bc.PoolTx(tx), "at %d", i)
	}

	srv.dbft.OnTimeout(timer.HV{Height: srv.dbft.Context.BlockIndex})

	for i := srv.dbft.BlockIndex; !native.ShouldUpdateCommittee(i+offset, bc); i++ {
		srv.dbft.OnTimeout(timer.HV{Height: srv.dbft.Context.BlockIndex})
	}

	t.Log("vote for candidates")
	for i, priv := range privateKeys {
		w := io.NewBufBinWriter()
		emit.AppCallWithOperationAndArgs(w.BinWriter, bc.GoverningTokenHash(), "vote",
			priv.GetScriptHash(), priv.PublicKey().Bytes())
		emit.Opcodes(w.BinWriter, opcode.ASSERT)
		require.NoError(t, w.Err)

		tx = transaction.New(netmode.UnitTestNet, w.Bytes(), 20_000_000)
		tx.ValidUntilBlock = bc.BlockHeight() + 1
		tx.NetworkFee = 20_000_000
		tx.Signers = []transaction.Signer{{Scopes: transaction.Global, Account: priv.GetScriptHash()}}
		require.NoError(t, wallet.NewAccountFromPrivateKey(priv).SignTx(tx))
		require.NoError(t, bc.PoolTx(tx), "at %d", i)
	}

	srv.dbft.OnTimeout(timer.HV{Height: srv.dbft.BlockIndex})

	return srv, acc
}

func TestService_NextConsensus(t *testing.T) {
	newAcc, err := wallet.NewAccount()
	require.NoError(t, err)
	script, err := smartcontract.CreateMajorityMultiSigRedeemScript(keys.PublicKeys{newAcc.PrivateKey().PublicKey()})
	require.NoError(t, err)

	checkNextConsensus := func(t *testing.T, bc *core.Blockchain, height uint32, h util.Uint160) {
		hdrHash := bc.GetHeaderHash(int(height))
		hdr, err := bc.GetHeader(hdrHash)
		require.NoError(t, err)
		require.Equal(t, h, hdr.NextConsensus)
	}

	t.Run("vote 1 block before update", func(t *testing.T) { // voting occurs every block in SingleTestChain
		srv, acc := initServiceNextConsensus(t, newAcc, 1)
		bc := srv.Chain.(*core.Blockchain)
		defer bc.Close()

		height := bc.BlockHeight()
		checkNextConsensus(t, bc, height, acc.Contract.ScriptHash())
		// Reset       <- we are here, next block will see new committee
		// Block       <-
		// PostPersist <- update committee
		srv.dbft.OnTimeout(timer.HV{Height: srv.dbft.BlockIndex})
		checkNextConsensus(t, bc, height+1, hash.Hash160(script))
	})

	t.Run("vote 2 blocks before update", func(t *testing.T) {
		srv, acc := initServiceNextConsensus(t, newAcc, 2)
		bc := srv.Chain.(*core.Blockchain)
		defer bc.Close()

		height := bc.BlockHeight()
		checkNextConsensus(t, bc, height, acc.Contract.ScriptHash())
		// Reset       <- we are here
		// Block       <-
		// PostPersist <- calculate committee, no update
		//
		// Reset       <- next block will see new committee
		// Block       <-
		// PostPersist <- update committee
		srv.dbft.OnTimeout(timer.HV{Height: srv.dbft.BlockIndex})
		checkNextConsensus(t, bc, height+1, acc.Contract.ScriptHash())

		srv.dbft.OnTimeout(timer.HV{Height: srv.dbft.BlockIndex})
		checkNextConsensus(t, bc, height+2, hash.Hash160(script))
	})
}

func TestService_GetVerified(t *testing.T) {
	srv := newTestService(t)
	srv.dbft.Start()
	var txs []*transaction.Transaction
	for i := 0; i < 4; i++ {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 100000)
		tx.Nonce = 123 + uint32(i)
		tx.ValidUntilBlock = 1
		txs = append(txs, tx)
	}
	addSender(t, txs...)
	signTx(t, srv.Chain, txs...)
	require.NoError(t, srv.Chain.PoolTx(txs[3]))

	hashes := []util.Uint256{txs[0].Hash(), txs[1].Hash(), txs[2].Hash()}

	// Everyone sends a message.
	for i := 0; i < 4; i++ {
		p := new(Payload)
		p.message = &message{}
		// One PrepareRequest and three ChangeViews.
		if i == 1 {
			p.SetType(payload.PrepareRequestType)
			p.SetPayload(&prepareRequest{transactionHashes: hashes})
		} else {
			p.SetType(payload.ChangeViewType)
			p.SetPayload(&changeView{newViewNumber: 1, timestamp: uint64(time.Now().UnixNano() / nsInMs)})
		}
		p.SetHeight(1)
		p.SetValidatorIndex(uint16(i))

		priv, _ := getTestValidator(i)
		require.NoError(t, p.Sign(priv))

		// Skip srv.OnPayload, because the service is not really started.
		srv.dbft.OnReceive(p)
	}
	require.Equal(t, uint8(1), srv.dbft.ViewNumber)
	require.Equal(t, hashes, srv.lastProposal)

	t.Run("new transactions will be proposed in case of failure", func(t *testing.T) {
		txx := srv.getVerifiedTx()
		require.Equal(t, 1, len(txx), "there is only 1 tx in mempool")
		require.Equal(t, txs[3], txx[0])
	})

	t.Run("more than half of the last proposal will be reused", func(t *testing.T) {
		for _, tx := range txs[:2] {
			require.NoError(t, srv.Chain.PoolTx(tx))
		}

		txx := srv.getVerifiedTx()
		require.Contains(t, txx, txs[0])
		require.Contains(t, txx, txs[1])
		require.NotContains(t, txx, txs[2])
	})
	srv.Chain.Close()
}

func TestService_ValidatePayload(t *testing.T) {
	srv := newTestService(t)
	priv, _ := getTestValidator(1)
	p := new(Payload)
	p.message = &message{}

	p.SetPayload(&prepareRequest{})

	t.Run("invalid validator index", func(t *testing.T) {
		p.SetValidatorIndex(11)
		require.NoError(t, p.Sign(priv))

		var ok bool
		require.NotPanics(t, func() { ok = srv.validatePayload(p) })
		require.False(t, ok)
	})

	t.Run("wrong validator index", func(t *testing.T) {
		p.SetValidatorIndex(2)
		require.NoError(t, p.Sign(priv))
		require.False(t, srv.validatePayload(p))
	})

	t.Run("normal case", func(t *testing.T) {
		p.SetValidatorIndex(1)
		require.NoError(t, p.Sign(priv))
		require.True(t, srv.validatePayload(p))
	})
	srv.Chain.Close()
}

func TestService_getTx(t *testing.T) {
	srv := newTestService(t)

	t.Run("transaction in mempool", func(t *testing.T) {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = 1234
		tx.ValidUntilBlock = 1
		addSender(t, tx)
		signTx(t, srv.Chain, tx)
		h := tx.Hash()

		require.Equal(t, nil, srv.getTx(h))

		require.NoError(t, srv.Chain.PoolTx(tx))

		got := srv.getTx(h)
		require.NotNil(t, got)
		require.Equal(t, h, got.Hash())
	})

	t.Run("transaction in local cache", func(t *testing.T) {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
		tx.Nonce = 4321
		tx.ValidUntilBlock = 1
		h := tx.Hash()

		require.Equal(t, nil, srv.getTx(h))

		srv.txx.Add(tx)

		got := srv.getTx(h)
		require.NotNil(t, got)
		require.Equal(t, h, got.Hash())
	})
	srv.Chain.Close()
}

func TestService_PrepareRequest(t *testing.T) {
	srv := newTestServiceWithState(t, true)
	srv.dbft.Start()
	defer srv.dbft.Timer.Stop()

	priv, _ := getTestValidator(1)
	p := new(Payload)
	p.message = &message{}
	p.SetValidatorIndex(1)

	p.SetPayload(&prepareRequest{})
	require.NoError(t, p.Sign(priv))
	require.Error(t, srv.verifyRequest(p), "invalid stateroot setting")

	p.SetPayload(&prepareRequest{stateRootEnabled: true})
	require.NoError(t, p.Sign(priv))
	require.Error(t, srv.verifyRequest(p), "invalid state root")

	sr, err := srv.Chain.GetStateRoot(srv.dbft.BlockIndex - 1)
	require.NoError(t, err)
	p.SetPayload(&prepareRequest{stateRootEnabled: true, stateRoot: sr.Root})
	require.NoError(t, p.Sign(priv))
	require.NoError(t, srv.verifyRequest(p))
}

func TestService_OnPayload(t *testing.T) {
	srv := newTestService(t)
	// This test directly reads things from srv.messages that normally
	// is read by internal goroutine started with Start(). So let's
	// pretend we really did start already.
	srv.started.Store(true)

	priv, _ := getTestValidator(1)
	p := new(Payload)
	p.message = &message{}
	p.SetValidatorIndex(1)
	p.SetPayload(&prepareRequest{})

	// payload is not signed
	srv.OnPayload(p)
	shouldNotReceive(t, srv.messages)
	require.Nil(t, srv.GetPayload(p.Hash()))

	require.NoError(t, p.Sign(priv))
	srv.OnPayload(p)
	shouldReceive(t, srv.messages)
	require.Equal(t, p, srv.GetPayload(p.Hash()))

	// payload has already been received
	srv.OnPayload(p)
	shouldNotReceive(t, srv.messages)
	srv.Chain.Close()
}

func TestVerifyBlock(t *testing.T) {
	srv := newTestService(t)
	defer srv.Chain.Close()

	srv.lastTimestamp = 1
	t.Run("good empty", func(t *testing.T) {
		b := testchain.NewBlock(t, srv.Chain, 1, 0)
		require.True(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("good pooled tx", func(t *testing.T) {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.RET)}, 100000)
		tx.ValidUntilBlock = 1
		addSender(t, tx)
		signTx(t, srv.Chain, tx)
		require.NoError(t, srv.Chain.PoolTx(tx))
		b := testchain.NewBlock(t, srv.Chain, 1, 0, tx)
		require.True(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("good non-pooled tx", func(t *testing.T) {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.RET)}, 100000)
		tx.ValidUntilBlock = 1
		addSender(t, tx)
		signTx(t, srv.Chain, tx)
		b := testchain.NewBlock(t, srv.Chain, 1, 0, tx)
		require.True(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("good conflicting tx", func(t *testing.T) {
		tx1 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.RET)}, 100000)
		tx1.NetworkFee = 20_000_000 * native.GASFactor
		tx1.ValidUntilBlock = 1
		addSender(t, tx1)
		signTx(t, srv.Chain, tx1)
		tx2 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.RET)}, 100000)
		tx2.NetworkFee = 20_000_000 * native.GASFactor
		tx2.ValidUntilBlock = 1
		addSender(t, tx2)
		signTx(t, srv.Chain, tx2)
		require.NoError(t, srv.Chain.PoolTx(tx1))
		require.Error(t, srv.Chain.PoolTx(tx2))
		b := testchain.NewBlock(t, srv.Chain, 1, 0, tx2)
		require.True(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("bad old", func(t *testing.T) {
		b := testchain.NewBlock(t, srv.Chain, 1, 0)
		b.Index = srv.Chain.BlockHeight()
		require.False(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("bad timestamp", func(t *testing.T) {
		b := testchain.NewBlock(t, srv.Chain, 1, 0)
		b.Timestamp = srv.lastTimestamp - 1
		require.False(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("bad big size", func(t *testing.T) {
		script := make([]byte, int(srv.Chain.GetPolicer().GetMaxBlockSize()))
		script[0] = byte(opcode.RET)
		tx := transaction.New(netmode.UnitTestNet, script, 100000)
		tx.ValidUntilBlock = 1
		addSender(t, tx)
		signTx(t, srv.Chain, tx)
		b := testchain.NewBlock(t, srv.Chain, 1, 0, tx)
		require.False(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("bad tx", func(t *testing.T) {
		tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.RET)}, 100000)
		tx.ValidUntilBlock = 1
		addSender(t, tx)
		signTx(t, srv.Chain, tx)
		tx.Scripts[0].InvocationScript[16] = ^tx.Scripts[0].InvocationScript[16]
		b := testchain.NewBlock(t, srv.Chain, 1, 0, tx)
		require.False(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
	t.Run("bad big sys fee", func(t *testing.T) {
		txes := make([]*transaction.Transaction, 2)
		for i := range txes {
			txes[i] = transaction.New(netmode.UnitTestNet, []byte{byte(opcode.RET)}, srv.Chain.GetPolicer().GetMaxBlockSystemFee()/2+1)
			txes[i].ValidUntilBlock = 1
			addSender(t, txes[i])
			signTx(t, srv.Chain, txes[i])
		}
		b := testchain.NewBlock(t, srv.Chain, 1, 0, txes...)
		require.False(t, srv.verifyBlock(&neoBlock{Block: *b}))
	})
}

func shouldReceive(t *testing.T, ch chan Payload) {
	select {
	case <-ch:
	default:
		require.Fail(t, "missing expected message")
	}
}

func shouldNotReceive(t *testing.T, ch chan Payload) {
	select {
	case <-ch:
		require.Fail(t, "unexpected message receive")
	default:
	}
}

func newTestServiceWithState(t *testing.T, stateRootInHeader bool) *service {
	return newTestServiceWithChain(t, newTestChain(t, stateRootInHeader))
}

func newTestService(t *testing.T) *service {
	return newTestServiceWithState(t, false)
}

func newTestServiceWithChain(t *testing.T, bc *core.Blockchain) *service {
	srv, err := NewService(Config{
		Logger:       zaptest.NewLogger(t),
		Broadcast:    func(*Payload) {},
		Chain:        bc,
		RequestTx:    func(...util.Uint256) {},
		TimePerBlock: time.Duration(bc.GetConfig().SecondsPerBlock) * time.Second,
		Wallet: &config.Wallet{
			Path:     "./testdata/wallet1.json",
			Password: "one",
		},
	})
	require.NoError(t, err)

	return srv.(*service)
}

func getTestValidator(i int) (*privateKey, *publicKey) {
	key := testchain.PrivateKey(i)
	return &privateKey{PrivateKey: key}, &publicKey{PublicKey: key.PublicKey()}
}

func newSingleTestChain(t *testing.T, f func(*config.Config)) *core.Blockchain {
	configPath := "../../config/protocol.unit_testnet.single.yml"
	cfg, err := config.LoadFile(configPath)
	require.NoError(t, err, "could not load config")
	if f != nil {
		f(&cfg)
	}

	chain, err := core.NewBlockchain(storage.NewMemoryStore(), cfg.ProtocolConfiguration, zaptest.NewLogger(t))
	require.NoError(t, err, "could not create chain")

	go chain.Run()
	return chain
}

func newTestChain(t *testing.T, stateRootInHeader bool) *core.Blockchain {
	unitTestNetCfg, err := config.Load("../../config", netmode.UnitTestNet)
	require.NoError(t, err)
	unitTestNetCfg.ProtocolConfiguration.StateRootInHeader = stateRootInHeader

	chain, err := core.NewBlockchain(storage.NewMemoryStore(), unitTestNetCfg.ProtocolConfiguration, zaptest.NewLogger(t))
	require.NoError(t, err)

	go chain.Run()

	return chain
}

var neoOwner = testchain.MultisigScriptHash()

func addSender(t *testing.T, txs ...*transaction.Transaction) {
	for _, tx := range txs {
		tx.Signers = []transaction.Signer{
			{
				Account: neoOwner,
			},
		}
	}
}

func signTx(t *testing.T, bc blockchainer.Blockchainer, txs ...*transaction.Transaction) {
	validators := make([]*keys.PublicKey, 4)
	privNetKeys := make([]*keys.PrivateKey, 4)
	for i := 0; i < 4; i++ {
		privNetKeys[i] = testchain.PrivateKey(i)
		validators[i] = privNetKeys[i].PublicKey()
	}
	privNetKeys = privNetKeys[:3]
	rawScript, err := smartcontract.CreateMultiSigRedeemScript(3, validators)
	require.NoError(t, err)
	for _, tx := range txs {
		size := io.GetVarSize(tx)
		netFee, sizeDelta := fee.Calculate(bc.GetPolicer().GetBaseExecFee(), rawScript)
		tx.NetworkFee += +netFee
		size += sizeDelta
		tx.NetworkFee += int64(size) * bc.FeePerByte()
		data := tx.GetSignedPart()

		buf := io.NewBufBinWriter()
		for _, key := range privNetKeys {
			signature := key.Sign(data)
			emit.Bytes(buf.BinWriter, signature)
		}

		tx.Scripts = []transaction.Witness{{
			InvocationScript:   buf.Bytes(),
			VerificationScript: rawScript,
		}}
	}
}
