package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/rpc"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type (
	// Server represents the JSON-RPC 2.0 server.
	Server struct {
		*http.Server
		chain      blockchainer.Blockchainer
		config     rpc.Config
		coreServer *network.Server
		log        *zap.Logger
		https      *http.Server
	}
)

const (
	// Message limit for receiving side.
	wsReadLimit = 4096

	// Disconnection timeout.
	wsPongLimit = 60 * time.Second

	// Ping period for connection liveness check.
	wsPingPeriod = wsPongLimit / 2

	// Write deadline.
	wsWriteLimit = wsPingPeriod / 2
)

var rpcHandlers = map[string]func(*Server, request.Params) (interface{}, *response.Error){
	"getaccountstate":      (*Server).getAccountState,
	"getapplicationlog":    (*Server).getApplicationLog,
	"getassetstate":        (*Server).getAssetState,
	"getbestblockhash":     (*Server).getBestBlockHash,
	"getblock":             (*Server).getBlock,
	"getblockcount":        (*Server).getBlockCount,
	"getblockhash":         (*Server).getBlockHash,
	"getblockheader":       (*Server).getBlockHeader,
	"getblocksysfee":       (*Server).getBlockSysFee,
	"getclaimable":         (*Server).getClaimable,
	"getconnectioncount":   (*Server).getConnectionCount,
	"getcontractstate":     (*Server).getContractState,
	"getnep5balances":      (*Server).getNEP5Balances,
	"getnep5transfers":     (*Server).getNEP5Transfers,
	"getpeers":             (*Server).getPeers,
	"getrawmempool":        (*Server).getRawMempool,
	"getrawtransaction":    (*Server).getrawtransaction,
	"getstorage":           (*Server).getStorage,
	"gettransactionheight": (*Server).getTransactionHeight,
	"gettxout":             (*Server).getTxOut,
	"getunclaimed":         (*Server).getUnclaimed,
	"getunspents":          (*Server).getUnspents,
	"getvalidators":        (*Server).getValidators,
	"getversion":           (*Server).getVersion,
	"invoke":               (*Server).invoke,
	"invokefunction":       (*Server).invokeFunction,
	"invokescript":         (*Server).invokescript,
	"sendrawtransaction":   (*Server).sendrawtransaction,
	"submitblock":          (*Server).submitBlock,
	"validateaddress":      (*Server).validateAddress,
}

var invalidBlockHeightError = func(index int, height int) *response.Error {
	return response.NewRPCError(fmt.Sprintf("Param at index %d should be greater than or equal to 0 and less then or equal to current block height, got: %d", index, height), "", nil)
}

// upgrader is a no-op websocket.Upgrader that reuses HTTP server buffers and
// doesn't set any Error function.
var upgrader = websocket.Upgrader{}

// New creates a new Server struct.
func New(chain blockchainer.Blockchainer, conf rpc.Config, coreServer *network.Server, log *zap.Logger) Server {
	httpServer := &http.Server{
		Addr: conf.Address + ":" + strconv.FormatUint(uint64(conf.Port), 10),
	}

	var tlsServer *http.Server
	if cfg := conf.TLSConfig; cfg.Enabled {
		tlsServer = &http.Server{
			Addr: net.JoinHostPort(cfg.Address, strconv.FormatUint(uint64(cfg.Port), 10)),
		}
	}

	return Server{
		Server:     httpServer,
		chain:      chain,
		config:     conf,
		coreServer: coreServer,
		log:        log,
		https:      tlsServer,
	}
}

