package main

import (
	"context"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
	"golang.org/x/xerrors"

	"gitlab.ns/lotus-worker/util"
	"gitlab.ns/lotus-worker/worker"
)

func init() {
	_ = logging.SetLogLevel("main", "DEBUG")
}

var log = logging.Logger("main")

func main() {

	local := []*cli.Command{
		runCmd,
		unSealedFileCmd,
		taskRunCmd,
	}

	app := &cli.App{
		Name:  "ns-worker",
		Usage: "Remote worker",

		Commands: local,
	}

	app.Setup()

	if err := app.Run(os.Args); err != nil {
		log.Warnf("%+v", err)
		os.Exit(-1)
	}

}

var runCmd = &cli.Command{

	Name:  "run",
	Usage: "Start ns worker",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "listen",
			Usage: "host address and port the worker api will listen on",
			Value: "127.0.0.1:3456",
		},
		&cli.StringFlag{
			Name:  "extrlisten",
			Usage: "extract host address and port the worker api will listen on",
			Value: "127.0.0.1:3456",
		},
		&cli.StringFlag{
			Name:  "timeout",
			Usage: "used when 'listen' is unspecified. must be a valid duration recognized by golang's time.ParseDuration function",
			Value: "30m",
		},
		&cli.StringFlag{
			Name:  "taskType",
			Usage: "Worker task type",
			Value: "ALL",
		},
		&cli.StringFlag{
			Name:  "minerapi",
			Usage: "miner api",
			Value: "ws://127.0.0.1:1234/rpc/v0",
		},
		&cli.IntFlag{
			Name:  "pc1lmt",
			Usage: "pc1 limit count",
			Value: 1,
		},
		&cli.IntFlag{
			Name:  "pc2lmt",
			Usage: "pc2 limit count",
			Value: 1,
		},
		&cli.IntFlag{
			Name:  "c1lmt",
			Usage: "c1 limit count",
			Value: -1,
		},
		&cli.IntFlag{
			Name:  "c2lmt",
			Usage: "c2 limit count",
			Value: 1,
		},
		&cli.Int64Flag{
			Name:  "pc1to",
			Usage: "pc1 time out",
			Value: 14400,
		},
		&cli.Int64Flag{
			Name:  "pc2to",
			Usage: "pc2 time out",
			Value: 7200,
		},
		&cli.Int64Flag{
			Name:  "c1to",
			Usage: "c1 time out",
			Value: 600,
		},
		&cli.Int64Flag{
			Name:  "c2to",
			Usage: "c2 time out",
			Value: 7200,
		},
		&cli.StringFlag{
			Name:  "workerid",
			Usage: "worker id",
		},
		&cli.StringFlag{
			Name:  "workeridpath",
			Usage: "worker id",
			Value: "/root/miner_storage/workerid",
		},
		&cli.BoolFlag{
			Name:  "post",
			Usage: "post flag",
		},
	},

	Action: func(cctx *cli.Context) error {

		workerID := cctx.String("workerid")
		if workerID == "" {
			wf, err := os.Open(cctx.String("workeridpath"))
			if err != nil {
				return xerrors.Errorf("open workeridpath %s error %v", cctx.String("workeridpath"), err)
			}
			defer wf.Close()
			workerIDs, err := ioutil.ReadAll(wf)
			if err != nil {
				return xerrors.Errorf("read workeridpath %s error %v", cctx.String("workeridpath"), err)
			}
			workerID = string(workerIDs)
		}

		mux := mux.NewRouter()

		rpcServer := jsonrpc.NewServer()

		lmtMap := map[string]int{
			util.PC1: cctx.Int("pc1lmt"),
			util.PC2: cctx.Int("pc2lmt"),
			util.C1:  cctx.Int("c1lmt"),
			util.C2:  cctx.Int("c2lmt"),
		}

		timeOut := map[string]int64{
			util.PC1: cctx.Int64("pc1to"),
			util.PC2: cctx.Int64("pc2to"),
			util.C1:  cctx.Int64("c1to"),
			util.C2:  cctx.Int64("c2to"),
		}

		runMap := map[string]*worker.TaskRuningInfo{
			util.PC1: &worker.TaskRuningInfo{},
			util.PC2: &worker.TaskRuningInfo{},
			util.C1:  &worker.TaskRuningInfo{},
			util.C2:  &worker.TaskRuningInfo{},
		}

		extrListen := cctx.String("extrlisten")
		addressSlice := strings.Split(extrListen, ":")
		if ip := net.ParseIP(addressSlice[0]); ip != nil {
			if ip.String() == "127.0.0.1" {
				rip, err := util.LocalIp()
				if err != nil {
					return err
				}
				extrListen = rip + ":" + addressSlice[1]
			}
		}

		workerInstan := &worker.Worker{
			MinerUrl:      cctx.String("minerapi"),
			TaskLimit:     lmtMap,
			TaskRun:       runMap,
			TaskCloseChnl: make(map[string]chan interface{}),
			WorkerID:      workerID,
			TaskTimeOut:   timeOut,
			ExtrListen:    extrListen,
			PoStMiner:     make(map[int64]worker.ActorPoStInfo),
		}

		rpcServer.Register("NSWORKER", workerInstan)

		mux.Handle("/rpc/v0", rpcServer)
		mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("test"))
		})
		mux.HandleFunc("/get/{type}/{actorid}/{sectornum}", workerInstan.RemoteGetSector).Methods("GET")
		mux.HandleFunc("/del/{type}/{actorid}/{sectornum}", workerInstan.RemoteDelSector).Methods("GET")
		mux.PathPrefix("/").Handler(http.DefaultServeMux)

		ah := &auth.Handler{
			Next: mux.ServeHTTP,
		}

		srv := &http.Server{
			Handler: ah,
			BaseContext: func(listener net.Listener) context.Context {
				return cctx.Context
			},
		}

		address := cctx.String("listen")

		log.Info("Setting up control endpoint at " + address)
		nl, err := net.Listen("tcp", address)
		if err != nil {
			return err
		}
		hostname, err := os.Hostname()
		if err != nil {
			return err
		}
		workerInstan.HostName = hostname
		err = workerInstan.ConnMiner()
		if err != nil {
			return err
		}
		if !cctx.Bool("post") {
			go workerInstan.ResetAbortedSession()
		}
		go workerInstan.WinPoStServer()
		return srv.Serve(nl)
	},
}

