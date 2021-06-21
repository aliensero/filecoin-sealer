package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/gorilla/mux"
	"golang.org/x/term"
	"golang.org/x/xerrors"

	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
	"gitlab.ns/lotus-worker/miner"
	"gitlab.ns/lotus-worker/mining"
	"gitlab.ns/lotus-worker/p2p"
	"gitlab.ns/lotus-worker/util"
	up2p "gitlab.ns/lotus-worker/util/p2p"
	"gitlab.ns/lotus-worker/worker"
)

var log = logging.Logger("miner")

func main() {

	localCmds := []*cli.Command{
		daemonCmd,
		p2p.SubCmd,
		p2p.TranCmd,
		p2p.ComputeReceis,
		taskTriggerCmd,
		addTaskCmd,
		createminerCmd,
		sendCmd,
		newAddrCmd,
		replaceMsgCmd,
		recoverPoStCmd,
		resetPoStCmd,
		refindPoStCmd,
		addPostCmd,
		retryPostCmd,
		manualPostCmd,
		mining.MiningCmd,
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

		if v, ok := os.LookupEnv("LOG_LEVEL"); ok {
			logging.SetLogLevel("miner-struct", v)
		}

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
		p2papi, err := util.NewP2pLotusAPI(lotusApi, up2p.NATNAME)
		if err != nil {
			return err
		}
		miner := miner.Miner{
			Db:               db,
			LotusApi:         p2papi,
			WaitSeedEpoch:    cctx.Int64("waitseedepoch"),
			SectorExpiration: cctx.Int64("expiration"),
			LotusUrl:         cctx.String("lotus-addr"),
			LotusToken:       cctx.String("lotus-token"),
			SyncPoStMap:      miner.NewSyncPostMap(),
			PoStMiner:        make(map[int64]util.ActorPoStInfo),
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

		go miner.WinPoStProofsServer()

		return srv.Serve(nl)
	},
}

const (
	SEED      = "SEED"
	PRECOMMIT = "PRECOMMIT"
	COMMIT    = "COMMIT"
)

var taskTriggerCmd = &cli.Command{
	Name: "tasktrig",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "prikey",
			Usage: "private key",
		},
		&cli.StringFlag{
			Name:  "deposit",
			Usage: "take FIL",
			Value: "0",
		},
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			EnvVars: []string{"MINER_API"},
			Value:   "ws://127.0.0.1:1234/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 3 {
			return fmt.Errorf("[actorid] [sectornum] [tasktype]")
		}

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()

		ai, err := strconv.ParseInt(cctx.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}
		var actorID int64 = ai
		si, err := strconv.ParseInt(cctx.Args().Get(1), 10, 64)
		if err != nil {
			return err
		}
		var sectorNum int64 = si

		taskType := cctx.Args().Get(2)

		pk := cctx.String("prikey")
		var readPrivateKey = func() error {
			if pk == "" {
				fmt.Print("enter private key:")
				buf, err := term.ReadPassword(int(os.Stdin.Fd()))
				if err != nil {
					return err
				}
				pk = string(buf)
			}
			return nil
		}

		var result interface{}
		switch taskType {
		case SEED:
			result, err = minerApi.GetSeedRand(actorID, sectorNum)
		case PRECOMMIT:
			err := readPrivateKey()
			if err != nil {
				return err
			}
			result, err = minerApi.SendPreCommitByPrivatKey(pk, actorID, sectorNum, cctx.String("deposit"))
		case COMMIT:
			err := readPrivateKey()
			if err != nil {
				return err
			}
			result, err = minerApi.SendCommitByPrivatKey(pk, actorID, sectorNum, cctx.String("deposit"))
		}
		if err == nil {
			log.Info(result)
		}
		return err
	},
}