// Start creates a new JSON-RPC server
// listening on the configured port.
func (s *Server) Start(errChan chan error) {
	if !s.config.Enabled {
		s.log.Info("RPC server is not enabled")
		return
	}
	s.Handler = http.HandlerFunc(s.handleHTTPRequest)
	s.log.Info("starting rpc-server", zap.String("endpoint", s.Addr))

	if cfg := s.config.TLSConfig; cfg.Enabled {
		s.https.Handler = http.HandlerFunc(s.handleHTTPRequest)
		s.log.Info("starting rpc-server (https)", zap.String("endpoint", s.https.Addr))
		go func() {
			err := s.https.ListenAndServeTLS(cfg.CertFile, cfg.KeyFile)
			if err != nil {
				s.log.Error("failed to start TLS RPC server", zap.Error(err))
			}
			errChan <- err
		}()
	}
	err := s.ListenAndServe()
	if err != nil {
		s.log.Error("failed to start RPC server", zap.Error(err))
	}
	errChan <- err
}

// Shutdown overrides the http.Server Shutdown
// method.
func (s *Server) Shutdown() error {
	var httpsErr error
	if s.config.TLSConfig.Enabled {
		s.log.Info("shutting down rpc-server (https)", zap.String("endpoint", s.https.Addr))
		httpsErr = s.https.Shutdown(context.Background())
	}

	s.log.Info("shutting down rpc-server", zap.String("endpoint", s.Addr))
	err := s.Server.Shutdown(context.Background())
	if err == nil {
		return httpsErr
	}
	return err
}

func (s *Server) handleHTTPRequest(w http.ResponseWriter, httpRequest *http.Request) {
	if httpRequest.URL.Path == "/ws" && httpRequest.Method == "GET" {
		ws, err := upgrader.Upgrade(w, httpRequest, nil)
		if err != nil {
			s.log.Info("websocket connection upgrade failed", zap.Error(err))
			return
		}
		resChan := make(chan response.Raw)
		go s.handleWsWrites(ws, resChan)
		s.handleWsReads(ws, resChan)
		return
	}

	req := request.NewIn()

	if httpRequest.Method != "POST" {
		s.writeHTTPErrorResponse(
			req,
			w,
			response.NewInvalidParamsError(
				fmt.Sprintf("Invalid method '%s', please retry with 'POST'", httpRequest.Method), nil,
			),
		)
		return
	}

	err := req.DecodeData(httpRequest.Body)
	if err != nil {
		s.writeHTTPErrorResponse(req, w, response.NewParseError("Problem parsing JSON-RPC request body", err))
		return
	}

	resp := s.handleRequest(req)
	s.writeHTTPServerResponse(req, w, resp)
}

func (s *Server) handleRequest(req *request.In) response.Raw {
	reqParams, err := req.Params()
	if err != nil {
		return s.packResponseToRaw(req, nil, response.NewInvalidParamsError("Problem parsing request parameters", err))
	}

	s.log.Debug("processing rpc request",
		zap.String("method", req.Method),
		zap.String("params", fmt.Sprintf("%v", reqParams)))

	incCounter(req.Method)

	handler, ok := rpcHandlers[req.Method]
	if !ok {
		return s.packResponseToRaw(req, nil, response.NewMethodNotFoundError(fmt.Sprintf("Method '%s' not supported", req.Method), nil))
	}
	res, resErr := handler(s, *reqParams)
	return s.packResponseToRaw(req, res, resErr)
}

func (s *Server) handleWsWrites(ws *websocket.Conn, resChan <-chan response.Raw) {
	pingTicker := time.NewTicker(wsPingPeriod)
	defer ws.Close()
	defer pingTicker.Stop()
	for {
		select {
		case res, ok := <-resChan:
			if !ok {
				return
			}
			ws.SetWriteDeadline(time.Now().Add(wsWriteLimit))
			if err := ws.WriteJSON(res); err != nil {
				return
			}
		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(wsWriteLimit))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func (s *Server) handleWsReads(ws *websocket.Conn, resChan chan<- response.Raw) {
	ws.SetReadLimit(wsReadLimit)
	ws.SetReadDeadline(time.Now().Add(wsPongLimit))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(wsPongLimit)); return nil })
	for {
		req := new(request.In)
		err := ws.ReadJSON(req)
		if err != nil {
			break
		}
		res := s.handleRequest(req)
		if res.Error != nil {
			s.logRequestError(req, res.Error)
		}
		resChan <- res
	}
	close(resChan)
	ws.Close()
}

