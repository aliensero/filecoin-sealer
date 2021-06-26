package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/filecoin-project/go-jsonrpc"
	"github.com/filecoin-project/go-jsonrpc/auth"
	"github.com/gorilla/mux"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
	"golang.org/x/term"
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
		recoveryCmd,
		taskAutoCmd,
		taskTriggerCmd,
		addPostCmd,
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
			Value: 28800,
		},
		&cli.Int64Flag{
			Name:  "pc2to",
			Usage: "pc2 time out",
			Value: 28800,
		},
		&cli.Int64Flag{
			Name:  "c1to",
			Usage: "c1 time out",
			Value: 28800,
		},
		&cli.Int64Flag{
			Name:  "c2to",
			Usage: "c2 time out",
			Value: 28800,
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
			util.PC1: {},
			util.PC2: {},
			util.C1:  {},
			util.C2:  {},
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
			PoStMiner:     make(map[int64]util.ActorPoStInfo),
			ActorTask:     make(map[int64][]string),
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

		close, minerApi, err := util.ConnMiner(cctx.String("minerapi"))
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
		qr, err := minerApi.QueryOnly(taskInfoWhr)
		if err != nil {
			return err
		}
		if qr.ResultCode == util.Err {
			return xerrors.Errorf(qr.Err)
		}
		taskInfo := qr.Results[0]

		_, _, err = worker.TaskRun(taskInfo, minerApi, cctx.String("minerapi"), cctx.String("session"))
		return err
	},
}

var recoveryCmd = &cli.Command{
	Name: "recovery",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "piececid",
			Value: "baga6ea4seaqao7s73y24kcutaosvacpdjgfe5pw76ooefnyqw4ynr3d2y6x2mpq",
		},
		&cli.StringFlag{
			Name:  "sealerProof",
			Value: "32GiB",
		},
		&cli.Int64Flag{
			Name:  "proofType",
			Value: 8,
		},
		&cli.StringFlag{
			Name: "cacheDir",
		},
		&cli.StringFlag{
			Name: "unsealedDir",
		},
		&cli.StringFlag{
			Name: "sealedFile",
		},
		&cli.Int64Flag{
			Name: "sectorNum",
		},
		&cli.Int64Flag{
			Name: "actorID",
		},
		&cli.StringFlag{
			Name: "ticketHex",
		},

		&cli.StringFlag{
			Name: "seedHex",
		},
	},
	Action: func(cctx *cli.Context) error {
		return worker.RecoverSealedFile(cctx.String("piececid"), cctx.String("sealerProof"), cctx.Int64("proofType"), cctx.String("cacheDir"), cctx.String("unsealedDir"), cctx.String("sealedFile"), cctx.Int64("sectorNum"), cctx.Int64("actorID"), cctx.String("ticketHex"), cctx.String("seedHex"))
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

var taskTriggerCmd = &cli.Command{
	Name: "tasktrig",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "bin",
			Usage:   "bin path",
			EnvVars: []string{"WORKER_BIN_PATH"},
		},
		&cli.BoolFlag{
			Name:  "restart",
			Value: false,
		},
		&cli.StringFlag{
			Name:    "workerapi",
			Usage:   "worker api",
			EnvVars: []string{"WORKER_API"},
			Value:   "ws://127.0.0.1:3456/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 3 {
			return fmt.Errorf("[actorid] [sectornum] [tasktype]")
		}

		close, workerApi, err := util.ConnectWorker(cctx.String("workerapi"))
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

		var result interface{}
		if !cctx.Bool("restart") {
			switch taskType {
			case util.PC1:
				result, err = workerApi.ProcessPrePhase1(actorID, sectorNum, cctx.String("bin"))
			case util.PC2:
				result, err = workerApi.ProcessPrePhase2(actorID, sectorNum, cctx.String("bin"))
			case util.C1:
				result, err = workerApi.ProcessCommitPhase1(actorID, sectorNum, cctx.String("bin"))
			case util.C2:
				result, err = workerApi.ProcessCommitPhase2(actorID, sectorNum, cctx.String("bin"))
			}
		} else {
			result, err = workerApi.RetryTaskPID(actorID, sectorNum, taskType, cctx.String("bin"))
		}
		if err == nil {
			log.Info(result)
		}
		return err
	},
}

var taskAutoCmd = &cli.Command{
	Name:  "taskauto",
	Usage: "[PC1] [PC2] [C1] [C2]",
	Flags: []cli.Flag{
		&cli.Int64Flag{
			Name:  "actorid",
			Usage: "actor ID",
			Value: -1,
		},
		&cli.BoolFlag{
			Name:  "add",
			Usage: "add to server",
		},
		&cli.BoolFlag{
			Name:  "del",
			Usage: "delete from server",
		},
		&cli.BoolFlag{
			Name:  "dis",
			Usage: "display from server",
		},
		&cli.StringFlag{
			Name:    "workerapi",
			Usage:   "worker api",
			EnvVars: []string{"WORKER_API"},
			Value:   "ws://127.0.0.1:3456/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		close, workerApi, err := util.ConnectWorker(cctx.String("workerapi"))
		if err != nil {
			return err
		}
		defer close()

		if cctx.Bool("add") && cctx.Args().Len() < 1 {
			return fmt.Errorf("[PC1] [PC2] [C1] [C2]")
		}

		ai := cctx.Int64("actorid")

		if cctx.Bool("del") && ai != -1 {
			err = workerApi.DeleteFromServer(ai)
		}

		if cctx.Bool("add") && ai != -1 {
			tasks := cctx.Args().Slice()
			err = workerApi.AddToServer(ai, tasks)
		}

		if cctx.Bool("dis") {
			var info interface{}
			info, err = workerApi.DisplayServer(ai)
			if err == nil {
				log.Info(info)
			}
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
			Name:    "workerapi",
			Usage:   "worker api",
			EnvVars: []string{"WORKER_API"},
			Value:   "ws://127.0.0.1:3456/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 2 {
			return fmt.Errorf("[actorid] [address or private key]")
		}

		close, workerApi, err := util.ConnectWorker(cctx.String("workerapi"))
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

		result, err := workerApi.AddPoStActor(actorID, cctx.String("type"), pk)

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
			Name:    "workerapi",
			Usage:   "worker api",
			EnvVars: []string{"WORKER_API"},
			Value:   "ws://127.0.0.1:3456/rpc/v0",
		},
	},
	Action: func(cctx *cli.Context) error {

		if cctx.Args().Len() < 2 {
			return fmt.Errorf("[actorid] [address or private key]")
		}

		close, workerApi, err := util.ConnectWorker(cctx.String("workerapi"))
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

		result, err := workerApi.RetryWinPoSt(actorID, pk, cctx.String("type"))

		if err == nil {
			log.Info(result)
		}
		return err
	},
}