var addTaskCmd = &cli.Command{
	Name: "addtask",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "storagePath",
			Usage:   "storage path",
			Value:   "",
			EnvVars: []string{"STORAGE_PATH"},
		},
		&cli.StringFlag{
			Name:    "unsealPath",
			Usage:   "unseal path",
			EnvVars: []string{"UNSEAL_PATH"},
		},
		&cli.StringFlag{
			Name:    "sealerProof",
			Usage:   "sealer path",
			Value:   "32GiB",
			EnvVars: []string{"SEALER_PROOF"},
		},
		&cli.IntFlag{
			Name:    "proofType",
			Usage:   "proof type",
			Value:   3,
			EnvVars: []string{"PROOF_TYPE"},
		},
		&cli.StringFlag{
			Name:    "pieceCID",
			Usage:   "piece cid",
			Value:   "baga6ea4seaqao7s73y24kcutaosvacpdjgfe5pw76ooefnyqw4ynr3d2y6x2mpq",
			EnvVars: []string{"PIECE_CID"},
		},
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			Value:   "ws://127.0.0.1:1234/rpc/v0",
			EnvVars: []string{"MINER_API"},
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 2 {
			return fmt.Errorf("[actorid] [sectornum]")
		}

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()

		ai, err := strconv.ParseInt(cctx.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}
		var actorID int64 = ai
		si, err := strconv.ParseInt(cctx.Args().Get(1), 10, 64)
		if err != nil {
			return err
		}
		var sectorNum int64 = si

		var result interface{}
		result, err = minerApi.AddTask(actorID, sectorNum, util.PC1, cctx.String("storagePath"), cctx.String("unsealPath"), cctx.String("sealerProof"), cctx.Int("proofType"), cctx.String("pieceCID"))
		if err != nil {
			log.Info(result)
		}
		return err
	},
}

var newAddrCmd = &cli.Command{
	Name: "newaddr",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "type",
			Usage: "address type",
			Value: "bls",
		},
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			EnvVars: []string{"MINER_API"},
			Value:   "ws://127.0.0.1:1234/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()

		var result interface{}
		result, err = minerApi.GenerateAddress(util.NsKeyType(cctx.String("type")))
		if err != nil {
			log.Info(result)
		}
		return err
	},
}

var sendCmd = &cli.Command{
	Name:  "send",
	Usage: "[to] [FIL]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "prikey",
			Usage: "private key",
		},
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			EnvVars: []string{"MINER_API"},
			Value:   "ws://127.0.0.1:1234/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 2 {
			return fmt.Errorf("[to] [FIL]")
		}

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()

		pk := cctx.String("prikey")
		var readPrivateKey = func() error {
			if pk == "" {
				fmt.Print("enter private key:")
				buf, err := term.ReadPassword(int(os.Stdin.Fd()))
				if err != nil {
					return err
				}
				pk = string(buf)
			}
			return nil
		}

		if pk == "" {
			err := readPrivateKey()
			if err != nil {
				return err
			}
		}

		var result interface{}

		result, err = minerApi.TxByPrivatKey(pk, cctx.Args().Get(0), cctx.Args().Get(1))
		if err == nil {
			log.Info(result)
		}
		return err
	},
}

var createminerCmd = &cli.Command{
	Name:  "createminer",
	Usage: "[owner] [worker]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "prikey",
			Usage: "private key",
		},
		&cli.Int64Flag{
			Name:  "type",
			Usage: "seal proof type defaul 3 (32GiB)",
			Value: 3,
		},
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			EnvVars: []string{"MINER_API"},
			Value:   "ws://127.0.0.1:1234/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 2 {
			return fmt.Errorf("[owner] [worker]")
		}

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()

		pk := cctx.String("prikey")
		var readPrivateKey = func() error {
			if pk == "" {
				fmt.Print("enter private key:")
				buf, err := term.ReadPassword(int(os.Stdin.Fd()))
				if err != nil {
					return err
				}
				pk = string(buf)
			}
			return nil
		}

		if pk == "" {
			err := readPrivateKey()
			if err != nil {
				return err
			}
		}

		var result interface{}

		result, err = minerApi.CreateMiner(pk, cctx.Args().Get(0), cctx.Args().Get(1), cctx.Int64("type"))
		if err == nil {
			log.Info(result)
		}
		return err
	},
}