func (s *Server) getBestBlockHash(_ request.Params) (interface{}, *response.Error) {
	return "0x" + s.chain.CurrentBlockHash().StringLE(), nil
}

func (s *Server) getBlockCount(_ request.Params) (interface{}, *response.Error) {
	return s.chain.BlockHeight() + 1, nil
}

func (s *Server) getConnectionCount(_ request.Params) (interface{}, *response.Error) {
	return s.coreServer.PeerCount(), nil
}

func (s *Server) getBlock(reqParams request.Params) (interface{}, *response.Error) {
	var hash util.Uint256

	param, ok := reqParams.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	switch param.Type {
	case request.StringT:
		var err error
		hash, err = param.GetUint256()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
	case request.NumberT:
		num, err := s.blockHeightFromParam(param)
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		hash = s.chain.GetHeaderHash(num)
	default:
		return nil, response.ErrInvalidParams
	}

	block, err := s.chain.GetBlock(hash)
	if err != nil {
		return nil, response.NewInternalServerError(fmt.Sprintf("Problem locating block with hash: %s", hash), err)
	}

	if len(reqParams) == 2 && reqParams[1].Value == 1 {
		return result.NewBlock(block, s.chain), nil
	}
	writer := io.NewBufBinWriter()
	block.EncodeBinary(writer.BinWriter)
	return hex.EncodeToString(writer.Bytes()), nil
}

