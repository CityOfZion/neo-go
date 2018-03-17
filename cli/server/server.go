package server

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/network"
	"github.com/CityOfZion/neo-go/pkg/util"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// NewCommand creates a new Node command.
func NewCommand() cli.Command {
	return cli.Command{
		Name:   "node",
		Usage:  "start a NEO node",
		Action: startServer,
		Flags: []cli.Flag{
			cli.StringFlag{Name: "config-path"},
			cli.BoolFlag{Name: "privnet, p"},
			cli.BoolFlag{Name: "mainnet, m"},
			cli.BoolFlag{Name: "testnet, t"},
			cli.BoolFlag{Name: "debug, d"},
		},
	}
}

func startServer(ctx *cli.Context) error {
	net := network.ModePrivNet
	if ctx.Bool("testnet") {
		net = network.ModeTestNet
	}
	if ctx.Bool("mainnet") {
		net = network.ModeMainNet
	}

	configPath := "./config"
	configPath = ctx.String("config-path")
	config, err := network.LoadConfig(configPath, net)
	if err != nil {
		return cli.NewExitError(err, 1)
	}

	serverConfig := network.NewServerConfig(config)
	chain, err := newBlockchain(net, config.ApplicationConfiguration.DataDirectoryPath)
	if err != nil {
		err = fmt.Errorf("could not initialize blockhain: %s", err)
		return cli.NewExitError(err, 1)
	}

	if ctx.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	fmt.Println(logo())
	network.NewServer(serverConfig, chain).Start()
	return nil
}

func newBlockchain(net network.NetMode, path string) (*core.Blockchain, error) {
	var startHash util.Uint256
	if net == network.ModePrivNet {
		startHash = core.GenesisHashPrivNet()
	}
	if net == network.ModeTestNet {
		startHash = core.GenesisHashTestNet()
	}
	if net == network.ModeMainNet {
		startHash = core.GenesisHashMainNet()
	}

	// Hardcoded for now.
	store, err := storage.NewLevelDBStore(path, nil)
	if err != nil {
		return nil, err
	}

	return core.NewBlockchain(store, startHash)
}

func logo() string {
	return `
    _   ____________        __________
   / | / / ____/ __ \      / ____/ __ \
  /  |/ / __/ / / / /_____/ / __/ / / /
 / /|  / /___/ /_/ /_____/ /_/ / /_/ /
/_/ |_/_____/\____/      \____/\____/
`
}