var replaceMsgCmd = &cli.Command{
	Name:  "replace",
	Usage: "replace message [cid] [cap] [premium] [limit]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "prikey",
			Usage: "private key",
		},
		&cli.Int64Flag{
			Name:  "type",
			Usage: "seal proof type defaul 3 (32GiB)",
			Value: 3,
		},
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			EnvVars: []string{"MINER_API"},
			Value:   "ws://127.0.0.1:1234/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 4 {
			return fmt.Errorf("[message cid] [cap] [premium] [limit]")
		}

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()

		var result []*util.NsSignedMessage
		var match *util.NsSignedMessage
		result, err = minerApi.MpoolPending()
		cid := cctx.Args().First()
		if err == nil {
			for _, sm := range result {
				if cid == sm.Cid().String() {
					match = sm
				}
			}
		}
		if match == nil {
			return xerrors.Errorf("mpool pending no found message cid %s", cid)
		}
		m := match.Message
		m.GasFeeCap, err = util.NsBigFromString(cctx.Args().Get(1))
		if err != nil {
			return err
		}
		m.GasPremium, err = util.NsBigFromString(cctx.Args().Get(2))
		if err != nil {
			return err
		}
		limit, err := strconv.ParseInt(cctx.Args().Get(3), 10, 64)
		if err != nil {
			return err
		}
		m.GasLimit = limit

		pk := cctx.String("prikey")
		var readPrivateKey = func() error {
			if pk == "" {
				fmt.Print("enter private key:")
				buf, err := term.ReadPassword(int(os.Stdin.Fd()))
				if err != nil {
					return err
				}
				pk = string(buf)
			}
			return nil
		}

		if pk == "" {
			err := readPrivateKey()
			if err != nil {
				return err
			}
		}

		toSing, err := minerApi.SignedMessage(pk, m)
		if err != nil {
			return nil
		}

		retcid, err := minerApi.MpoolPush(toSing)
		if err != nil {
			return err
		}
		log.Infof("push message cid %v", retcid)
		return nil
	},
}

var recoverPoStCmd = &cli.Command{
	Name:  "recover",
	Usage: "reover post [actorid] [addr or private key] [deadline]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "type",
			Usage: "address or private key",
			Value: "pri",
		},
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			EnvVars: []string{"MINER_API"},
			Value:   "ws://127.0.0.1:1234/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 3 {
			return fmt.Errorf("[actorid] [addr or private key] [deadline]")
		}

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()

		t := cctx.String("type")
		pk := cctx.Args().Get(1)
		if t == worker.PRI && pk == "" {
			fmt.Print("enter private key:")
			buf, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				return err
			}
			fmt.Println()
			pk = string(buf)
		}

		ai, err := strconv.ParseInt(cctx.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}

		di, err := strconv.ParseUint(cctx.Args().Get(2), 10, 64)
		if err != nil {
			return err
		}

		result, err := minerApi.CheckRecoveries(ai, pk, t, di)
		if err == nil {
			log.Info(result)
		}
		return err
	},
}

var resetPoStCmd = &cli.Command{
	Name:  "reset-post",
	Usage: "reset post",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			EnvVars: []string{"MINER_API"},
			Value:   "ws://127.0.0.1:1234/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 3 {
			return fmt.Errorf("[actorid] [addr or private key] [deadline]")
		}
		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()
		result := minerApi.ResetSyncPoStMap()
		log.Info(result)
		return nil
	},
}

