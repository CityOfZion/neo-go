package consensus

import (
	"errors"
	"sort"
	"time"

	"github.com/nspcc-dev/dbft"
	"github.com/nspcc-dev/dbft/block"
	"github.com/nspcc-dev/dbft/crypto"
	"github.com/nspcc-dev/dbft/payload"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	coreb "github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

// cacheMaxCapacity is the default cache capacity taken
// from C# implementation https://github.com/neo-project/neo/blob/master/neo/Ledger/Blockchain.cs#L64
const cacheMaxCapacity = 100

// defaultTimePerBlock is a period between blocks which is used in NEO.
const defaultTimePerBlock = 15 * time.Second

// Number of nanoseconds in millisecond.
const nsInMs = 1000000

// Service represents consensus instance.
type Service interface {
	// Start initializes dBFT and starts event loop for consensus service.
	// It must be called only when sufficient amount of peers are connected.
	Start()
	// Shutdown stops dBFT event loop.
	Shutdown()

	// OnPayload is a callback to notify Service about new received payload.
	OnPayload(p *Payload)
	// OnTransaction is a callback to notify Service about new received transaction.
	OnTransaction(tx *transaction.Transaction)
	// GetPayload returns Payload with specified hash if it is present in the local cache.
	GetPayload(h util.Uint256) *Payload
}

type service struct {
	Config

	log *zap.Logger
	// cache is a fifo cache which stores recent payloads.
	cache *relayCache
	// txx is a fifo cache which stores miner transactions.
	txx  *relayCache
	dbft *dbft.DBFT
	// messages and transactions are channels needed to process
	// everything in single thread.
	messages     chan Payload
	transactions chan *transaction.Transaction
	// blockEvents is used to pass a new block event to the consensus
	// process.
	blockEvents  chan *coreb.Block
	lastProposal []util.Uint256
	wallet       *wallet.Wallet
	network      netmode.Magic
	// started is a flag set with Start method that runs an event handling
	// goroutine.
	started  *atomic.Bool
	quit     chan struct{}
	finished chan struct{}
}

// Config is a configuration for consensus services.
type Config struct {
	// Logger is a logger instance.
	Logger *zap.Logger
	// Broadcast is a callback which is called to notify server
	// about new consensus payload to sent.
	Broadcast func(p *Payload)
	// Chain is a core.Blockchainer instance.
	Chain blockchainer.Blockchainer
	// RequestTx is a callback to which will be called
	// when a node lacks transactions present in a block.
	RequestTx func(h ...util.Uint256)
	// TimePerBlock minimal time that should pass before next block is accepted.
	TimePerBlock time.Duration
	// Wallet is a local-node wallet configuration.
	Wallet *config.Wallet
}

// NewService returns new consensus.Service instance.
func NewService(cfg Config) (Service, error) {
	if cfg.TimePerBlock <= 0 {
		cfg.TimePerBlock = defaultTimePerBlock
	}

	if cfg.Logger == nil {
		return nil, errors.New("empty logger")
	}

	srv := &service{
		Config: cfg,

		log:      cfg.Logger,
		cache:    newFIFOCache(cacheMaxCapacity),
		txx:      newFIFOCache(cacheMaxCapacity),
		messages: make(chan Payload, 100),

		transactions: make(chan *transaction.Transaction, 100),
		blockEvents:  make(chan *coreb.Block, 1),
		network:      cfg.Chain.GetConfig().Magic,
		started:      atomic.NewBool(false),
		quit:         make(chan struct{}),
		finished:     make(chan struct{}),
	}

	if cfg.Wallet == nil {
		return srv, nil
	}

	var err error

	if srv.wallet, err = wallet.NewWalletFromFile(cfg.Wallet.Path); err != nil {
		return nil, err
	}

	// Check that wallet password is correct for at least one account.
	var ok bool
	for _, acc := range srv.wallet.Accounts {
		err := acc.Decrypt(srv.Config.Wallet.Password)
		if err == nil {
			ok = true
			break
		}
	}
	if !ok {
		return nil, errors.New("no account with provided password was found")
	}

	defer srv.wallet.Close()

	srv.dbft = dbft.New(
		dbft.WithLogger(srv.log),
		dbft.WithSecondsPerBlock(cfg.TimePerBlock),
		dbft.WithGetKeyPair(srv.getKeyPair),
		dbft.WithRequestTx(cfg.RequestTx),
		dbft.WithGetTx(srv.getTx),
		dbft.WithGetVerified(srv.getVerifiedTx),
		dbft.WithBroadcast(srv.broadcast),
		dbft.WithProcessBlock(srv.processBlock),
		dbft.WithVerifyBlock(srv.verifyBlock),
		dbft.WithGetBlock(srv.getBlock),
		dbft.WithWatchOnly(func() bool { return false }),
		dbft.WithNewBlockFromContext(srv.newBlockFromContext),
		dbft.WithCurrentHeight(cfg.Chain.BlockHeight),
		dbft.WithCurrentBlockHash(cfg.Chain.CurrentBlockHash),
		dbft.WithGetValidators(srv.getValidators),
		dbft.WithGetConsensusAddress(srv.getConsensusAddress),

		dbft.WithNewConsensusPayload(srv.newPayload),
		dbft.WithNewPrepareRequest(func() payload.PrepareRequest { return new(prepareRequest) }),
		dbft.WithNewPrepareResponse(func() payload.PrepareResponse { return new(prepareResponse) }),
		dbft.WithNewChangeView(func() payload.ChangeView { return new(changeView) }),
		dbft.WithNewCommit(func() payload.Commit { return new(commit) }),
		dbft.WithNewRecoveryRequest(func() payload.RecoveryRequest { return new(recoveryRequest) }),
		dbft.WithNewRecoveryMessage(func() payload.RecoveryMessage { return new(recoveryMessage) }),
		dbft.WithVerifyPrepareRequest(srv.verifyRequest),
		dbft.WithVerifyPrepareResponse(func(_ payload.ConsensusPayload) error { return nil }),
	)

	if srv.dbft == nil {
		return nil, errors.New("can't initialize dBFT")
	}

	return srv, nil
}

var (
	_ block.Transaction = (*transaction.Transaction)(nil)
	_ block.Block       = (*neoBlock)(nil)
)

// NewPayload creates new consensus payload for the provided network.
func NewPayload(m netmode.Magic) *Payload {
	return &Payload{
		network: m,
		message: new(message),
	}
}

func (s *service) newPayload() payload.ConsensusPayload {
	return NewPayload(s.network)
}

func (s *service) Start() {
	if s.started.CAS(false, true) {
		s.dbft.Start()
		s.Chain.SubscribeForBlocks(s.blockEvents)
		go s.eventLoop()
	}
}

// Shutdown implements Service interface.
func (s *service) Shutdown() {
	close(s.quit)
	<-s.finished
}

func (s *service) eventLoop() {
events:
	for {
		select {
		case <-s.quit:
			s.dbft.Timer.Stop()
			break events
		case <-s.dbft.Timer.C():
			hv := s.dbft.Timer.HV()
			s.log.Debug("timer fired",
				zap.Uint32("height", hv.Height),
				zap.Uint("view", uint(hv.View)))
			s.dbft.OnTimeout(hv)
		case msg := <-s.messages:
			fields := []zap.Field{
				zap.Uint8("from", msg.validatorIndex),
				zap.Stringer("type", msg.Type()),
			}

			if msg.Type() == payload.RecoveryMessageType {
				rec := msg.GetRecoveryMessage().(*recoveryMessage)
				if rec.preparationHash == nil {
					req := rec.GetPrepareRequest(&msg, s.dbft.Validators, uint16(s.dbft.PrimaryIndex))
					if req != nil {
						h := req.Hash()
						rec.preparationHash = &h
					}
				}

				fields = append(fields,
					zap.Int("#preparation", len(rec.preparationPayloads)),
					zap.Int("#commit", len(rec.commitPayloads)),
					zap.Int("#changeview", len(rec.changeViewPayloads)),
					zap.Bool("#request", rec.prepareRequest != nil),
					zap.Bool("#hash", rec.preparationHash != nil))
			}

			s.log.Debug("received message", fields...)
			s.dbft.OnReceive(&msg)
		case tx := <-s.transactions:
			s.dbft.OnTransaction(tx)
		case b := <-s.blockEvents:
			s.handleChainBlock(b)
		}
		// Always process block event if there is any, we can add one above.
		select {
		case b := <-s.blockEvents:
			s.handleChainBlock(b)
		default:
		}

	}
	close(s.finished)
}

func (s *service) handleChainBlock(b *coreb.Block) {
	// We can get our own block here, so check for index.
	if b.Index >= s.dbft.BlockIndex {
		s.log.Debug("new block in the chain",
			zap.Uint32("dbft index", s.dbft.BlockIndex),
			zap.Uint32("chain index", s.Chain.BlockHeight()))
		s.dbft.InitializeConsensus(0)
	}
}

func (s *service) validatePayload(p *Payload) bool {
	validators := s.getValidators()
	if int(p.validatorIndex) >= len(validators) {
		return false
	}

	pub := validators[p.validatorIndex]
	h := pub.(*publicKey).GetScriptHash()

	return s.Chain.VerifyWitness(h, p, &p.Witness, payloadGasLimit) == nil
}

func (s *service) getKeyPair(pubs []crypto.PublicKey) (int, crypto.PrivateKey, crypto.PublicKey) {
	for i := range pubs {
		sh := pubs[i].(*publicKey).GetScriptHash()
		acc := s.wallet.GetAccount(sh)
		if acc == nil {
			continue
		}

		key := acc.PrivateKey()
		if acc.PrivateKey() == nil {
			err := acc.Decrypt(s.Config.Wallet.Password)
			if err != nil {
				s.log.Fatal("can't unlock account", zap.String("address", address.Uint160ToString(sh)))
				break
			}
			key = acc.PrivateKey()
		}

		return i, &privateKey{PrivateKey: key}, &publicKey{PublicKey: key.PublicKey()}
	}

	return -1, nil, nil
}

// OnPayload handles Payload receive.
func (s *service) OnPayload(cp *Payload) {
	log := s.log.With(zap.Stringer("hash", cp.Hash()))
	if s.cache.Has(cp.Hash()) {
		log.Debug("payload is already in cache")
		return
	} else if !s.validatePayload(cp) {
		log.Debug("can't validate payload")
		return
	}

	s.Config.Broadcast(cp)
	s.cache.Add(cp)

	if s.dbft == nil || !s.started.Load() {
		log.Debug("dbft is inactive or not started yet")
		return
	}

	// decode payload data into message
	if cp.message.payload == nil {
		if err := cp.decodeData(); err != nil {
			log.Debug("can't decode payload data")
			return
		}
	}

	s.messages <- *cp
}

func (s *service) OnTransaction(tx *transaction.Transaction) {
	if s.dbft != nil {
		s.transactions <- tx
	}
}

// GetPayload returns payload stored in cache.
func (s *service) GetPayload(h util.Uint256) *Payload {
	p := s.cache.Get(h)
	if p == nil {
		return (*Payload)(nil)
	}

	cp := *p.(*Payload)

	return &cp
}

func (s *service) broadcast(p payload.ConsensusPayload) {
	if err := p.(*Payload).Sign(s.dbft.Priv.(*privateKey)); err != nil {
		s.log.Warn("can't sign consensus payload", zap.Error(err))
	}

	s.cache.Add(p)
	s.Config.Broadcast(p.(*Payload))
}

func (s *service) getTx(h util.Uint256) block.Transaction {
	if tx := s.txx.Get(h); tx != nil {
		return tx.(*transaction.Transaction)
	}

	tx, _, _ := s.Config.Chain.GetTransaction(h)

	// this is needed because in case of absent tx dBFT expects to
	// get nil interface, not a nil pointer to any concrete type
	if tx != nil {
		return tx
	}

	return nil
}

func (s *service) verifyBlock(b block.Block) bool {
	coreb := &b.(*neoBlock).Block

	if s.Chain.BlockHeight() >= coreb.Index {
		s.log.Warn("proposed block has already outdated")
		return false
	}
	maxBlockSize := int(s.Chain.GetMaxBlockSize())
	size := io.GetVarSize(coreb)
	if size > maxBlockSize {
		s.log.Warn("proposed block size exceeds policy max block size",
			zap.Int("max size allowed", maxBlockSize),
			zap.Int("block size", size))
		return false
	}

	var fee int64
	var pool = mempool.New(len(coreb.Transactions))
	var mainPool = s.Chain.GetMemPool()
	for _, tx := range coreb.Transactions {
		var err error

		fee += tx.SystemFee
		if mainPool.ContainsKey(tx.Hash()) {
			err = pool.Add(tx, s.Chain)
			if err == nil {
				continue
			}
		} else {
			err = s.Chain.PoolTx(tx, pool)
		}
		if err != nil {
			s.log.Warn("invalid transaction in proposed block",
				zap.Stringer("hash", tx.Hash()),
				zap.Error(err))
			return false
		}
		if s.Chain.BlockHeight() >= coreb.Index {
			s.log.Warn("proposed block has already outdated")
			return false
		}
	}

	maxBlockSysFee := s.Chain.GetMaxBlockSystemFee()
	if fee > maxBlockSysFee {
		s.log.Warn("proposed block system fee exceeds policy max block system fee",
			zap.Int("max system fee allowed", int(maxBlockSysFee)),
			zap.Int("block system fee", int(fee)))
		return false
	}

	return true
}

func (s *service) verifyRequest(p payload.ConsensusPayload) error {
	req := p.GetPrepareRequest().(*prepareRequest)
	// Save lastProposal for getVerified().
	s.lastProposal = req.transactionHashes

	return nil
}

func (s *service) processBlock(b block.Block) {
	bb := &b.(*neoBlock).Block
	bb.Script = *(s.getBlockWitness(bb))

	if err := s.Chain.AddBlock(bb); err != nil {
		// The block might already be added via the regular network
		// interaction.
		if _, errget := s.Chain.GetBlock(bb.Hash()); errget != nil {
			s.log.Warn("error on add block", zap.Error(err))
		}
	}
}

func (s *service) getBlockWitness(b *coreb.Block) *transaction.Witness {
	dctx := s.dbft.Context
	pubs := convertKeys(dctx.Validators)
	sigs := make(map[*keys.PublicKey][]byte)

	for i := range pubs {
		if p := dctx.CommitPayloads[i]; p != nil && p.ViewNumber() == dctx.ViewNumber {
			sigs[pubs[i]] = p.GetCommit().Signature()
		}
	}

	m := s.dbft.Context.M()
	verif, err := smartcontract.CreateMultiSigRedeemScript(m, pubs)
	if err != nil {
		s.log.Warn("can't create multisig redeem script", zap.Error(err))
		return nil
	}

	sort.Sort(keys.PublicKeys(pubs))

	buf := io.NewBufBinWriter()
	for i, j := 0, 0; i < len(pubs) && j < m; i++ {
		if sig, ok := sigs[pubs[i]]; ok {
			emit.Bytes(buf.BinWriter, sig)
			j++
		}
	}

	return &transaction.Witness{
		InvocationScript:   buf.Bytes(),
		VerificationScript: verif,
	}
}

func (s *service) getBlock(h util.Uint256) block.Block {
	b, err := s.Chain.GetBlock(h)
	if err != nil {
		return nil
	}

	return &neoBlock{Block: *b}
}

func (s *service) getVerifiedTx() []block.Transaction {
	pool := s.Config.Chain.GetMemPool()

	var txx []*transaction.Transaction

	if s.dbft.ViewNumber > 0 {
		txx = make([]*transaction.Transaction, 0, len(s.lastProposal))
		for i := range s.lastProposal {
			if tx, ok := pool.TryGetValue(s.lastProposal[i]); ok {
				txx = append(txx, tx)
			}
		}

		if len(txx) < len(s.lastProposal)/2 {
			txx = pool.GetVerifiedTransactions()
		}
	} else {
		txx = pool.GetVerifiedTransactions()
	}

	if len(txx) > 0 {
		txx = s.Config.Chain.ApplyPolicyToTxSet(txx)
	}

	res := make([]block.Transaction, len(txx))
	for i := range txx {
		res[i] = txx[i]
	}

	return res
}

func (s *service) getValidators(txes ...block.Transaction) []crypto.PublicKey {
	var (
		pKeys []*keys.PublicKey
		err   error
	)
	if txes == nil {
		pKeys, err = s.Chain.GetNextBlockValidators()
	} else {
		pKeys, err = s.Chain.GetValidators()
	}
	if err != nil {
		s.log.Error("error while trying to get validators", zap.Error(err))
	}

	pubs := make([]crypto.PublicKey, len(pKeys))
	for i := range pKeys {
		pubs[i] = &publicKey{PublicKey: pKeys[i]}
	}

	return pubs
}

func (s *service) getConsensusAddress(validators ...crypto.PublicKey) util.Uint160 {
	return util.Uint160{}
}

func convertKeys(validators []crypto.PublicKey) (pubs []*keys.PublicKey) {
	pubs = make([]*keys.PublicKey, len(validators))
	for i, k := range validators {
		pubs[i] = k.(*publicKey).PublicKey
	}

	return
}

func (s *service) newBlockFromContext(ctx *dbft.Context) block.Block {
	block := new(neoBlock)
	if ctx.TransactionHashes == nil {
		return nil
	}

	block.Block.Network = s.network
	block.Block.Timestamp = ctx.Timestamp / nsInMs
	block.Block.Index = ctx.BlockIndex

	validators, err := s.Chain.GetValidators()
	if err != nil {
		return nil
	}
	script, err := smartcontract.CreateMultiSigRedeemScript(s.dbft.Context.M(), validators)
	if err != nil {
		return nil
	}
	block.Block.NextConsensus = crypto.Hash160(script)
	block.Block.PrevHash = ctx.PrevHash
	block.Block.Version = ctx.Version
	block.Block.ConsensusData.Nonce = ctx.Nonce

	primaryIndex := uint32(ctx.PrimaryIndex)
	block.Block.ConsensusData.PrimaryIndex = primaryIndex

	hashes := make([]util.Uint256, len(ctx.TransactionHashes)+1)
	hashes[0] = block.Block.ConsensusData.Hash()
	copy(hashes[1:], ctx.TransactionHashes)
	block.Block.MerkleRoot = hash.CalcMerkleRoot(hashes)

	return block
}
