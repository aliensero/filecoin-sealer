package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/term"
	"golang.org/x/xerrors"
)

var log = logging.Logger("server")

var sleepDuration = time.Duration(30)

func main() {
	logging.SetLogLevel("server", "INFO")

	prihex := flag.String("prihex", "", "privat key")
	actorID := flag.Int64("actorid", 0, "actorID")
	bin := flag.String("binpath", "ns-worker", "worker execute path")
	minerUrl := flag.String("minerapi", "", "miner api url")
	workerUrl := flag.String("workerapi", "", "worker api url")
	minerTask := flag.Bool("mtask", false, "precommit seed commit")
	workerTask := flag.Bool("wtask", false, "pc1 pc2 c1 c2")
	pc1Task := flag.Bool("pc1", false, "pc1")
	pc2Task := flag.Bool("pc2", false, "pc2")
	c1Task := flag.Bool("c1", false, "c1")
	c2Task := flag.Bool("c2", false, "c2")
	sd := flag.Int("sleep", 0, "sleep interval")
	flag.Parse()

	if time.Duration(*sd) > sleepDuration {
		sleepDuration = time.Duration(*sd)
	}

	if *actorID == 0 {
		panic(xerrors.Errorf("actorID must set"))
	}

	if *minerUrl == "" {
		panic(xerrors.Errorf("miner api must set"))
	}
	if *workerUrl == "" {
		panic(xerrors.Errorf("worker api must set"))
	}

	_, ma, err := util.ConnMiner(*minerUrl)
	if err != nil {
		panic(err)
	}

	_, wa, err := util.ConnectWorker(*workerUrl)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	if *minerTask {

		if *prihex == "" {
			fmt.Print("enter privat key:")
			buf, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				panic(err)
			}
			fmt.Println()
			*prihex = string(buf)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			precommitSrv(ma, *prihex, *actorID)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			getSeedSrv(ma, *actorID)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			commitSrv(ma, *prihex, *actorID)
		}()
	}

	if *workerTask || *pc1Task {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pc1Srv(wa, *actorID, *bin)
		}()
	}

	if *workerTask || *pc2Task {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pc2Srv(wa, *actorID, *bin)
		}()
	}

	if *workerTask || *c1Task {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c1Srv(wa, *actorID, *bin)
		}()
	}

	if *workerTask || *c2Task {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c2Srv(wa, *actorID, *bin)
		}()
	}

	wg.Wait()

}

func precommitSrv(minerapi *util.MinerAPI, prihex string, actorID int64) {
	for {
		ret, err := minerapi.SendPreCommitByPrivatKey(prihex, actorID, -1, "0")
		if err != nil {
			log.Errorf("PrecommitSrv error %v", err)
		} else {
			log.Infof("PrecommitSrv recsult %v", ret)
		}
		time.Sleep(sleepDuration * time.Second)
	}
}

func getSeedSrv(ma *util.MinerAPI, actorID int64) {
	for {
		ret, err := ma.GetSeedRand(actorID, -1)
		if err != nil {
			log.Errorf("getSeedSrv error %v", err)
		} else {
			log.Infof("getSeedSrv recsult %v", ret)
		}
		time.Sleep(sleepDuration * time.Second)
	}
}

func commitSrv(ma *util.MinerAPI, prihex string, actorID int64) {
	for {
		ret, err := ma.SendCommitByPrivatKey(prihex, actorID, -1, "0")
		if err != nil {
			log.Errorf("CommitSrv error %v", err)
		} else {
			log.Infof("CommitSrv recsult %v", ret)
		}
		time.Sleep(sleepDuration * time.Second)
	}
}

func pc1Srv(wa *util.WorkerAPI, actorID int64, bin string) {
	for {
		ret, err := wa.ProcessPrePhase1(actorID, -1, bin)
		if err != nil {
			log.Errorf("pre-phase1 error %v", err)
		} else {
			log.Infof("pre-phase1 recsult %v", ret)
		}
		time.Sleep(sleepDuration * time.Second)
	}
}

func pc2Srv(wa *util.WorkerAPI, actorID int64, bin string) {
	for {
		ret, err := wa.ProcessPrePhase2(actorID, -1, bin)
		if err != nil {
			log.Errorf("pre-phase2 error %v", err)
		} else {
			log.Infof("pre-phase2 recsult %v", ret)
		}
		time.Sleep(sleepDuration * time.Second)
	}
}

func c1Srv(wa *util.WorkerAPI, actorID int64, bin string) {
	for {
		ret, err := wa.ProcessCommitPhase1(actorID, -1, bin)
		if err != nil {
			log.Errorf("phase1 error %v", err)
		} else {
			log.Infof("phase1 recsult %v", ret)
		}
		time.Sleep(sleepDuration * time.Second)
	}
}

func c2Srv(wa *util.WorkerAPI, actorID int64, bin string) {
	for {
		ret, err := wa.ProcessCommitPhase2(actorID, -1, bin)
		if err != nil {
			log.Errorf("phase2 error %v", err)
		} else {
			log.Infof("phase2 recsult %v", ret)
		}
		time.Sleep(sleepDuration * time.Second)
	}
}