var refindPoStCmd = &cli.Command{
	Name:  "refind-post",
	Usage: "refind-post [actorid]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			EnvVars: []string{"MINER_API"},
			Value:   "ws://127.0.0.1:1234/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 1 {
			return fmt.Errorf("[actorid]")
		}

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()

		ai, err := strconv.ParseInt(cctx.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}

		result, err := minerApi.ReFindPoStTable(ai)
		if err == nil {
			log.Info(result)
		}
		return err
	},
}

var addPostCmd = &cli.Command{
	Name:  "addPost",
	Usage: "[actorid] [address or private key]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "type",
			Usage: "private type",
			Value: "pri",
		},
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			EnvVars: []string{"MINER_API"},
			Value:   "ws://127.0.0.1:4321/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 2 {
			return fmt.Errorf("[actorid] [address or private key]")
		}

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()

		ai, err := strconv.ParseInt(cctx.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}
		var actorID int64 = ai

		t := cctx.String("type")
		pk := cctx.Args().Get(1)
		if t == worker.PRI && pk == "" {
			fmt.Print("enter private key:")
			buf, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				return err
			}
			fmt.Println()
			pk = string(buf)
		}

		result, err := minerApi.AddPoStActor(actorID, pk, cctx.String("type"))

		if err == nil {
			log.Info(result)
		}
		return err
	},
}

var retryPostCmd = &cli.Command{
	Name:  "retryPost",
	Usage: "[actorid] [address or private key]",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "type",
			Usage: "private type",
			Value: "pri",
		},
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			EnvVars: []string{"MINER_API"},
			Value:   "ws://127.0.0.1:4321/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 2 {
			return fmt.Errorf("[actorid] [address or private key]")
		}

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()

		ai, err := strconv.ParseInt(cctx.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}
		var actorID int64 = ai

		t := cctx.String("type")
		pk := cctx.Args().Get(1)
		if t == worker.PRI && pk == "" {
			fmt.Print("enter private key:")
			buf, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				return err
			}
			fmt.Println()
			pk = string(buf)
		}

		result, err := minerApi.WinPoStProofs(actorID, pk)

		if err == nil {
			log.Info(result)
		}
		return err
	},
}

var manualPostCmd = &cli.Command{
	Name:  "manualPost",
	Usage: "[actorid] [address or private key]",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "submit",
			Usage: "submit message",
			Value: false,
		},
		&cli.Int64Flag{
			Name:  "ptype",
			Usage: "proof type",
			Value: 3,
		},
		&cli.Int64Flag{
			Name:  "cepoch",
			Usage: "chanllage epoch",
		},
		&cli.StringFlag{
			Name:  "sipath",
			Usage: "sectors info path",
			Value: "sectorinfo",
		},
		&cli.StringFlag{
			Name:    "spath",
			Usage:   "storage path",
			EnvVars: []string{"STORAGE_PATH"},
			Value:   "/data01/alien/files/t0182363",
		},
		&cli.Int64Flag{
			Name:  "dindex",
			Usage: "deadline index",
			Value: 0,
		},
		&cli.IntFlag{
			Name:  "pinde",
			Usage: "partition index",
			Value: 0,
		},
		&cli.StringFlag{
			Name:    "minerapi",
			Usage:   "miner api",
			EnvVars: []string{"MINER_API"},
			Value:   "ws://127.0.0.1:4321/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 1 {
			return fmt.Errorf("[actorid]")
		}

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()

		ai, err := strconv.ParseUint(cctx.Args().Get(0), 10, 64)
		if err != nil {
			return err
		}
		var actorID uint64 = ai

		t := cctx.Bool("submit")
		pk := ""
		if t {
			fmt.Print("enter private key:")
			buf, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				return err
			}
			fmt.Println()
			pk = string(buf)
		}

		result, err := minerApi.TestWindowPostProofs(actorID, cctx.String("sipath"), cctx.String("spath"), cctx.Int64("ptype"), cctx.Int64("cepoch"), cctx.Int64("dindex"), cctx.Int("pindex"), pk)

		if err == nil {
			log.Info(result)
		}
		return err
	},
}
