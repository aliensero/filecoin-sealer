package worker

import (
	"encoding/hex"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/google/uuid"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func (w *Worker) SealCommitPhase1(actorID int64, sectorNum int64) (string, error) {

	var err error
	var session = uuid.New().String()
	taskType := util.C1

	err = w.IncrementTask(taskType)
	if err != nil {
		return "", err
	}

	defer func() {
		if err != nil {
			log.Warnf("Task %s runing %d", taskType, w.TaskRun[taskType].RuningCnt)
			err = w.DecrementTask(taskType)
			log.Warnf("DecrementTask %s runing %d error %v", taskType, w.TaskRun[taskType].RuningCnt, err)
		}
	}()

	reqInfo := util.RequestInfo{
		ActorID:      actorID,
		SectorNum:    sectorNum,
		TaskType:     taskType,
		WorkerID:     w.WorkerID,
		HostName:     w.HostName,
		Session:      session,
		WorkerListen: w.ExtrListen,
	}

	taskInfo, err := w.queryTask(reqInfo)

	if err != nil && !strings.Contains(err.Error(), "record not found") {
		log.Errorf("worker SealCommitPhase1 error %v", err)
		err2 := w.ConnMiner()
		if err2 != nil {
			return "MinerApi reconnect please retry", err2
		}
		log.Info("MinerApi reconnect please retry")
		return session, err
	}

	if err != nil && strings.Contains(err.Error(), "record not found") {
		log.Warnf("worker SealCommitPhase1 record not found")
		return "", err
	}

	go w.sealCommitPhase1(taskInfo, session)

	return session, nil
}

func (w *Worker) sealCommitPhase1(
	taskInfo util.DbTaskInfo,
	session string,
) ([]byte, error) {

	timebeat := make(chan interface{}, 1)
	process := make(chan interface{}, 1)
	stopchnl := make(chan interface{}, 1)

	w.TaskCloseChnl[session] = stopchnl

	closeBeat := taskHeartBeat(timebeat, 30*time.Second)

	var minerAPIErr, err error
	var phase1Output []byte

	go func() {

		defer func() {
			process <- err
		}()

		sectorSizeInt, err1 := units.RAMInBytes(taskInfo.SealerProof)
		err = err1
		if err1 != nil {
			log.Error(xerrors.Errorf("error parsing sector size (specify as \"32GiB\", for instance): %w", err1))
			return
		}

		proofType := util.NsRegisteredSealProof(taskInfo.ProofType)
		sealedCID, err1 := util.Parse(taskInfo.CommR)
		err = err1
		if err1 != nil {
			log.Errorf("worker SealCommitPhase1 parse sealedCID error %v", err1)
			return
		}
		unsealedCID, err1 := util.Parse(taskInfo.CommD)
		err = err1
		if err1 != nil {
			log.Errorf("worker SealCommitPhase1 parse unsealedCID error %v", err1)
			return
		}
		ticketBytes, err1 := hex.DecodeString(taskInfo.TicketHex)
		err = err1
		if err != nil {
			log.Errorf("worker SealCommitPhase1 parse ticketBytes error %v", err1)
			return
		}
		seedBytes, err1 := hex.DecodeString(taskInfo.SeedHex)
		err = err1
		if err1 != nil {
			log.Errorf("worker SealCommitPhase1 parse seedBytes error %v", err1)
			return
		}

		pieces, err1 := util.NewNsPieceInfo(taskInfo.PieceStr, sectorSizeInt)
		err = err1
		if err1 != nil {
			log.Error(err1)
			return
		}

		log.Infof(" params proofType %d sealedCID %s unsealedCID %s cacheDirPath %s sealedSectorPath %s sealedSectorPath sectorNum %d minerID %d ticket %v seed %v pieces %v", proofType, sealedCID, unsealedCID, taskInfo.CacheDirPath, taskInfo.SealedSectorPath, *taskInfo.ActorID, *taskInfo.SectorNum, ticketBytes, seedBytes, pieces)

		phase1Output, err = util.NsSealCommitPhase1(proofType, sealedCID, unsealedCID, taskInfo.CacheDirPath, taskInfo.SealedSectorPath, util.NsSectorNum(*taskInfo.SectorNum), util.NsActorID(*taskInfo.ActorID), util.NsSealRandomness(ticketBytes[:]), util.NsSeed(seedBytes[:]), pieces)
		if err != nil {
			minerAPIErr = w.MinerApi.RecieveTaskResult(*taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.TaskType, session, true, []byte(err.Error()))
			return
		}
		if err == nil {
			err1 := delCache(taskInfo.CacheDirPath)
			if err1 != nil {
				log.Warnf("sealCommitPhase1 delCache %s error %v", taskInfo.CacheDirPath, err1)
			}
		}
	}()
	w.handleProccess(timebeat, closeBeat, process, stopchnl, *taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.TaskType, &err, session)

	w.DeferMinerRecieve(*taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.TaskType, session, phase1Output, err)

	return phase1Output, err
}

