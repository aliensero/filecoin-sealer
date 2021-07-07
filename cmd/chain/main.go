package main

import (
	"os"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
	"gitlab.ns/lotus-worker/chain"
	"golang.org/x/xerrors"
)

var log = logging.Logger("chain")

func init() {
	if level, ok := os.LookupEnv("LEVEL_LOG"); !ok {
		logging.SetLogLevel("chain", "INFO")
	} else {
		logging.SetLogLevel("chain", level)
	}
}

func main() {

	localCmds := []*cli.Command{
		daemonCmd,
	}

	app := cli.App{
		Name:     "chain",
		Usage:    "chain daemon",
		Commands: localCmds,
	}

	if err := app.Run(os.Args); err != nil {
		log.Warnf("ns-miner daemon setup error %v", err)
	}

}

var daemonCmd = &cli.Command{
	Name:  "daemon",
	Usage: "[lotus api] [listen]",
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 2 {
			return xerrors.Errorf("[lotus api] [listen]")
		}

		return chain.Daemon(cctx.Args().Get(0), cctx.Args().Get(1))
	},
}
