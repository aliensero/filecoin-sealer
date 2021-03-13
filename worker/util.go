package worker

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/docker/go-units"
	"github.com/filecoin-project/go-jsonrpc"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func taskHeartBeat(notify chan interface{}, sleeptime time.Duration) chan interface{} {
	closeChan := make(chan interface{}, 1)
	go func() {
	loop:
		for {
			time.Sleep(sleeptime)
			select {
			case notify <- nil:
			case <-closeChan:
				break loop
			}
		}
	}()
	return closeChan
}

func (w *Worker) handleProccess(timeBeat, closeBeat, process, cancel chan interface{}, actorID int64, sectorNum int64, taskType string, processErr *error, session string, cbs ...func()) {

	{

		start := time.Now().UTC()
		duration := w.TaskTimeOut[taskType]
		cbflg := len(cbs) > 0
	loop:
		for {
			select {
			case <-timeBeat:
				log.Infof("actorID %d sectorNum: %d %s task heartbeat now goroutine number %d", actorID, sectorNum, taskType, runtime.NumGoroutine())
				if int64(time.Now().UTC().Sub(start).Seconds()) > duration && cbflg {
					for _, cb := range cbs {
						cb()
					}
					*processErr = xerrors.Errorf("task run timeout")
					break loop
				}
			case err := <-process:
				log.Warnf("actorID %d sectorNum: %d %s task complite error %v", actorID, sectorNum, taskType, err)
				break loop
			case <-cancel:
				log.Warnf("actorID %d sectorNum %d task %s shutdown", actorID, sectorNum, taskType)
				*processErr = xerrors.Errorf("shutdown")
				for _, cb := range cbs {
					cb()
				}
				break loop
			}
		}
	}
	closeBeat <- nil
	delete(w.TaskCloseChnl, session)
	err := w.DecrementTask(taskType)
	log.Warnf("DecrementTask actorID %d sectorNum: %d %s error %v", actorID, sectorNum, taskType, err)
}

func (w *Worker) ShutdownReq(reqSession string) (string, error) {
	if _, ok := w.TaskCloseChnl[reqSession]; ok {
		w.TaskCloseChnl[reqSession] <- nil
	} else {
		return "", xerrors.Errorf("request session %s no found", reqSession)
	}
	return reqSession + " shutdown", nil
}

func deferMinerRecieve(minerApi *util.MinerAPI, minerUrl string, actorID int64, sectorNum int64, taskType string, session string, result []byte, processErr error) (jsonrpc.ClientCloser, error) {

	defer func() {
		if err := recover(); err != nil {
			log.Errorf("deferMinerRecieve painc %v", err)
		}
	}()

	var close jsonrpc.ClientCloser
	var minerApiErr error

	if processErr != nil {
		minerApiErr = minerApi.RecieveTaskResult(actorID, sectorNum, taskType, session, true, []byte(processErr.Error()))
	} else {
		minerApiErr = minerApi.RecieveTaskResult(actorID, sectorNum, taskType, session, false, result)
	}

	if minerApiErr != nil {
		var reConnErr error
		for i := 0; i < 5; i++ {
			log.Errorf("MinerApi.RecieveTaskResult actorID %d taskType %s session %s error %v", actorID, taskType, session, minerApiErr)
			log.Warnf("MinerApi.RecieveTaskResult actorID %d taskType %s session %s retry", actorID, taskType, session)
			if minerApi != nil {
				if !minerApi.CheckServer() {
					close, minerApi, reConnErr = ConnMiner(minerUrl)
				}
			}
			if reConnErr != nil {
				log.Warnf("MinerApi.RecieveTaskResult actorID %d taskType %s session %s reConnect %d error %v", actorID, taskType, session, i, reConnErr)
			} else {
				if processErr != nil {
					minerApiErr = minerApi.RecieveTaskResult(actorID, sectorNum, taskType, session, true, []byte(processErr.Error()))
				} else {
					minerApiErr = minerApi.RecieveTaskResult(actorID, sectorNum, taskType, session, false, result)
				}
				if minerApiErr == nil {
					break
				}
			}
		}
	}
	log.Infof("worker util deferMinerRecieve complite actorID %d sectorNum %d taskType %s", actorID, sectorNum, taskType)
	return close, minerApiErr

}

