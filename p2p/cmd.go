package p2p

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
	"gitlab.ns/lotus-worker/util"
	"go.uber.org/fx"
)

var log = logging.Logger("p2p")

func init() {
	logging.SetLogLevel("p2p", "DEBUG")
}

var SubCmd = &cli.Command{
	Name: "sub",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "netname",
			Usage: "Networker name [testnetnet|calibrationnet]",
			Value: "testnetnet",
			//Value: "calibrationnet",
		},
		&cli.StringFlag{
			Name:  "lotusapi",
			Usage: "lotus api string [https://api.node.glif.io|https://calibration.node.glif.io]",
			Value: "https://api.node.glif.io",
		},
		&cli.StringFlag{
			Name:  "actor",
			Usage: "actor id",
			Value: "t01000",
		},
		&cli.StringFlag{
			Name:  "prihex",
			Usage: "private key hex",
		},
		&cli.Uint64Flag{
			Name:  "testpower",
			Usage: "test miner power",
			Value: 10995116277760, // 10TiB
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := cctx.Context
		opts := []fx.Option{}
		opts = append(opts, LIBP2PONLY...)
		opts = append(opts, Override(new(util.NsNetworkName), util.NsNetworkName(cctx.String("netname"))))
		opts = append(opts, Override(new(util.LotusApiStr), util.LotusApiStr(cctx.String("lotusapi"))))
		opts = append(opts, Override(new(AddrStr), AddrStr(cctx.String("actor"))))
		opts = append(opts, Override(new(PriHex), PriHex(cctx.String("prihex"))))
		opts = append(opts, Override(new(Power), Power(cctx.Uint64("testpower"))))
		stopFunc, err := NewNoDefault(ctx, opts...)
		if err != nil {
			return err
		}
		defer stopFunc(ctx)
		return nil
	},
}