func (s *Server) getBlockHash(reqParams request.Params) (interface{}, *response.Error) {
	param, ok := reqParams.ValueWithType(0, request.NumberT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	num, err := s.blockHeightFromParam(param)
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	return s.chain.GetHeaderHash(num), nil
}

func (s *Server) getVersion(_ request.Params) (interface{}, *response.Error) {
	return result.Version{
		Port:      s.coreServer.Port,
		Nonce:     s.coreServer.ID(),
		UserAgent: s.coreServer.UserAgent,
	}, nil
}

func (s *Server) getPeers(_ request.Params) (interface{}, *response.Error) {
	peers := result.NewGetPeers()
	peers.AddUnconnected(s.coreServer.UnconnectedPeers())
	peers.AddConnected(s.coreServer.ConnectedPeers())
	peers.AddBad(s.coreServer.BadPeers())
	return peers, nil
}

func (s *Server) getRawMempool(_ request.Params) (interface{}, *response.Error) {
	mp := s.chain.GetMemPool()
	hashList := make([]util.Uint256, 0)
	for _, item := range mp.GetVerifiedTransactions() {
		hashList = append(hashList, item.Tx.Hash())
	}
	return hashList, nil
}

func (s *Server) validateAddress(reqParams request.Params) (interface{}, *response.Error) {
	param, ok := reqParams.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	return validateAddress(param.Value), nil
}

func (s *Server) getAssetState(reqParams request.Params) (interface{}, *response.Error) {
	param, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	paramAssetID, err := param.GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	as := s.chain.GetAssetState(paramAssetID)
	if as != nil {
		return result.NewAssetState(as), nil
	}
	return nil, response.NewRPCError("Unknown asset", "", nil)
}

// getApplicationLog returns the contract log based on the specified txid.
func (s *Server) getApplicationLog(reqParams request.Params) (interface{}, *response.Error) {
	param, ok := reqParams.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	txHash, err := param.GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	appExecResult, err := s.chain.GetAppExecResult(txHash)
	if err != nil {
		return nil, response.NewRPCError("Unknown transaction", "", nil)
	}

	tx, _, err := s.chain.GetTransaction(txHash)
	if err != nil {
		return nil, response.NewRPCError("Error while getting transaction", "", nil)
	}

	var scriptHash util.Uint160
	switch t := tx.Data.(type) {
	case *transaction.InvocationTX:
		scriptHash = hash.Hash160(t.Script)
	default:
		return nil, response.NewRPCError("Invalid transaction type", "", nil)
	}

	return result.NewApplicationLog(appExecResult, scriptHash), nil
}

func (s *Server) getClaimable(ps request.Params) (interface{}, *response.Error) {
	p, ok := ps.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	u, err := p.GetUint160FromAddress()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	var unclaimed []state.UnclaimedBalance
	if acc := s.chain.GetAccountState(u); acc != nil {
		err := acc.Unclaimed.ForEach(func(b *state.UnclaimedBalance) error {
			unclaimed = append(unclaimed, *b)
			return nil
		})
		if err != nil {
			return nil, response.NewInternalServerError("Unclaimed processing failure", err)
		}
	}

	var sum util.Fixed8
	claimable := make([]result.Claimable, 0, len(unclaimed))
	for _, ub := range unclaimed {
		gen, sys, err := s.chain.CalculateClaimable(ub.Value, ub.Start, ub.End)
		if err != nil {
			s.log.Info("error while calculating claim bonus", zap.Error(err))
			continue
		}

		uc := gen.Add(sys)
		sum += uc

		claimable = append(claimable, result.Claimable{
			Tx:          ub.Tx,
			N:           int(ub.Index),
			Value:       ub.Value,
			StartHeight: ub.Start,
			EndHeight:   ub.End,
			Generated:   gen,
			SysFee:      sys,
			Unclaimed:   uc,
		})
	}

	return result.ClaimableInfo{
		Spents:    claimable,
		Address:   p.String(),
		Unclaimed: sum,
	}, nil
}

func (s *Server) getNEP5Balances(ps request.Params) (interface{}, *response.Error) {
	p, ok := ps.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	u, err := p.GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	as := s.chain.GetNEP5Balances(u)
	bs := &result.NEP5Balances{
		Address:  address.Uint160ToString(u),
		Balances: []result.NEP5Balance{},
	}
	if as != nil {
		cache := make(map[util.Uint160]int64)
		for h, bal := range as.Trackers {
			dec, err := s.getDecimals(h, cache)
			if err != nil {
				continue
			}
			amount := amountToString(bal.Balance, dec)
			bs.Balances = append(bs.Balances, result.NEP5Balance{
				Asset:       h,
				Amount:      amount,
				LastUpdated: bal.LastUpdatedBlock,
			})
		}
	}
	return bs, nil
}

func (s *Server) getNEP5Transfers(ps request.Params) (interface{}, *response.Error) {
	p, ok := ps.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	u, err := p.GetUint160FromAddress()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	bs := &result.NEP5Transfers{
		Address:  address.Uint160ToString(u),
		Received: []result.NEP5Transfer{},
		Sent:     []result.NEP5Transfer{},
	}
	lg := s.chain.GetNEP5TransferLog(u)
	cache := make(map[util.Uint160]int64)
	err = lg.ForEach(func(tr *state.NEP5Transfer) error {
		transfer := result.NEP5Transfer{
			Timestamp: tr.Timestamp,
			Asset:     tr.Asset,
			Index:     tr.Block,
			TxHash:    tr.Tx,
		}
		d, err := s.getDecimals(tr.Asset, cache)
		if err != nil {
			return nil
		}
		if tr.Amount > 0 { // token was received
			transfer.Amount = amountToString(tr.Amount, d)
			if !tr.From.Equals(util.Uint160{}) {
				transfer.Address = address.Uint160ToString(tr.From)
			}
			bs.Received = append(bs.Received, transfer)
			return nil
		}

		transfer.Amount = amountToString(-tr.Amount, d)
		if !tr.From.Equals(util.Uint160{}) {
			transfer.Address = address.Uint160ToString(tr.To)
		}
		bs.Sent = append(bs.Sent, transfer)
		return nil
	})
	if err != nil {
		return nil, response.NewInternalServerError("invalid NEP5 transfer log", err)
	}
	return bs, nil
}

func amountToString(amount int64, decimals int64) string {
	if decimals == 0 {
		return strconv.FormatInt(amount, 10)
	}
	pow := int64(math.Pow10(int(decimals)))
	q := amount / pow
	r := amount % pow
	if r == 0 {
		return strconv.FormatInt(q, 10)
	}
	fs := fmt.Sprintf("%%d.%%0%dd", decimals)
	return fmt.Sprintf(fs, q, r)
}

func (s *Server) getDecimals(h util.Uint160, cache map[util.Uint160]int64) (int64, *response.Error) {
	if d, ok := cache[h]; ok {
		return d, nil
	}
	script, err := request.CreateFunctionInvocationScript(h, request.Params{
		{
			Type:  request.StringT,
			Value: "decimals",
		},
		{
			Type:  request.ArrayT,
			Value: []request.Param{},
		},
	})
	if err != nil {
		return 0, response.NewInternalServerError("Can't create script", err)
	}
	res := s.runScriptInVM(script)
	if res == nil || res.State != "HALT" || len(res.Stack) == 0 {
		return 0, response.NewInternalServerError("execution error", errors.New("no result"))
	}

	var d int64
	switch item := res.Stack[len(res.Stack)-1]; item.Type {
	case smartcontract.IntegerType:
		d = item.Value.(int64)
	case smartcontract.ByteArrayType:
		d = emit.BytesToInt(item.Value.([]byte)).Int64()
	default:
		return 0, response.NewInternalServerError("invalid result", errors.New("not an integer"))
	}
	if d < 0 {
		return 0, response.NewInternalServerError("incorrect result", errors.New("negative result"))
	}
	cache[h] = d
	return d, nil
}

func (s *Server) getStorage(ps request.Params) (interface{}, *response.Error) {
	param, ok := ps.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	scriptHash, err := param.GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	scriptHash = scriptHash.Reverse()

	param, ok = ps.Value(1)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	key, err := param.GetBytesHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	item := s.chain.GetStorageItem(scriptHash.Reverse(), key)
	if item == nil {
		return nil, nil
	}

	return hex.EncodeToString(item.Value), nil
}

func (s *Server) getrawtransaction(reqParams request.Params) (interface{}, *response.Error) {
	var resultsErr *response.Error
	var results interface{}

	if param0, ok := reqParams.Value(0); !ok {
		return nil, response.ErrInvalidParams
	} else if txHash, err := param0.GetUint256(); err != nil {
		resultsErr = response.ErrInvalidParams
	} else if tx, height, err := s.chain.GetTransaction(txHash); err != nil {
		err = errors.Wrapf(err, "Invalid transaction hash: %s", txHash)
		return nil, response.NewRPCError("Unknown transaction", err.Error(), err)
	} else if len(reqParams) >= 2 {
		_header := s.chain.GetHeaderHash(int(height))
		header, err := s.chain.GetHeader(_header)
		if err != nil {
			resultsErr = response.NewInvalidParamsError(err.Error(), err)
		}

		param1, _ := reqParams.Value(1)
		switch v := param1.Value.(type) {

		case int, float64, bool, string:
			if v == 0 || v == "0" || v == 0.0 || v == false || v == "false" {
				results = hex.EncodeToString(tx.Bytes())
			} else {
				results = result.NewTransactionOutputRaw(tx, header, s.chain)
			}
		default:
			results = result.NewTransactionOutputRaw(tx, header, s.chain)
		}
	} else {
		results = hex.EncodeToString(tx.Bytes())
	}

	return results, resultsErr
}

func (s *Server) getTransactionHeight(ps request.Params) (interface{}, *response.Error) {
	p, ok := ps.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	h, err := p.GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	_, height, err := s.chain.GetTransaction(h)
	if err != nil {
		return nil, response.NewRPCError("unknown transaction", "", nil)
	}

	return height, nil
}

func (s *Server) getTxOut(ps request.Params) (interface{}, *response.Error) {
	p, ok := ps.Value(0)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	h, err := p.GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	p, ok = ps.ValueWithType(1, request.NumberT)
	if !ok {
		return nil, response.ErrInvalidParams
	}

	num, err := p.GetInt()
	if err != nil || num < 0 {
		return nil, response.ErrInvalidParams
	}

	tx, _, err := s.chain.GetTransaction(h)
	if err != nil {
		return nil, response.NewInvalidParamsError(err.Error(), err)
	}

	if num >= len(tx.Outputs) {
		return nil, response.NewInvalidParamsError("invalid index", errors.New("too big index"))
	}

	out := tx.Outputs[num]
	return result.NewTxOutput(&out), nil
}

// getContractState returns contract state (contract information, according to the contract script hash).
func (s *Server) getContractState(reqParams request.Params) (interface{}, *response.Error) {
	var results interface{}

	param, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	} else if scriptHash, err := param.GetUint160FromHex(); err != nil {
		return nil, response.ErrInvalidParams
	} else {
		cs := s.chain.GetContractState(scriptHash)
		if cs != nil {
			results = result.NewContractState(cs)
		} else {
			return nil, response.NewRPCError("Unknown contract", "", nil)
		}
	}
	return results, nil
}

