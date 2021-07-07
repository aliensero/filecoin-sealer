package worker

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/go-units"
	"github.com/google/uuid"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func (w *Worker) SealPreCommitPhase1(actorID int64, sectorNum int64) (string, error) {

	var session = uuid.New().String()
	var err error

	taskType := util.PC1

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
	if err != nil {
		return "", err
	}

	go w.sealPreCommitPhase1(taskInfo, session)
	return session, nil
}

func (w *Worker) sealPreCommitPhase1(
	taskInfo util.DbTaskInfo,
	session string,
) ([]byte, error) {

	taskType := util.PC1
	timebeat := make(chan interface{}, 1)
	process := make(chan interface{}, 1)
	stopchnl := make(chan interface{}, 1)

	w.TaskCloseChnl[session] = stopchnl

	closeBeat := taskHeartBeat(timebeat, 30*time.Second)

	var err error
	var phase1Output []byte

	go func() {

		var err error
		defer func() {
			process <- err
		}()

		sectorSizeInt, err := units.RAMInBytes(taskInfo.SealerProof)

		if err != nil {
			log.Error(xerrors.Errorf("error parsing sector size (specify as \"32GiB\", for instance): %w", err))
			return
		}

		var ticketBtyes []byte
		if taskInfo.TicketHex == "" {
			ticketBtyes1, err := w.MinerApi.GetTicket(*taskInfo.ActorID, *taskInfo.SectorNum)
			if err != nil {
				log.Error(err)
				return
			}
			ticketBtyes = ticketBtyes1
		} else {
			ticketBtyes1, err := hex.DecodeString(taskInfo.TicketHex)
			if err != nil {
				log.Error(err)
				return
			}
			ticketBtyes = ticketBtyes1
		}

		ticket := util.NsSealRandomness(ticketBtyes[:])

		pieces, err := util.NewNsPieceInfo(taskInfo.PieceStr, sectorSizeInt)

		if err != nil {
			log.Error(err)
			return
		}

		sealedBasePath := filepath.Dir(taskInfo.SealedSectorPath)

		if err := os.MkdirAll(sealedBasePath, 0755); err != nil { // nolint

			if !os.IsExist(err) {
				if err := os.Mkdir(sealedBasePath, 0755); err != nil { // nolint:gosec
					log.Error(xerrors.Errorf("mkdir sealed path %s error: %v", sealedBasePath, err))
					return
				}
			}
		}

		e, err := os.OpenFile(taskInfo.SealedSectorPath, os.O_RDWR|os.O_CREATE, 0644) // nolint:gosec

		if err != nil {
			log.Error(xerrors.Errorf("ensuring sealed file exists: %w", err))
			return
		}
		if err := e.Close(); err != nil {

			log.Error(err)
			return
		}

		if err := os.MkdirAll(taskInfo.CacheDirPath, 0755); err != nil { // nolint

			if !os.IsExist(err) {
				log.Error(xerrors.Errorf("mkdir cache path %s error: %v", taskInfo.CacheDirPath, err))
				return
			}
		}

		proofType := util.NsRegisteredSealProof(taskInfo.ProofType)
		cacheDirPath := taskInfo.CacheDirPath
		stagedSectorPath := taskInfo.StagedSectorPath
		sealedSectorPath := taskInfo.SealedSectorPath
		sectorNum := util.NsSectorNum(*taskInfo.SectorNum)
		minerID := util.NsActorID(*taskInfo.ActorID)

		log.Info("sealPreCommitPhase1 pieces[]", pieces)

		phase1Output, err = util.NsSealPreCommitPhase1(proofType, cacheDirPath, stagedSectorPath, sealedSectorPath, sectorNum, minerID, ticket, pieces)
	}()

	w.handleProccess(timebeat, closeBeat, process, stopchnl, *taskInfo.ActorID, *taskInfo.SectorNum, taskType, &err, session)

	w.DeferMinerRecieve(*taskInfo.ActorID, *taskInfo.SectorNum, taskType, session, util.TaskResult{CommBytes: phase1Output}, err)

	return phase1Output, err
}

func (w *Worker) ProcessPrePhase1(actorID int64, sectorNum int, binPath string) (util.ChildProcessInfo, error) {

	session := uuid.New().String()

	var err error
	taskType := util.PC1

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
		SectorNum:    int64(sectorNum),
		WorkerListen: w.ExtrListen,
	}

	taskInfo, err = w.queryTask(reqInfo)
	if err != nil {
		return util.ChildProcessInfo{}, err
	}

	defer func() {
		if err != nil {
			w.DeferMinerRecieve(*taskInfo.ActorID, *taskInfo.SectorNum, taskType, session, util.TaskResult{}, err)
		}
	}()
	childProcessInfo, err := w.processPrePhase1(taskInfo, binPath, session)

	return childProcessInfo, err
}

func (w *Worker) processPrePhase1(taskInfo util.DbTaskInfo, binPath, session string) (util.ChildProcessInfo, error) {

	var err error
	sealedBasePath := filepath.Dir(taskInfo.SealedSectorPath)

	if err := os.MkdirAll(sealedBasePath, 0755); err != nil { // nolint
		if !os.IsExist(err) {
			if err := os.Mkdir(sealedBasePath, 0755); err != nil { // nolint:gosec
				log.Error(xerrors.Errorf("mkdir sealed path %s error: %v", sealedBasePath, err))
				return util.ChildProcessInfo{}, err
			}
		}
	}

	e, err := os.OpenFile(taskInfo.SealedSectorPath, os.O_RDWR|os.O_CREATE, 0644) // nolint:gosec

	if err != nil {
		log.Error(xerrors.Errorf("ensuring sealed file %s exists: %w", taskInfo.SealedSectorPath, err))
		return util.ChildProcessInfo{}, err
	}
	if err := e.Close(); err != nil {
		log.Error(err)
		return util.ChildProcessInfo{}, err
	}

	if err := os.MkdirAll(taskInfo.CacheDirPath, 0755); err != nil { // nolint
		if !os.IsExist(err) {
			log.Error(xerrors.Errorf("mkdir cache path %s error: %v", taskInfo.CacheDirPath, err))
			return util.ChildProcessInfo{}, err
		}
	}

	pid, err := w.ChildProcess(taskInfo, binPath, session)

	return util.ChildProcessInfo{ActorID: *taskInfo.ActorID, SectorNum: *taskInfo.SectorNum, Pid: pid}, err
}
