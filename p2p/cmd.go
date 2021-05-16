package p2p

import (
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
	"gitlab.ns/lotus-worker/util"
	up2p "gitlab.ns/lotus-worker/util/p2p"
)

var log = logging.Logger("p2p")

func init() {
	logging.SetLogLevel("p2p", "DEBUG")
}

var SubCmd = &cli.Command{
	Name: "sub",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "repo",
			Usage: "chain bitswap repo",
			Value: "chain-swap-repo",
		},
		&cli.StringFlag{
			Name:  "privatekey",
			Usage: "private key",
		},
		&cli.Uint64Flag{
			Name:  "actorid",
			Usage: "actorID",
		},
	},
	Action: func(cctx *cli.Context) error {
		ctx := cctx.Context
		r, err := up2p.CreateRepo(cctx.String("repo"))
		if err != nil {
			return err
		}
		err = r.Init(up2p.NsFullNode)
		if err != nil {
			return err
		}
		stop, err := up2p.NsNodeNew(ctx,
			up2p.Repo(r),
			up2p.ChainSwapOpt,
			up2p.NsOverride(new(up2p.PrivateKey), up2p.PrivateKey(cctx.String("privatekey"))),
			up2p.NsOverride(new(util.NsAddress), func() (util.NsAddress, error) {
				return util.NsNewIDAddress(cctx.Uint64("actorid"))
			}),
		)
		if err != nil {
			return err
		}
		defer stop(ctx)
		return nil
	},
}
