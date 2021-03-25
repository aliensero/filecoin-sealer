package main

import (
	"net"
	"net/http"
	"os"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/gorilla/mux"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
	"gitlab.ns/lotus-worker/miner"
	"gitlab.ns/lotus-worker/p2p"
	"gitlab.ns/lotus-worker/util"
)

func init() {
	_ = logging.SetLogLevel("miner", "DEBUG")
}

var log = logging.Logger("miner")

func main() {

	localCmds := []*cli.Command{
		daemonCmd,
		p2p.SubCmd,
	}

	app := cli.App{
		Name:     "ns-miner",
		Usage:    "ns-miner daemon",
		Commands: localCmds,
	}

	if err := app.Run(os.Args); err != nil {
		log.Warnf("ns-miner daemon setup error %v", err)
	}

}

var daemonCmd = &cli.Command{
	Name: "daemon",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "listen",
			Value: "127.0.0.1:4321",
		},
		&cli.StringFlag{
			Name:  "user",
			Usage: "database user",
			Value: "dockeruser",
		},
		&cli.StringFlag{
			Name:  "passwd",
			Usage: "database password",
			Value: "123456",
		},
		&cli.StringFlag{
			Name:  "ip",
			Usage: "database ip",
			Value: "127.0.0.1",
		},
		&cli.StringFlag{
			Name:  "port",
			Usage: "database port",
			Value: "3306",
		},
		&cli.StringFlag{
			Name:  "name",
			Usage: "database name",
			Value: "db_worker",
		},
		&cli.StringFlag{
			Name:  "lotus-addr",
			Usage: "lotus address",
			Value: "http://127.0.0.1:1234/rpc/v0",
		},
		&cli.StringFlag{
			Name:  "lotus-token",
			Usage: "lotus token",
			Value: "token",
		},
		&cli.Int64Flag{
			Name:  "waitseedepoch",
			Usage: "wait seed epoch",
			Value: 3,
		},
		&cli.Int64Flag{
			Name:  "expiration",
			Usage: "Sector Expiration",
			Value: 581402,
		},
	},
	Action: func(cctx *cli.Context) error {
		db, err := util.InitMysql(cctx.String("user"), cctx.String("passwd"), cctx.String("ip"), cctx.String("port"), cctx.String("name"))
		if err != nil {
			return err
		}
		err = miner.InitDbTaskFailed(db)
		if err != nil {
			return err
		}
		var close jsonrpc.ClientCloser
		var lotusApi util.LotusAPI
		if cctx.String("lotus-token") != "" {
			lotusApi1, close1, err := util.NewLotusApi(cctx.String("lotus-addr"), cctx.String("lotus-token"))
			if err != nil {
				return err
			}
			close = close1
			lotusApi = lotusApi1
		} else {
			lotusApi1, close1, err := util.NewPubLotusApi(cctx.String("lotus-addr"))
			if err != nil {
				return err
			}
			close = close1
			lotusApi = lotusApi1
		}
		defer close()
		miner := miner.Miner{
			Db:               db,
			LotusApi:         lotusApi,
			WaitSeedEpoch:    cctx.Int64("waitseedepoch"),
			SectorExpiration: cctx.Int64("expiration"),
			LotusUrl:         cctx.String("lotus-addr"),
			LotusToken:       cctx.String("lotus-token"),
			SyncPoStMap:      miner.NewSyncPostMap(),
		}
		rpcServer := jsonrpc.NewServer()
		mux := mux.NewRouter()
		rpcServer.Register("NSMINER", &miner)
		mux.Handle("/rpc/v0", rpcServer)
		mux.HandleFunc("/session", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("session"))
		})
		mux.PathPrefix("/").Handler(http.DefaultServeMux)

		srv := &http.Server{
			Handler: mux,
		}
		log.Info("Setting up control endpoint at " + cctx.String("listen"))
		nl, err := net.Listen("tcp", cctx.String("listen"))
		if err != nil {
			return err
		}
		return srv.Serve(nl)
	},
}