func (s *Server) getAccountState(ps request.Params) (interface{}, *response.Error) {
	return s.getAccountStateAux(ps, false)
}

func (s *Server) getUnspents(ps request.Params) (interface{}, *response.Error) {
	return s.getAccountStateAux(ps, true)
}

// getAccountState returns account state either in short or full (unspents included) form.
func (s *Server) getAccountStateAux(reqParams request.Params, unspents bool) (interface{}, *response.Error) {
	var resultsErr *response.Error
	var results interface{}

	param, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	} else if scriptHash, err := param.GetUint160FromAddress(); err != nil {
		return nil, response.ErrInvalidParams
	} else {
		as := s.chain.GetAccountState(scriptHash)
		if as == nil {
			as = state.NewAccount(scriptHash)
		}
		if unspents {
			str, err := param.GetString()
			if err != nil {
				return nil, response.ErrInvalidParams
			}
			results = result.NewUnspents(as, s.chain, str)
		} else {
			results = result.NewAccountState(as)
		}
	}
	return results, resultsErr
}

// getBlockSysFee returns the system fees of the block, based on the specified index.
func (s *Server) getBlockSysFee(reqParams request.Params) (interface{}, *response.Error) {
	param, ok := reqParams.ValueWithType(0, request.NumberT)
	if !ok {
		return 0, response.ErrInvalidParams
	}

	num, err := s.blockHeightFromParam(param)
	if err != nil {
		return 0, response.NewRPCError("Invalid height", "", nil)
	}

	headerHash := s.chain.GetHeaderHash(num)
	block, errBlock := s.chain.GetBlock(headerHash)
	if errBlock != nil {
		return 0, response.NewRPCError(errBlock.Error(), "", nil)
	}

	var blockSysFee util.Fixed8
	for _, tx := range block.Transactions {
		blockSysFee += tx.SystemFee
	}

	return blockSysFee, nil
}

