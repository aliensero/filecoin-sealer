package mining

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/term"
	"golang.org/x/xerrors"
)

var MiningCmd = &cli.Command{
	Name:  "mining",
	Usage: "[storage paths]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "privatekey",
			Usage: "private key",
		},
		&cli.Uint64Flag{
			Name:  "actorid",
			Usage: "actorID",
		},
		&cli.StringFlag{
			Name:    "lotusapi",
			Usage:   "lotus api [https://calibration.node.glif.io] [https://api.node.glif.io]",
			EnvVars: []string{"LOTUS_API"},
			Value:   "https://calibration.node.glif.io",
		},
		&cli.StringFlag{
			Name:  "netname",
			Usage: "networker name [calibrationnet] [testnetnet]",
			Value: "calibrationnet",
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := cctx.Context
		ps := cctx.Args().Slice()
		if len(ps) < 1 {
			return xerrors.Errorf("mining [storage paths]")
		}

		pk := cctx.String("privatekey")
		if pk == "" {
			fmt.Print("enter private key:")
			buf, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				return err
			}
			fmt.Println()
			pk = string(buf)
		}

		fa, close, err := util.NewPubLotusApi(cctx.String("lotusapi"))
		if err != nil {
			return err
		}
		defer close()
		nn := cctx.String("netname")
		log.Infof("p2p networker name %s", nn)
		ffa, err := util.NewP2pLotusAPI(fa, util.NsNetworkName(nn))
		if err != nil {
			return err
		}

		mr, err := util.NsNewIDAddress(cctx.Uint64("actorid"))
		if err != nil {
			return err
		}

		ki, err := util.GenerateKeyByHexString(pk)
		if err != nil {
			return err
		}

		MinerServerNotify(ctx, ffa, mr, ki, ps)

		return nil
	},
}