func (w *Worker) ProcessCommitPhase1(actorID int64, sectorNum int, binPath string) (ChildProcessInfo, error) {
	session := uuid.New().String()

	var err error

	taskType := util.C1

	err = w.IncrementTask(taskType)
	if err != nil {
		return ChildProcessInfo{}, err
	}

	defer func() {
		if err != nil {
			log.Warnf("Task %s runing %d", taskType, w.TaskRun[taskType].RuningCnt)
			err = w.DecrementTask(taskType)
			log.Warnf("DecrementTask %s runing %d error %v", taskType, w.TaskRun[taskType].RuningCnt, err)
		}
	}()

	var taskInfo util.DbTaskInfo
	reqInfo := util.RequestInfo{
		ActorID:      actorID,
		TaskType:     taskType,
		WorkerID:     w.WorkerID,
		HostName:     w.HostName,
		Session:      session,
		SectorNum:    int64(sectorNum),
		WorkerListen: w.ExtrListen,
	}

	taskInfo, err = w.queryTask(reqInfo)
	if err != nil && !strings.Contains(err.Error(), "record not found") {
		log.Errorf("worker %s error %v", taskType, err)
		err2 := w.ConnMiner()
		if err2 != nil {
			return ChildProcessInfo{}, err2
		}
		log.Info("MinerApi reconnect please retry")
		return ChildProcessInfo{}, err
	}

	if err != nil && strings.Contains(err.Error(), "record not found") {
		return ChildProcessInfo{}, err
	}

	defer func() {
		if err != nil {
			w.DeferMinerRecieve(*taskInfo.ActorID, *taskInfo.SectorNum, taskType, session, []byte{}, err)
		}
	}()

	childProcessInfo, err := w.processCommitPhase1(taskInfo, binPath, session)

	return childProcessInfo, err
}

func (w *Worker) processCommitPhase1(taskInfo util.DbTaskInfo, binPath, session string) (ChildProcessInfo, error) {
	pid, err := w.ChildProcess(taskInfo, binPath, session)
	return ChildProcessInfo{*taskInfo.ActorID, *taskInfo.SectorNum, pid}, err
}

func (w *Worker) SealCommitPhase2(actorID int64, sectorNum int64) (string, error) {

	var err error
	var session = uuid.New().String()
	taskType := util.C2

	err = w.IncrementTask(taskType)
	if err != nil {
		return "", err
	}

	defer func() {
		if err != nil {
			log.Warnf("Task %s runing %d", taskType, w.TaskRun[taskType].RuningCnt)
			err = w.DecrementTask(taskType)
			log.Warnf("DecrementTask %s runing %d error %v", taskType, w.TaskRun[taskType].RuningCnt, err)
		}
	}()

	reqInfo := util.RequestInfo{
		ActorID:      actorID,
		SectorNum:    sectorNum,
		TaskType:     taskType,
		WorkerID:     w.WorkerID,
		HostName:     w.HostName,
		Session:      session,
		WorkerListen: w.ExtrListen,
	}

	taskInfo, err := w.queryTask(reqInfo)

	if err != nil && !strings.Contains(err.Error(), "record not found") {
		log.Errorf("worker SealCommitPhase2 error %v", err)
		err2 := w.ConnMiner()
		if err2 != nil {
			return "MinerApi reconnect please retry", err2
		}
		log.Info("MinerApi reconnect please retry")
		return session, err
	}

	if err != nil && strings.Contains(err.Error(), "record not found") {
		log.Warnf("worker SealCommitPhase2 record not found")
		return "", err
	}

	go w.sealCommitPhase2(taskInfo, session)

	return session, nil
}