// getBlockHeader returns the corresponding block header information according to the specified script hash.
func (s *Server) getBlockHeader(reqParams request.Params) (interface{}, *response.Error) {
	var verbose bool

	param, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	hash, err := param.GetUint256()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	param, ok = reqParams.ValueWithType(1, request.NumberT)
	if ok {
		v, err := param.GetInt()
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		verbose = v != 0
	}

	h, err := s.chain.GetHeader(hash)
	if err != nil {
		return nil, response.NewRPCError("unknown block", "", nil)
	}

	if verbose {
		return result.NewHeader(h, s.chain), nil
	}

	buf := io.NewBufBinWriter()
	h.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil, response.NewInternalServerError("encoding error", buf.Err)
	}
	return hex.EncodeToString(buf.Bytes()), nil
}

// getUnclaimed returns unclaimed GAS amount of the specified address.
func (s *Server) getUnclaimed(ps request.Params) (interface{}, *response.Error) {
	p, ok := ps.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	u, err := p.GetUint160FromAddress()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	acc := s.chain.GetAccountState(u)
	if acc == nil {
		return nil, response.NewInternalServerError("unknown account", nil)
	}
	res, errRes := result.NewUnclaimed(acc, s.chain)
	if errRes != nil {
		return nil, response.NewInternalServerError("can't create unclaimed response", errRes)
	}
	return res, nil
}