func ConnMiner(minerUrl string) (jsonrpc.ClientCloser, *util.MinerAPI, error) {

	var mi util.MinerAPI
	closer, err := jsonrpc.NewMergeClient(context.TODO(), minerUrl, "NSMINER",
		[]interface{}{
			&mi,
		},
		nil,
		jsonrpc.WithNoReconnect(),
	)
	if err != nil {
		log.Error(err)
		return nil, nil, err
	}
	log.Infof("lotusMiner connect error %v", err)
	return closer, &mi, nil
}

func TaskProcess(binPath string, minerapi string, session string, sectorNum int64, actorID int64) (*exec.Cmd, error) {

	var cmd *exec.Cmd
	cmd = exec.Command(binPath, "taskrun", "--minerapi", minerapi, "--session", session, "--sectorNum", strconv.FormatInt(int64(sectorNum), 10), "--actorID", strconv.FormatInt(int64(actorID), 10))
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Infof("cmd string %s", cmd.String())
	var err error
	go func() {
		err = cmd.Start()
		if err != nil {
			return
		}
	}()
	return cmd, err
}

func TaskRun(taskInfo util.DbTaskInfo, minerApi *util.MinerAPI, minerUrl string, session string) ([]byte, error, error) {

	var result []byte
	var minerAPIErr, err error
	defer func() {
		if errReserve := recover(); errReserve != nil {
			log.Errorf("TaskRun painc %v", errReserve)
			err, _ = errReserve.(error)
		}
		var close jsonrpc.ClientCloser
		close, minerAPIErr = deferMinerRecieve(minerApi, minerUrl, *taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.TaskType, session, result, err)
		if close != nil {
			close()
		}
		if minerAPIErr == nil && taskInfo.TaskType == util.C1 {
			err1 := delCache(taskInfo.CacheDirPath)
			if err1 != nil {
				log.Warnf("sealCommitPhase1 delCache %s error %v", taskInfo.CacheDirPath, err1)
			}
		}
	}()

	switch taskInfo.TaskType {

	case util.PC1:
		sectorSizeInt, err1 := units.RAMInBytes(taskInfo.SealerProof)
		err = err1
		if err1 != nil {
			log.Error(xerrors.Errorf("error parsing sector size (specify as \"32GiB\", for instance): %w", err1))
			return result, minerAPIErr, err1
		}

		var ticketBtyes []byte
		if taskInfo.TicketHex == "" {
			ticketBtyes1, err1 := minerApi.GetTicket(*taskInfo.ActorID, *taskInfo.SectorNum)
			err = err1
			if err1 != nil {
				return result, minerAPIErr, err1
			}
			ticketBtyes = ticketBtyes1
		} else {
			ticketBtyes1, err1 := hex.DecodeString(taskInfo.TicketHex)
			err = err1
			if err1 != nil {
				return result, minerAPIErr, err1
			}
			ticketBtyes = ticketBtyes1
		}

		pieces, err1 := util.NewNsPieceInfo(taskInfo.PieceStr, sectorSizeInt)
		err = err1
		if err1 != nil {
			return result, minerAPIErr, err1
		}

		phase1Out, err1 := util.NsSealPreCommitPhase1(util.NsRegisteredSealProof(taskInfo.ProofType), taskInfo.CacheDirPath, taskInfo.StagedSectorPath, taskInfo.SealedSectorPath, util.NsSectorNum(*taskInfo.SectorNum), util.NsActorID(*taskInfo.ActorID), util.NsSealRandomness(ticketBtyes[:]), pieces)
		result = phase1Out
		err = err1
		if err1 != nil {
			return result, minerAPIErr, err1
		}
		log.Infof("TaskRun PC1 phase1OutHex %v", hex.EncodeToString(phase1Out))
	case util.PC2:
		log.Infof("sealPreCommitPhase2 sectorNum %d, minerID %d, cacheDirPath %s, sealedSectorPath %s", *taskInfo.SectorNum, *taskInfo.ActorID, taskInfo.CacheDirPath, taskInfo.SealedSectorPath)
		phase1Output := taskInfo.Phase1Output
		sealedCID, unsealedCID, err1 := util.NsSealPreCommitPhase2(phase1Output, taskInfo.CacheDirPath, taskInfo.SealedSectorPath)
		result = []byte(fmt.Sprintf("%s-%s", sealedCID.String(), unsealedCID.String()))
		err = err1
		if err1 != nil {
			return result, minerAPIErr, err1
		}
		log.Infof("TaskRun PC2 phase2Out %v", fmt.Sprintf("%s-%s", sealedCID.String(), unsealedCID.String()))

	case util.C1:

		sectorSizeInt, err1 := units.RAMInBytes(taskInfo.SealerProof)
		err = err1
		if err1 != nil {
			log.Error(xerrors.Errorf("error parsing sector size (specify as \"32GiB\", for instance): %w", err1))
			return result, minerAPIErr, err1
		}

		proofType := util.NsRegisteredSealProof(taskInfo.ProofType)
		sealedCID, err1 := util.Parse(taskInfo.CommR)
		err = err1
		if err1 != nil {
			log.Errorf("TaskRun C1 parse sealedCID error %v", err1)
			return result, minerAPIErr, err1
		}
		unsealedCID, err1 := util.Parse(taskInfo.CommD)
		err = err1
		if err1 != nil {
			log.Errorf("TaskRun C1 parse unsealedCID error %v", err1)
			return result, minerAPIErr, err1
		}
		ticketBytes, err1 := hex.DecodeString(taskInfo.TicketHex)
		err = err1
		if err1 != nil {
			log.Errorf("TaskRun C1 parse ticketBytes error %v", err1)
			return result, minerAPIErr, err1
		}
		seedBytes, err1 := hex.DecodeString(taskInfo.SeedHex)
		err = err1
		if err1 != nil {
			log.Errorf("TaskRun C1 parse seedBytes error %v", err1)
			return result, minerAPIErr, err1
		}

		pieces, err1 := util.NewNsPieceInfo(taskInfo.PieceStr, sectorSizeInt)
		err = err1
		if err1 != nil {
			log.Error(err)
			return result, minerAPIErr, err1
		}

		log.Infof(" params proofType %d sealedCID %s unsealedCID %s cacheDirPath %s sealedSectorPath %s sealedSectorPath sectorNum %d minerID %d ticket %v seed %v pieces %v", proofType, sealedCID, unsealedCID, taskInfo.CacheDirPath, taskInfo.SealedSectorPath, *taskInfo.ActorID, *taskInfo.SectorNum, ticketBytes, seedBytes, pieces)

		phase1Output, err1 := util.NsSealCommitPhase1(proofType, sealedCID, unsealedCID, taskInfo.CacheDirPath, taskInfo.SealedSectorPath, util.NsSectorNum(*taskInfo.SectorNum), util.NsActorID(*taskInfo.ActorID), util.NsSealRandomness(ticketBytes[:]), util.NsSeed(seedBytes[:]), pieces)
		result = phase1Output
		err = err1
		if err1 != nil {
			return result, minerAPIErr, err1
		}

	case util.C2:
		phase1Output := taskInfo.C1Out
		phase2Out, err1 := util.NsSealCommitPhase2(phase1Output, util.NsSectorNum(*taskInfo.SectorNum), util.NsActorID(*taskInfo.ActorID))
		result = phase2Out
		err = err1
		if err1 != nil {
			return result, minerAPIErr, err1
		}
	}

	return result, minerAPIErr, err
}
