package p2p

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
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
			Usage: "Networker name",
			Value: "testnetnet",
			//Value: "calibrationnet",
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := cctx.Context
		opts := []fx.Option{}
		opts = append(opts, LIBP2PONLY...)
		opts = append(opts, Override(new(NsNetWorkerName), NsNetWorkerName(cctx.String("netname"))))
		stopFunc, err := NewNoDefault(ctx, opts...)
		if err != nil {
			return err
		}
		defer stopFunc(ctx)
		return nil
	},
}