func (w *Worker) sealCommitPhase2(
	taskInfo util.DbTaskInfo,
	session string,
) ([]byte, error) {

	timebeat := make(chan interface{}, 1)
	process := make(chan interface{}, 1)
	stopchnl := make(chan interface{}, 1)

	w.TaskCloseChnl[session] = stopchnl

	closeBeat := taskHeartBeat(timebeat, 30*time.Second)

	var err error
	var phase2Out []byte

	go func() {

		defer func() {
			process <- err
		}()

		// c1File, err := os.Open(taskInfo.C1OutPath)

		// if err != nil {
		// 	return
		// }
		// buf := make([]byte, 1024*1024*5)
		// cnt, err := c1File.Read(buf)

		// if err != nil {
		// 	return
		// }
		phase1Output := taskInfo.C1Out
		phase2Out, err = util.NsSealCommitPhase2(phase1Output, util.NsSectorNum(*taskInfo.SectorNum), util.NsActorID(*taskInfo.ActorID))

	}()

	w.handleProccess(timebeat, closeBeat, process, stopchnl, *taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.TaskType, &err, session)

	w.DeferMinerRecieve(*taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.TaskType, session, phase2Out, err)

	return phase2Out, err

}

func (w *Worker) ProcessCommitPhase2(actorID int64, sectorNum int, binPath string) (ChildProcessInfo, error) {
	session := uuid.New().String()

	var err error

	taskType := util.C2

	err = w.IncrementTask(taskType)
	if err != nil {
		return ChildProcessInfo{}, err
	}

	defer func() {
		if err != nil {
			log.Warnf("Task %s runing %d", taskType, w.TaskRun[taskType].RuningCnt)
			err = w.DecrementTask(taskType)
			log.Warnf("DecrementTask %s runing %d error %v", taskType, w.TaskRun[taskType].RuningCnt, err)
		}
	}()

	var taskInfo util.DbTaskInfo

	reqInfo := util.RequestInfo{
		ActorID:      actorID,
		TaskType:     taskType,
		WorkerID:     w.WorkerID,
		HostName:     w.HostName,
		Session:      session,
		SectorNum:    int64(sectorNum),
		WorkerListen: w.ExtrListen,
	}

	taskInfo, err = w.queryTask(reqInfo)
	if err != nil && !strings.Contains(err.Error(), "record not found") {
		log.Errorf("worker %s error %v", taskType, err)
		err2 := w.ConnMiner()
		if err2 != nil {
			return ChildProcessInfo{}, err2
		}
		log.Info("MinerApi reconnect please retry")
		return ChildProcessInfo{}, err
	}

	if err != nil && strings.Contains(err.Error(), "record not found") {
		return ChildProcessInfo{}, err
	}

	defer func() {
		if err != nil {
			w.DeferMinerRecieve(*taskInfo.ActorID, *taskInfo.SectorNum, taskType, session, []byte{}, err)
		}
	}()

	childProcessInfo, err := w.processCommitPhase2(taskInfo, binPath, session)

	return childProcessInfo, err
}

func (w *Worker) processCommitPhase2(taskInfo util.DbTaskInfo, binPath, session string) (ChildProcessInfo, error) {
	pid, err := w.ChildProcess(taskInfo, binPath, session)
	return ChildProcessInfo{*taskInfo.ActorID, *taskInfo.SectorNum, pid}, err
}
