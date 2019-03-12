package rpc

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tc struct {
	rpcCall        string
	method         string
	expectedResult string
}

var testRpcCases = []tc{

	{
		rpcCall:        `{"jsonrpc": "2.0", "id": 1, "method": "getassetstate", "params": ["602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"] }`,
		method:         "getassetstate_1",
		expectedResult: `{"jsonrpc":"2.0","result":{"assetId":"0x602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7","assetType":1,"name":"NEOGas","amount":"100000000","available":"0","precision":8,"fee":0,"address":"0x0000000000000000000000000000000000000000","owner":"00","admin":"AWKECj9RD8rS8RPcpCgYVjk1DeYyHwxZm3","issuer":"AFmseVrdL9f9oyCzZefL9tG6UbvhPbdYzM","expiration":0,"is_frozen":false},"id":1}`,
	},

	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getassetstate", "params": ["c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b"] }`,
		method:         "getassetstate_2",
		expectedResult: `{"jsonrpc":"2.0","result":{"assetId":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","assetType":0,"name":"NEO","amount":"100000000","available":"0","precision":0,"fee":0,"address":"0x0000000000000000000000000000000000000000","owner":"00","admin":"Abf2qMs1pzQb8kYk9RuxtUb9jtRKJVuBJt","issuer":"AFmseVrdL9f9oyCzZefL9tG6UbvhPbdYzM","expiration":0,"is_frozen":false},"id":1}`,
	},

	{
		rpcCall:        `{"jsonrpc": "2.0", "id": 1, "method": "getassetstate", "params": ["62c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7"] }`,
		method:         "getassetstate_3",
		expectedResult: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"},"id":1}`,
	},

	{
		rpcCall:        `{"jsonrpc": "2.0", "id": 1, "method": "getassetstate", "params": [123] }`,
		method:         "getassetstate_4",
		expectedResult: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"},"id":1}`,
	},

	{
		rpcCall:        `{"jsonrpc": "2.0", "id": 1, "method": "getblockhash", "params": [10] }`,
		method:         "getblockhash_1",
		expectedResult: `{"jsonrpc":"2.0","result":"0xd69e7a1f62225a35fed91ca578f33447d93fa0fd2b2f662b957e19c38c1dab1e","id":1}`,
	},

	{
		rpcCall:        `{"jsonrpc": "2.0", "id": 1, "method": "getblockhash", "params": [-2] }`,
		method:         "getblockhash_2",
		expectedResult: `{"jsonrpc":"2.0","error":{"code":-32603,"message":"Internal error","data":"Internal server error"},"id":1}`,
	},

	{
		rpcCall:        `{"jsonrpc": "2.0", "id": 1, "method": "getblock", "params": [10] }`,
		method:         "getblock",
		expectedResult: `{"jsonrpc":"2.0","result":{"version":0,"previousblockhash":"0x7c5b4c8a70336bf68e8679be7c9a2a15f85c0f6d0e14389019dcc3edfab2bb4b","merkleroot":"0xc027979ad29226b7d34523b1439a64a6cf57fe3f4e823e9d3e90d43934783d26","time":1529926220,"height":10,"nonce":8313828522725096825,"next_consensus":"0xbe48d3a3f5d10013ab9ffee489706078714f1ea2","script":{"invocation":"40ac828e1c2a214e4d356fd2eccc7c7be9ef426f8e4ea67a50464e90ca4367e611c4c5247082b85a7d5ed985cfb90b9af2f1195531038f49c63fb6894b517071ea40b22b83d9457ca5c4c5bb2d8d7e95333820611d447bb171ce7b8af3b999d0a5a61c2301cdd645a33a47defd09c0f237a0afc86e9a84c2fe675d701e4015c0302240a6899296660c612736edc22f8d630927649d4ef1301868079032d80aae6cc1e21622f256497a84a71d7afeeef4c124135f611db24a0f7ab3d2a6886f15db7865","verification":"532102103a7f7dd016558597f7960d27c516a4394fd968b9e65155eb4b013e4040406e2102a7bc55fe8684e0119768d104ba30795bdcc86619e864add26156723ed185cd622102b3622bf4017bdfe317c58aed5f4c753f206b7db896046fa7d774bbc4bf7f8dc22103d90c07df63e690ce77912e10ab51acc944b66860237b608c4f8f8309e71ee69954ae"},"tx":[{"type":"MinerTransaction","version":0,"attributes":null,"vin":null,"vout":null,"scripts":null}],"confirmations":12338,"nextblockhash":"0x2b1c78633dae7ab81f64362e0828153079a17b018d779d0406491f84c27b086f","hash":"0xd69e7a1f62225a35fed91ca578f33447d93fa0fd2b2f662b957e19c38c1dab1e"},"id":1}`,
	},

	{
		rpcCall:        `{"jsonrpc": "2.0", "id": 1, "method": "getblockcount", "params": [] }`,
		method:         "getblockcount",
		expectedResult: `{"jsonrpc":"2.0","result":12349,"id":1}`,
	},

	{
		rpcCall:        `{"jsonrpc": "2.0", "id": 1, "method": "getconnectioncount", "params": [] }`,
		method:         "getconnectioncount",
		expectedResult: `{"jsonrpc":"2.0","result":0,"id":1}`,
	},

	{
		rpcCall:        `{"jsonrpc": "2.0", "id": 1, "method": "getbestblockhash", "params": [] }`,
		method:         "getbestblockhash",
		expectedResult: `{"jsonrpc":"2.0","result":"877f5f2084181b85ce4726ab0a86bea6cc82cdbcb6f2eb59e6b04d27fd10929c","id":1}`,
	},

	{
		rpcCall:        `{"jsonrpc": "2.0", "id": 1, "method": "getpeers", "params": [] }`,
		method:         "getpeers",
		expectedResult: `{"jsonrpc":"2.0","result":{"unconnected":[],"connected":[],"bad":[]},"id":1}`},

	// Good case, valid transaction ((param[1]=1 -> verbose = 1))
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", 1] }`,
		method:         "getrawtransaction_1",
		expectedResult: `{"jsonrpc":"2.0","result":{"type":"ContractTransaction","version":0,"attributes":[{"data":"23ba2703c53263e8d6e522dc32203339dcd8eee9","usage":"Script"}],"vin":[{"txid":"0x539084697cc220916cb5b16d2805945ec9f267aa004b6688fbf15e116c846aff","vout":0}],"vout":[{"address":"AXpNr3SDfLXbPHNdqxYeHK5cYpKMHZxMZ9","asset":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","n":0,"value":"10000"},{"address":"AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y","asset":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","n":1,"value":"99990000"}],"scripts":[{"invocation":"40a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c6","verification":"21031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac"}],"txid":"0xf999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a","size":283,"sys_fee":"0","net_fee":"0","blockhash":"0x6088bf9d3b55c67184f60b00d2e380228f713b4028b24c1719796dcd2006e417","confirmations":2902,"blocktime":1533756500},"id":1}`,
	},

	// Good case, valid transaction (param[1]=3 -> verbose = 1. Following the C# any number different from 0 is interpreted as verbose = 1)
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", 3] }`,
		method:         "getrawtransaction_2",
		expectedResult: `{"jsonrpc":"2.0","result":{"type":"ContractTransaction","version":0,"attributes":[{"data":"23ba2703c53263e8d6e522dc32203339dcd8eee9","usage":"Script"}],"vin":[{"txid":"0x539084697cc220916cb5b16d2805945ec9f267aa004b6688fbf15e116c846aff","vout":0}],"vout":[{"address":"AXpNr3SDfLXbPHNdqxYeHK5cYpKMHZxMZ9","asset":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","n":0,"value":"10000"},{"address":"AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y","asset":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","n":1,"value":"99990000"}],"scripts":[{"invocation":"40a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c6","verification":"21031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac"}],"txid":"0xf999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a","size":283,"sys_fee":"0","net_fee":"0","blockhash":"0x6088bf9d3b55c67184f60b00d2e380228f713b4028b24c1719796dcd2006e417","confirmations":2902,"blocktime":1533756500},"id":1}`,
	},

	// Good case, valid transaction (param[1]="dads" -> verbose = 1. Following the C# any string different from "0", "false" is interpreted as verbose = 1)
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", "dads"] }`,
		method:         "getrawtransaction_3",
		expectedResult: `{"jsonrpc":"2.0","result":{"type":"ContractTransaction","version":0,"attributes":[{"data":"23ba2703c53263e8d6e522dc32203339dcd8eee9","usage":"Script"}],"vin":[{"txid":"0x539084697cc220916cb5b16d2805945ec9f267aa004b6688fbf15e116c846aff","vout":0}],"vout":[{"address":"AXpNr3SDfLXbPHNdqxYeHK5cYpKMHZxMZ9","asset":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","n":0,"value":"10000"},{"address":"AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y","asset":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","n":1,"value":"99990000"}],"scripts":[{"invocation":"40a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c6","verification":"21031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac"}],"txid":"0xf999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a","size":283,"sys_fee":"0","net_fee":"0","blockhash":"0x6088bf9d3b55c67184f60b00d2e380228f713b4028b24c1719796dcd2006e417","confirmations":2902,"blocktime":1533756500},"id":1}`,
	},

	// Bad case, invalid transaction
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["45a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", 1] }`,
		method:         "getrawtransaction_4",
		expectedResult: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"},"id":1}`,
	},

	// Good case, valid transaction
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a"] }`,
		method:         "getrawtransaction_5",
		expectedResult: `{"jsonrpc":"2.0","result":"8000012023ba2703c53263e8d6e522dc32203339dcd8eee901ff6a846c115ef1fb88664b00aa67f2c95e9405286db1b56c9120c27c698490530000029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50010a5d4e8000000affb37f5fdb9c6fec48d9f0eee85af82950f9b4a9b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500f01b9b0986230023ba2703c53263e8d6e522dc32203339dcd8eee9014140a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c62321031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac","id":1}`,
	},

	// Good case, valid transaction (param[1]= 0 -> verbose = 0)
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", 0] }`,
		method:         "getrawtransaction_6",
		expectedResult: `{"jsonrpc":"2.0","result":"8000012023ba2703c53263e8d6e522dc32203339dcd8eee901ff6a846c115ef1fb88664b00aa67f2c95e9405286db1b56c9120c27c698490530000029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50010a5d4e8000000affb37f5fdb9c6fec48d9f0eee85af82950f9b4a9b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500f01b9b0986230023ba2703c53263e8d6e522dc32203339dcd8eee9014140a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c62321031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac","id":1}`,
	},

	// Good case, valid transaction (param[1]="false" -> verbose = 0)
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", "false"] }`,
		method:         "getrawtransaction_6_a",
		expectedResult: `{"jsonrpc":"2.0","result":"8000012023ba2703c53263e8d6e522dc32203339dcd8eee901ff6a846c115ef1fb88664b00aa67f2c95e9405286db1b56c9120c27c698490530000029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50010a5d4e8000000affb37f5fdb9c6fec48d9f0eee85af82950f9b4a9b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500f01b9b0986230023ba2703c53263e8d6e522dc32203339dcd8eee9014140a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c62321031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac","id":1}`,
	},

	// Good case, valid transaction (param[1]=false -> verbose = 0)
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", false] }`,
		method:         "getrawtransaction_6_b",
		expectedResult: `{"jsonrpc":"2.0","result":"8000012023ba2703c53263e8d6e522dc32203339dcd8eee901ff6a846c115ef1fb88664b00aa67f2c95e9405286db1b56c9120c27c698490530000029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50010a5d4e8000000affb37f5fdb9c6fec48d9f0eee85af82950f9b4a9b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500f01b9b0986230023ba2703c53263e8d6e522dc32203339dcd8eee9014140a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c62321031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac","id":1}`,
	},

	// Good case, valid transaction (param[1]="0" -> verbose = 0)
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": ["f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a", "0"] }`,
		method:         "getrawtransaction_6_c",
		expectedResult: `{"jsonrpc":"2.0","result":"8000012023ba2703c53263e8d6e522dc32203339dcd8eee901ff6a846c115ef1fb88664b00aa67f2c95e9405286db1b56c9120c27c698490530000029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc50010a5d4e8000000affb37f5fdb9c6fec48d9f0eee85af82950f9b4a9b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500f01b9b0986230023ba2703c53263e8d6e522dc32203339dcd8eee9014140a88bd1fcfba334b06da0ce1a679f80711895dade50352074e79e438e142dc95528d04a00c579398cb96c7301428669a09286ae790459e05e907c61ab8a1191c62321031a6c6fbbdf02ca351745fa86b9ba5a9452d785ac4f7fc2b7548ca2a46c4fcf4aac","id":1}`,
	},

	// Bad case, param at index 0 not a string
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getrawtransaction", "params": [123, 0] }`,
		method:         "getrawtransaction_7",
		expectedResult: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"},"id":1}`,
	},

	// Good case, valid address
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getaccountstate", "params": ["AK2nJJpJr6o664CWJKi1QRXjqeic2zRp8y"] }`,
		method:         "getaccountstate_1",
		expectedResult: `{"jsonrpc":"2.0","result":{"version":0,"script_hash":"0xe9eed8dc39332032dc22e5d6e86332c50327ba23","frozen":false,"votes":[],"balances":[{"asset":"0x602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7","value":"72099.99960000"},{"asset":"0xc56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b","value":"99989900"}]},"id":1}`,
	},

	// Bad case, invalid address
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getaccountstate", "params": ["AK2nJJpJr6o664CWJKi1QRXjqeic2zR"] }`,
		method:         "getaccountstate_2",
		expectedResult: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"},"id":1}`,
	},

	// Bad case, not string
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getaccountstate", "params": [123] }`,
		method:         "getaccountstate_3",
		expectedResult: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"},"id":1}`,
	},

	// Bad case, empty params
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "getaccountstate", "params": [] }`,
		method:         "getaccountstate_4",
		expectedResult: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"},"id":1}`,
	},

	// Good case, valid address
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "validateaddress", "params": ["AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i"] }`,
		method:         "validateaddress_1",
		expectedResult: `{"jsonrpc":"2.0","result":{"address":"AQVh2pG732YvtNaxEGkQUei3YA4cvo7d2i","isvalid":true},"id":1}`,
	},

	// Bad case, invalid address
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "validateaddress", "params": ["152f1muMCNa7goXYhYAQC61hxEgGacmncB"] }`,
		method:         "validateaddress_2",
		expectedResult: `{"jsonrpc":"2.0","result":{"address":"152f1muMCNa7goXYhYAQC61hxEgGacmncB","isvalid":false},"id":1}`,
	},

	// Bad case, not string
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "validateaddress", "params": [1] }`,
		method:         "validateaddress_3",
		expectedResult: `{"jsonrpc":"2.0","result":{"address":1,"isvalid":false},"id":1}`,
	},

	// Bad case, empty params
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "validateaddress", "params": [] }`,
		method:         "validateaddress_4",
		expectedResult: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"},"id":1}`,
	},

	// Good case
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "sendrawtransaction", "params": ["80000190274d792072617720636f6e7472616374207472616e73616374696f6e206465736372697074696f6e01949354ea0a8b57dfee1e257a1aedd1e0eea2e5837de145e8da9c0f101bfccc8e0100029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500a3e11100000000ea610aa6db39bd8c8556c9569d94b5e5a5d0ad199b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc5004f2418010000001cc9c05cefffe6cdd7b182816a9152ec218d2ec0014140dbd3cddac5cb2bd9bf6d93701f1a6f1c9dbe2d1b480c54628bbb2a4d536158c747a6af82698edf9f8af1cac3850bcb772bd9c8e4ac38f80704751cc4e0bd0e67232103cbb45da6072c14761c9da545749d9cfd863f860c351066d16df480602a2024c6ac"] }`,
		method:         "sendrawtransaction_1",
		expectedResult: `{"jsonrpc":"2.0","result":true,"id":1}`,
	},

	/* Good case: TODO: uncomment this test case once https://github.com/CityOfZion/neo-go/issues/173 is fixed!
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "sendrawtransaction", "params": ["d1001b00046e616d6567d3d8602814a429a91afdbaa3914884a1c90c733101201cc9c05cefffe6cdd7b182816a9152ec218d2ec000000141403387ef7940a5764259621e655b3c621a6aafd869a611ad64adcc364d8dd1edf84e00a7f8b11b630a377eaef02791d1c289d711c08b7ad04ff0d6c9caca22cfe6232103cbb45da6072c14761c9da545749d9cfd863f860c351066d16df480602a2024c6ac"] }`,
		method:         "sendrawtransaction_2",
		expectedResult: `{"jsonrpc":"2.0","result":true,"id":1}`,
	},*/

	// Bad case, incorrect raw transaction
	{
		rpcCall:        `{ "jsonrpc": "2.0", "id": 1, "method": "sendrawtransaction", "params": ["0274d792072617720636f6e7472616374207472616e73616374696f6e206465736372697074696f6e01949354ea0a8b57dfee1e257a1aedd1e0eea2e5837de145e8da9c0f101bfccc8e0100029b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc500a3e11100000000ea610aa6db39bd8c8556c9569d94b5e5a5d0ad199b7cffdaa674beae0f930ebe6085af9093e5fe56b34a5c220ccdcf6efc336fc5004f2418010000001cc9c05cefffe6cdd7b182816a9152ec218d2ec0014140dbd3cddac5cb2bd9bf6d93701f1a6f1c9dbe2d1b480c54628bbb2a4d536158c747a6af82698edf9f8af1cac3850bcb772bd9c8e4ac38f80704751cc4e0bd0e67232103cbb45da6072c14761c9da545749d9cfd863f860c351066d16df480602a2024c6ac"] }`,
		method:         "sendrawtransaction_1",
		expectedResult: `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid Params"},"id":1}`,
	},
}