// getValidators returns the current NEO consensus nodes information and voting status.
func (s *Server) getValidators(_ request.Params) (interface{}, *response.Error) {
	var validators keys.PublicKeys

	validators, err := s.chain.GetValidators()
	if err != nil {
		return nil, response.NewRPCError("can't get validators", "", err)
	}
	enrollments, err := s.chain.GetEnrollments()
	if err != nil {
		return nil, response.NewRPCError("can't get enrollments", "", err)
	}
	var res []result.Validator
	for _, v := range enrollments {
		res = append(res, result.Validator{
			PublicKey: *v.Key,
			Votes:     v.Votes.Int64(),
			Active:    validators.Contains(v.Key),
		})
	}
	return res, nil
}

// invoke implements the `invoke` RPC call.
func (s *Server) invoke(reqParams request.Params) (interface{}, *response.Error) {
	scriptHashHex, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	scriptHash, err := scriptHashHex.GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	sliceP, ok := reqParams.ValueWithType(1, request.ArrayT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	slice, err := sliceP.GetArray()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	script, err := request.CreateInvocationScript(scriptHash, slice)
	if err != nil {
		return nil, response.NewInternalServerError("can't create invocation script", err)
	}
	return s.runScriptInVM(script), nil
}

// invokescript implements the `invokescript` RPC call.
func (s *Server) invokeFunction(reqParams request.Params) (interface{}, *response.Error) {
	scriptHashHex, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	scriptHash, err := scriptHashHex.GetUint160FromHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	script, err := request.CreateFunctionInvocationScript(scriptHash, reqParams[1:])
	if err != nil {
		return nil, response.NewInternalServerError("can't create invocation script", err)
	}
	return s.runScriptInVM(script), nil
}

// invokescript implements the `invokescript` RPC call.
func (s *Server) invokescript(reqParams request.Params) (interface{}, *response.Error) {
	if len(reqParams) < 1 {
		return nil, response.ErrInvalidParams
	}

	script, err := reqParams[0].GetBytesHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}

	return s.runScriptInVM(script), nil
}

// runScriptInVM runs given script in a new test VM and returns the invocation
// result.
func (s *Server) runScriptInVM(script []byte) *result.Invoke {
	vm := s.chain.GetTestVM()
	vm.SetGasLimit(s.config.MaxGasInvoke)
	vm.LoadScript(script)
	_ = vm.Run()
	result := &result.Invoke{
		State:       vm.State(),
		GasConsumed: vm.GasConsumed().String(),
		Script:      hex.EncodeToString(script),
		Stack:       vm.Estack().ToContractParameters(),
	}
	return result
}

// submitBlock broadcasts a raw block over the NEO network.
func (s *Server) submitBlock(reqParams request.Params) (interface{}, *response.Error) {
	param, ok := reqParams.ValueWithType(0, request.StringT)
	if !ok {
		return nil, response.ErrInvalidParams
	}
	blockBytes, err := param.GetBytesHex()
	if err != nil {
		return nil, response.ErrInvalidParams
	}
	b := block.Block{}
	r := io.NewBinReaderFromBuf(blockBytes)
	b.DecodeBinary(r)
	if r.Err != nil {
		return nil, response.ErrInvalidParams
	}
	err = s.chain.AddBlock(&b)
	if err != nil {
		switch err {
		case core.ErrInvalidBlockIndex, core.ErrAlreadyExists:
			return nil, response.ErrAlreadyExists
		default:
			return nil, response.ErrValidationFailed
		}
	}
	return true, nil
}

