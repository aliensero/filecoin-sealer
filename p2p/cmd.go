package p2p

import (
	"github.com/filecoin-project/lotus/node"
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
		&cli.StringFlag{
			Name:  "listen",
			Usage: "RPC server listen",
			Value: "127.0.0.1:4321",
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
			up2p.NsOverride(new(up2p.Faddr), up2p.Faddr(cctx.String("listen"))),
		)
		if err != nil {
			return err
		}
		defer stop(ctx)
		return nil
	},
}

var TranCmd = &cli.Command{
	Name:  "coltrn",
	Usage: "collect transaction",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "repo",
			Usage: "chain bitswap repo",
			Value: "chain-repo",
		},
		&cli.StringFlag{
			Name:  "user",
			Usage: "database user",
			Value: "root",
		},
		&cli.StringFlag{
			Name:  "password",
			Usage: "database password",
		},
		&cli.StringFlag{
			Name:  "ip",
			Usage: "database ip",
			Value: "127.0.0.1",
		},
		&cli.StringFlag{
			Name:  "port",
			Usage: "database port",
		},
		&cli.StringFlag{
			Name:  "database",
			Usage: "database name",
			Value: "db_worker",
		},
		&cli.StringFlag{
			Name:  "listen",
			Usage: "RPC server listen",
			Value: "127.0.0.1:4321",
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
			up2p.CollectTtranOpt,
			node.Override(new(up2p.DbParams), func() up2p.DbParams {
				return up2p.DbParams{
					User:     cctx.String("user"),
					Password: cctx.String("password"),
					Ip:       cctx.String("ip"),
					Port:     cctx.String("port"),
					Database: cctx.String("database"),
				}
			}),
			up2p.NsOverride(new(up2p.Faddr), up2p.Faddr(cctx.String("listen"))),
		)
		if err != nil {
			return err
		}
		defer stop(ctx)
		return nil
	},
}