func TestHandler(t *testing.T) {
	// setup rpcServer server
	net := config.ModeUnitTestNet
	configPath := "../../config"
	cfg, err := config.Load(configPath, net)
	require.NoError(t, err, "could not load config")

	chain, err := core.NewBlockchainLevelDB(context.Background(), cfg)
	require.NoError(t, err, "could not create levelDB chain")

	serverConfig := network.NewServerConfig(cfg)
	server := network.NewServer(serverConfig, chain)
	rpcServer := NewServer(chain, cfg.ApplicationConfiguration.RPCPort, server)

	// setup handler
	handler := http.HandlerFunc(rpcServer.requestHandler)

	testRpcCases = append(testRpcCases, tc{
		rpcCall:        `{"jsonrpc": "2.0", "id": 1, "method": "getversion", "params": [] }`,
		method:         "getversion",
		expectedResult: fmt.Sprintf(`{"jsonrpc":"2.0","result":{"port":20333,"nonce":%s,"useragent":"/NEO-GO:/"},"id":1}`, strconv.FormatUint(uint64(server.ID()), 10)),
	})

	for _, tc := range testRpcCases {
		t.Run(fmt.Sprintf("method: %s, rpc call: %s", tc.method, tc.rpcCall), func(t *testing.T) {

			req := httptest.NewRequest("POST", "http://0.0.0.0:20333/", strings.NewReader(tc.rpcCall))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler(w, req)
			resp := w.Result()
			body, err := ioutil.ReadAll(resp.Body)
			assert.NoErrorf(t, err, "could not read response from the request: %s", tc.rpcCall)
			assert.JSONEq(t, tc.expectedResult, string(bytes.TrimSpace(body)))
		})

	}
}