func (s *Server) sendrawtransaction(reqParams request.Params) (interface{}, *response.Error) {
	var resultsErr *response.Error
	var results interface{}

	if len(reqParams) < 1 {
		return nil, response.ErrInvalidParams
	} else if byteTx, err := reqParams[0].GetBytesHex(); err != nil {
		return nil, response.ErrInvalidParams
	} else {
		tx, err := transaction.NewTransactionFromBytes(byteTx)
		if err != nil {
			return nil, response.ErrInvalidParams
		}
		relayReason := s.coreServer.RelayTxn(tx)
		switch relayReason {
		case network.RelaySucceed:
			results = true
		case network.RelayAlreadyExists:
			resultsErr = response.ErrAlreadyExists
		case network.RelayOutOfMemory:
			resultsErr = response.ErrOutOfMemory
		case network.RelayUnableToVerify:
			resultsErr = response.ErrUnableToVerify
		case network.RelayInvalid:
			resultsErr = response.ErrValidationFailed
		case network.RelayPolicyFail:
			resultsErr = response.ErrPolicyFail
		default:
			resultsErr = response.ErrUnknown
		}
	}

	return results, resultsErr
}

func (s *Server) blockHeightFromParam(param *request.Param) (int, *response.Error) {
	num, err := param.GetInt()
	if err != nil {
		return 0, nil
	}

	if num < 0 || num > int(s.chain.BlockHeight()) {
		return 0, invalidBlockHeightError(0, num)
	}
	return num, nil
}

func (s *Server) packResponseToRaw(r *request.In, result interface{}, respErr *response.Error) response.Raw {
	resp := response.Raw{
		HeaderAndError: response.HeaderAndError{
			Header: response.Header{
				JSONRPC: r.JSONRPC,
				ID:      r.RawID,
			},
		},
	}
	if respErr != nil {
		resp.Error = respErr
	} else {
		resJSON, err := json.Marshal(result)
		if err != nil {
			s.log.Error("failed to marshal result",
				zap.Error(err),
				zap.String("method", r.Method))
			resp.Error = response.NewInternalServerError("failed to encode result", err)
		} else {
			resp.Result = resJSON
		}
	}
	return resp
}

// logRequestError is a request error logger.
func (s *Server) logRequestError(r *request.In, jsonErr *response.Error) {
	logFields := []zap.Field{
		zap.Error(jsonErr.Cause),
		zap.String("method", r.Method),
	}

	params, err := r.Params()
	if err == nil {
		logFields = append(logFields, zap.Any("params", params))
	}

	s.log.Error("Error encountered with rpc request", logFields...)
}

// writeHTTPErrorResponse writes an error response to the ResponseWriter.
func (s *Server) writeHTTPErrorResponse(r *request.In, w http.ResponseWriter, jsonErr *response.Error) {
	resp := s.packResponseToRaw(r, nil, jsonErr)
	s.writeHTTPServerResponse(r, w, resp)
}

func (s *Server) writeHTTPServerResponse(r *request.In, w http.ResponseWriter, resp response.Raw) {
	// Errors can happen in many places and we can only catch ALL of them here.
	if resp.Error != nil {
		s.logRequestError(r, resp.Error)
		w.WriteHeader(resp.Error.HTTPCode)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if s.config.EnableCORSWorkaround {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Access-Control-Allow-Headers, Authorization, X-Requested-With")
	}

	encoder := json.NewEncoder(w)
	err := encoder.Encode(resp)

	if err != nil {
		s.log.Error("Error encountered while encoding response",
			zap.String("err", err.Error()),
			zap.String("method", r.Method))
	}
}

// validateAddress verifies that the address is a correct NEO address
// see https://docs.neo.org/en-us/node/cli/2.9.4/api/validateaddress.html
func validateAddress(addr interface{}) result.ValidateAddress {
	resp := result.ValidateAddress{Address: addr}
	if addr, ok := addr.(string); ok {
		_, err := address.StringToUint160(addr)
		resp.IsValid = (err == nil)
	}
	return resp
}