var unSealedFileCmd = &cli.Command{
	Name: "unsealedFile",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "sealerProof",
			Value: "32GiB",
		},

		&cli.StringFlag{
			Name:  "path",
			Value: "/tmp/32GiB",
		},
	},
	Action: func(cctx *cli.Context) error {
		w := &worker.Worker{}
		unsealedCID, err := w.AddPiece(cctx.String("sealerProof"), cctx.String("path"), false)
		log.Infof("unsealedCID %s", unsealedCID)
		return err
	},
}

var taskRunCmd = &cli.Command{
	Name: "taskrun",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:  "sectorNum",
			Usage: "sector number",
		},
		&cli.Int64Flag{
			Name:  "actorID",
			Usage: "actor id",
		},
		&cli.StringFlag{
			Name:  "session",
			Usage: "request id",
		},
		&cli.StringFlag{
			Name:  "minerapi",
			Usage: "miner api",
			Value: "ws://127.0.0.1:1234/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		close, minerApi, err := worker.ConnMiner(cctx.String("minerapi"))
		if err != nil {
			return err
		}
		defer close()
		var actorID int64 = cctx.Int64("actorID")
		var sectorNum int64 = cctx.Int64("sectorNum")
		taskInfoWhr := util.DbTaskInfo{
			ActorID:   &actorID,
			SectorNum: &sectorNum,
		}
		taskInfos, err := minerApi.QueryOnly(taskInfoWhr)
		if err != nil {
			return err
		}

		taskInfo := taskInfos[0]

		_, _, err = worker.TaskRun(taskInfo, minerApi, cctx.String("minerapi"), cctx.String("session"))
		return err
	},
}

func extractRoutableIP(timeout time.Duration) (string, error) {
	minerMultiAddrKey := "MINER_API_INFO"
	deprecatedMinerMultiAddrKey := "STORAGE_API_INFO"
	env, ok := os.LookupEnv(minerMultiAddrKey)
	if !ok {
		// TODO remove after deprecation period
		_, ok = os.LookupEnv(deprecatedMinerMultiAddrKey)
		if ok {
			log.Warnf("Using a deprecated env(%s) value, please use env(%s) instead.", deprecatedMinerMultiAddrKey, minerMultiAddrKey)
		}
		return "", xerrors.New("MINER_API_INFO environment variable required to extract IP")
	}
	minerAddr := strings.Split(env, "/")
	conn, err := net.DialTimeout("tcp", minerAddr[2]+":"+minerAddr[4], timeout)
	if err != nil {
		return "", err
	}
	defer conn.Close() //nolint:errcheck

	localAddr := conn.LocalAddr().(*net.TCPAddr)

	return strings.Split(localAddr.IP.String(), ":")[0], nil
}
