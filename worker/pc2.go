package worker

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func (w *Worker) SealPreCommitPhase2(actorID int64, sectorNum int64) (string, error) {

	var err error
	var session = uuid.New().String()
	taskType := util.PC2

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
		log.Errorf("worker SealPreCommitPhase2 error %v", err)
		err2 := w.ConnMiner()
		if err2 != nil {
			return "MinerApi reconnect please retry", err2
		}
		log.Info("MinerApi reconnect please retry")
		return session, err
	}

	if err != nil && strings.Contains(err.Error(), "record not found") {
		log.Warnf("worker SealPreCommitPhase2 record not found")
		return "", xerrors.Errorf("%v", err)
	}

	go w.sealPreCommitPhase2(taskInfo, session)

	return session, nil
}

func (w *Worker) sealPreCommitPhase2(
	taskInfo util.DbTaskInfo,
	session string,
) (util.NsCid, util.NsCid, error) {

	taskType := util.PC2
	timebeat := make(chan interface{}, 1)
	process := make(chan interface{}, 1)
	stopchnl := make(chan interface{}, 1)

	w.TaskCloseChnl[session] = stopchnl

	closeBeat := taskHeartBeat(timebeat, 30*time.Second)

	var err error
	var sealedCID, unsealedCID util.NsCid

	go func() {

		defer func() {
			process <- err
		}()

		log.Infof("sealPreCommitPhase2 sectorNum %d, minerID %d, cacheDirPath %s, sealedSectorPath %s", *taskInfo.SectorNum, *taskInfo.ActorID, taskInfo.CacheDirPath, taskInfo.SealedSectorPath)

		phase1Output := taskInfo.Phase1Output
		sealedCID, unsealedCID, err = util.NsSealPreCommitPhase2(phase1Output, taskInfo.CacheDirPath, taskInfo.SealedSectorPath)

	}()
	w.handleProccess(timebeat, closeBeat, process, stopchnl, *taskInfo.ActorID, *taskInfo.SectorNum, taskType, &err, session)

	w.DeferMinerRecieve(*taskInfo.ActorID, *taskInfo.SectorNum, taskType, session, util.TaskResult{SealedCID: sealedCID.String(), UnsealedCID: unsealedCID.String()}, err)

	return sealedCID, unsealedCID, err
}

func (w *Worker) ProcessPrePhase2(actorID int64, sectorNum int64, binPath string) (util.ChildProcessInfo, error) {
	session := uuid.New().String()

	var err error
	taskType := util.PC2

	err = w.IncrementTask(taskType)
	if err != nil {
		return util.ChildProcessInfo{}, err
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
		SectorNum:    sectorNum,
		WorkerListen: w.ExtrListen,
	}

	taskInfo, err = w.queryTask(reqInfo)
	if err != nil {
		log.Info("MinerApi reconnect please retry")
		return util.ChildProcessInfo{}, err
	}

	defer func() {
		if err != nil {
			w.DeferMinerRecieve(*taskInfo.ActorID, *taskInfo.SectorNum, taskType, session, util.TaskResult{}, err)
		}
	}()

	childProcessInfo, err := w.processPrePhase2(taskInfo, binPath, session)

	return childProcessInfo, err
}

func (w *Worker) processPrePhase2(taskInfo util.DbTaskInfo, binPath, session string) (util.ChildProcessInfo, error) {
	pid, err := w.ChildProcess(taskInfo, binPath, session)
	return util.ChildProcessInfo{ActorID: *taskInfo.ActorID, SectorNum: *taskInfo.SectorNum, Pid: pid}, err
}
