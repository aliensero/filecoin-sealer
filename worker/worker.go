package worker

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/go-units"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/google/uuid"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"

	logging "github.com/ipfs/go-log/v2"
)

func init() {
	_ = logging.SetLogLevel("worker-struct", "DEBUG")
}

var log = logging.Logger("worker-struct")

type Worker struct {
	WorkerID   string
	HostName   string
	ExtrListen string

	MinerApi    *util.MinerAPI
	MinerUrl    string
	MinerCloser jsonrpc.ClientCloser

	FialedMap map[string]FailedTask

	TaskRun   map[string]*TaskRuningInfo
	TaskLimit map[string]int

	TaskCloseChnl map[string]chan interface{}

	TaskTimeOut map[string]int64

	PoStMiner map[int64]util.ActorPoStInfo
}

type FailedTask struct {
	ActorID   int64
	SectorNum int64
	TaskType  string
	IsErr     bool
	Result    []byte
}

type TaskRuningInfo struct {
	LmtLock   sync.Mutex
	RuningCnt int
}

type Reader struct{}

func (Reader) Read(out []byte) (int, error) {
	for i := range out {
		out[i] = 0
	}
	return len(out), nil
}

func (w *Worker) AddPiece(sealerProof string, path string, async bool) (string, error) {
	if async {
		var session = uuid.New()
		go w.addPiece(sealerProof, path, session.String())
		return session.String(), nil
	} else {
		pieceInfo, err := w.addPiece(sealerProof, path, "")
		if err != nil {
			return "", err
		}
		return pieceInfo.PieceCID.String(), nil
	}
}

func (w *Worker) PieceInfo(path string) (util.NsPieceInfo, error) {
	var pi util.NsPieceInfo
	{
		file, err := os.OpenFile(path, os.O_RDWR, 0666)
		if err != nil {
			return util.NsPieceInfo{}, err
		}
		defer file.Close()
		err = pi.UnmarshalCBOR(file)
		if err != nil {
			return util.NsPieceInfo{}, err
		}
	}
	return pi, nil
}

func (w *Worker) addPiece(sealerProof string, path string, sessiong string) (util.NsPieceInfo, error) {

	sectorSizeInt, err := units.RAMInBytes(sealerProof)
	if err != nil {
		return util.NsPieceInfo{}, xerrors.Errorf("error parsing sector size (specify as \"32GiB\", for instance): %w", err)
	}

	reader := (io.LimitReader(&Reader{}, sectorSizeInt)).(*io.LimitedReader)

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0775)
	if err != nil {
		return util.NsPieceInfo{}, err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return util.NsPieceInfo{}, err
	}

	sp := util.NsFilRegisteredSealProof(sectorSizeInt)
	file.Seek(0, io.SeekStart)
	size := util.NsPaddedPieceSize(sectorSizeInt)
	resp := util.NsFilGeneratePieceCommitment(sp, int32(file.Fd()), uint64(size.Unpadded()))
	resp.Deref()

	defer util.NsFilDestroyGeneratePieceCommitmentResponse(resp)

	if resp.StatusCode != util.NsFCPResponseStatusFCPNoError {
		return util.NsPieceInfo{}, errors.New(util.NsRawString(resp.ErrorMsg).Copy())
	}

	pieceCID, err := util.NsPieceCommitmentV1ToCID(resp.CommP[:])
	if err != nil {
		return util.NsPieceInfo{}, xerrors.Errorf("generate unsealed CID: %w", err)
	}

	// validate that the pieceCID was properly formed
	if _, err := util.NsCIDToPieceCommitmentV1(pieceCID); err != nil {
		return util.NsPieceInfo{}, err
	}

	return util.NsPieceInfo{
		Size:     size,
		PieceCID: pieceCID,
	}, nil
}

func (w *Worker) ConnMiner() (err error) {

	minerAlive := false
	defer func() {
		if err1 := recover(); err1 != nil {
			err = err1.(error)
			log.Errorf("worker.ConnMiner panic %v", err1)
			w.MinerApi = nil
			w.MinerCloser = nil
			minerAlive = false
		}
		if !minerAlive {
			closer, mi, err1 := util.ConnMiner(w.MinerUrl)
			err = err1
			if err1 != nil {
				log.Errorf("worker.ConnMiner error %v", err1)
				return
			}
			w.MinerCloser = closer
			w.MinerApi = mi

			_, err = w.MinerApi.WorkerLogin(w.WorkerID, w.HostName, w.ExtrListen)
			log.Warnf("worker.WorkerLogin %v", err)
		}
	}()

	if w.MinerApi != nil {
		minerAlive = w.MinerApi.CheckServer()
		log.Infof("w.MinerApi minerAlive %v", minerAlive)
		if minerAlive {
			return
		}
	}
	if w.MinerCloser != nil {
		w.MinerCloser()
		w.MinerApi = nil
	}
	return
}

func (w *Worker) FailedTask() map[string]FailedTask {
	return w.FialedMap
}

func (w *Worker) IncrementTask(taskType string) error {

	if w.TaskLimit[taskType] == -1 {
		return nil
	}

	if w.TaskLimit[taskType] <= w.TaskRun[taskType].RuningCnt {
		return xerrors.Errorf("task greater than taskType %s limit %d,now running %d", taskType, w.TaskLimit[taskType], w.TaskRun[taskType].RuningCnt)
	}

	// mux := w.TaskRun[taskType].LmtLock
	w.TaskRun[taskType].LmtLock.Lock()
	defer w.TaskRun[taskType].LmtLock.Unlock()
	if w.TaskLimit[taskType] <= w.TaskRun[taskType].RuningCnt {
		return xerrors.Errorf("task greater than taskType %s limit %d,now running %d", taskType, w.TaskLimit[taskType], w.TaskRun[taskType].RuningCnt)
	}
	rif := w.TaskRun[taskType]
	rif.RuningCnt += 1

	return nil
}

func (w *Worker) DecrementTask(taskType string) error {

	if _, ok := w.TaskLimit[taskType]; !ok {
		return nil
	}

	if w.TaskLimit[taskType] == -1 {
		return nil
	}

	if 0 == w.TaskRun[taskType].RuningCnt {
		return nil
	}
	// mux := w.TaskRun[taskType].LmtLock
	w.TaskRun[taskType].LmtLock.Lock()
	defer w.TaskRun[taskType].LmtLock.Unlock()
	rif := w.TaskRun[taskType]
	rif.RuningCnt -= 1

	return nil
}

func (w *Worker) MotifyTaskLimit(taskType string, cnt int) error {
	if _, ok := w.TaskLimit[taskType]; ok {
		w.TaskLimit[taskType] = cnt
	} else {
		return xerrors.Errorf("taskType %s no found", taskType)
	}
	return nil
}

func (w *Worker) DisplayTaskTimeOut() map[string]int64 {
	return w.TaskTimeOut
}

func (w *Worker) MotifyTaskTimeOut(taskType string, cnt int64) error {
	if _, ok := w.TaskTimeOut[taskType]; ok {
		w.TaskTimeOut[taskType] = cnt
	} else {
		return xerrors.Errorf("taskType %s no found", taskType)
	}
	return nil
}

func (w *Worker) DisplayTaskLimit() map[string]int {
	return w.TaskLimit
}

func (w *Worker) DisplayWorkerListen() string {
	return w.ExtrListen
}

func (w *Worker) MotifyWorkerListen(l string) string {
	w.ExtrListen = l
	return l
}

func (w *Worker) DeferMinerRecieve(actorID int64, sectorNum int64, taskType string, session string, result util.TaskResult, processErr error) {

	var minerApiErr error
	close, minerApiErr := deferMinerRecieve(w.MinerApi, w.MinerUrl, actorID, sectorNum, taskType, session, result, processErr)
	if minerApiErr != nil {
		failedTask := FailedTask{
			ActorID:   actorID,
			SectorNum: sectorNum,
			TaskType:  taskType,
		}
		if processErr != nil {
			failedTask.IsErr = true
			failedTask.Result = []byte(processErr.Error())
		} else {
			failedTask.Result = result.CommBytes
		}
		w.FialedMap[session] = failedTask
	}
	if close != nil && minerApiErr == nil {
		w.MinerCloser = close
	}

	if minerApiErr == nil && taskType == util.C1 {
		err1 := delCache(taskType)
		if err1 != nil {
			log.Warnf("(w *Worker) DeferMinerRecieve delCache %s error %v", taskType, err1)
		}
	}
	log.Infof("DeferMinerRecieve actorID %d sectorNum: %d taskType %s session %s result %v minerApiErr %v processErr %v", actorID, sectorNum, taskType, session, result, minerApiErr, processErr)
}

func (w *Worker) RetryTask(actorID int64, sectorNum int64, taskType string) (string, error) {

	var err error
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

	session := uuid.New().String()

	reqInfo := util.RequestInfo{
		ActorID:      actorID,
		SectorNum:    sectorNum,
		TaskType:     taskType,
		WorkerID:     w.WorkerID,
		HostName:     w.HostName,
		WorkerListen: w.ExtrListen,
		Session:      session,
	}

	taskInfo, err := w.queryRetry(reqInfo)

	if err != nil {
		return "", err
	}

	switch taskType {
	case util.PC1:

		go w.sealPreCommitPhase1(taskInfo, session)
		return session, nil
	case util.PC2:

		go w.sealPreCommitPhase2(taskInfo, session)
		return session, nil
	case util.C1:

		go w.sealCommitPhase1(taskInfo, session)
		return session, nil
	case util.C2:

		go w.sealCommitPhase2(taskInfo, session)
		return session, nil
	}
	err = xerrors.Errorf("RetryTask actorID %d sectorNum %d taskType %s undefine error", actorID, sectorNum, taskType)
	return "", err
}

func (w *Worker) RetryTaskPID(actorID int64, sectorNum int64, taskType string, binPath string) (string, error) {

	var err error

	session := uuid.New().String()

	reqInfo := util.RequestInfo{
		ActorID:      actorID,
		SectorNum:    sectorNum,
		TaskType:     taskType,
		WorkerID:     w.WorkerID,
		HostName:     w.HostName,
		WorkerListen: w.ExtrListen,
		Session:      session,
	}

	taskInfo, err := w.queryRetry(reqInfo)

	if err != nil && !strings.Contains(err.Error(), "record not found") {
		log.Errorf("worker SealPreCommitPhase2 error %v", err)
		err2 := w.ConnMiner()
		if err2 != nil {
			return "MinerApi reconnect please retry", err2
		}
		log.Info("MinerApi reconnect please retry")
		return session, err
	}

	switch taskType {
	case util.PC1:

		go w.processPrePhase1(taskInfo, binPath, session)
		return session, nil
	case util.PC2:

		go w.processPrePhase2(taskInfo, binPath, session)
		return session, nil
	case util.C1:

		go w.processCommitPhase1(taskInfo, binPath, session)
		return session, nil
	case util.C2:

		go w.processCommitPhase2(taskInfo, binPath, session)
		return session, nil

	case util.PRECOMMIT:
		go w.updateTaskInfo(*taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.TaskType, util.INIT)

	case util.SEED:
		go w.updateTaskInfo(*taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.TaskType, util.INIT)

	case util.COMMIT:
		go w.updateTaskInfo(*taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.TaskType, util.INIT)
	}
	err = xerrors.Errorf("RetryTask actorID %d sectorNum %d taskType %s undefine error", actorID, sectorNum, taskType)
	return session, err
}

func (w *Worker) RetryTaskPIDByState(actorID int64, taskType string, binPath string) (string, error) {

	var err error
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
		ActorID:  actorID,
		TaskType: taskType,
		WorkerID: w.WorkerID,
	}

	taskInfo, err := w.queryRetryByState(reqInfo)
	if err != nil && !strings.Contains(err.Error(), "record not found") {
		log.Errorf("worker %s error %v", taskType, err)
		err2 := w.ConnMiner()
		if err2 != nil {
			return "", err2
		}
		log.Info("MinerApi reconnect please retry")
		return "", err
	}

	if err != nil && strings.Contains(err.Error(), "record not found") {
		return "", err
	}
	ret, err := w.RetryTaskPID(actorID, *taskInfo.SectorNum, taskType, binPath)
	return ret, err
}

func (w *Worker) ChildProcess(taskInfo util.DbTaskInfo, binPath, session string) (int, error) {

	var err error
	cmd, err := TaskProcess(binPath, w.MinerUrl, session, *taskInfo.SectorNum, *taskInfo.ActorID)

	if err != nil {
		return -1, err
	}

	c := time.After(30 * time.Second)
	{
	loop:
		for {
			select {
			case <-c:
				if cmd.Process == nil {
					err = xerrors.Errorf("wait child pid time out")
					break loop
				}
			default:
				log.Warnf("cmd.Process %v", cmd.Process)
				if cmd.Process != nil {
					break loop
				}
				time.Sleep(1 * time.Second)
			}
		}
	}
	if err != nil {
		return -1, err
	}

	timebeat := make(chan interface{}, 1)
	process := make(chan interface{}, 1)
	stopchnl := make(chan interface{}, 1)

	w.TaskCloseChnl[session] = stopchnl

	closeBeat := taskHeartBeat(timebeat, 30*time.Second)

	go func() {
		errReserver := cmd.Wait()
		if errReserver != nil {
			log.Warnf("acotID %d sectorNum %d pid %v cmd.Wait() error %v", *taskInfo.ActorID, *taskInfo.SectorNum, cmd.Process.Pid, errReserver)
		}
		process <- nil
	}()

	log.Infof("TaskProcess pid %v actorID %d sectorNum %d", cmd.Process.Pid, *taskInfo.ActorID, *taskInfo.SectorNum)
	go func() {
		w.handleProccess(timebeat, closeBeat, process, stopchnl, *taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.TaskType, &err, session, func() {
			cmd.Process.Kill()
		})
		if err != nil {
			w.DeferMinerRecieve(*taskInfo.ActorID, *taskInfo.SectorNum, taskInfo.TaskType, session, util.TaskResult{}, err)
		}
	}()

	return cmd.Process.Pid, nil
}

func (w *Worker) CheckSession(session string) bool {
	_, ok := w.TaskCloseChnl[session]
	return ok
}

func (w *Worker) CheckMiner() (string, error) {

	var err error
	defer func() {
		err1 := recover()
		if err1 != nil || err != nil {
			log.Errorf("worker CheckMiner panic %v error %v", err1, err)
			for i := 0; i < 5; i++ {
				err1 := w.ConnMiner()
				if err1 != nil {
					log.Errorf("worker CheckMiner reconnect miner error %v retry %d", err1, i)
				} else {
					break
				}
			}
		}
	}()

	if w.MinerApi.CheckServer() {
		return "check minerAPI successed", nil
	}
	err = xerrors.Errorf("check minerAPI failed")
	return "", err
}

func (w *Worker) ResetAbortedSession() ([]map[string]interface{}, error) {
	var state int64 = util.RUNING
	taskInfo := util.DbTaskInfo{
		WorkerID: w.WorkerID,
		HostName: w.HostName,
		State:    &state,
	}
	ret := make([]map[string]interface{}, 0, 1)
	taskInfos, err := w.queryOnly(taskInfo)
	if err != nil {
		return ret, err
	}
	for _, ti := range taskInfos {
		_, ok := w.TaskCloseChnl[ti.LastReqID]
		if !ok {
			_, err := os.Stat(filepath.Join(ti.CacheDirPath, "sc-02-data-tree-r-last-0.dat"))
			if ti.TaskType == util.PC2 && err == nil {
				cmd := exec.Command("/bin/bash", "-c", "rm -f "+ti.SealedSectorPath)
				cmd.Stderr = os.Stderr
				cmd.Stdout = os.Stdout
				err := cmd.Run()
				if err != nil {
					log.Errorf("delete actorID %d sectorNum %d sealedfile %s error %v", *ti.ActorID, *ti.SectorNum, ti.SealedSectorPath, err)
				}
				cmd = exec.Command("/bin/bash", "-c", "rm -f "+filepath.Join(ti.CacheDirPath, `sc-02-data-tree-[cr]*`))
				cmd.Stderr = os.Stderr
				cmd.Stdout = os.Stdout
				err = cmd.Run()
				if err != nil {
					log.Errorf("delete actorID %d sectorNum %d Cachefile %s/sc-02-data-tree-[cr]* error %v", *ti.ActorID, *ti.SectorNum, ti.CacheDirPath, err)
				}
				cmd = exec.Command("/bin/bash", "-c", "rm -f "+filepath.Join(ti.CacheDirPath, `c1Out`))
				cmd.Stderr = os.Stderr
				cmd.Stdout = os.Stdout
				err = cmd.Run()
				if err != nil {
					log.Errorf("delete actorID %d sectorNum %d Cachefile %s/c1Out error %v", *ti.ActorID, *ti.SectorNum, ti.CacheDirPath, err)
				}
				_, err = w.updateTaskInfo(*ti.ActorID, *ti.SectorNum, util.PC1, util.RETRY)
				if err != nil {
					log.Warnf("actorID %d sectorNum %d taskType %s ResetAbortedSession error %w", *ti.ActorID, *ti.SectorNum, ti.TaskType, err)
					continue
				}
			} else {
				_, err := w.updateTaskInfo(*ti.ActorID, *ti.SectorNum, ti.TaskType, util.RETRY)
				if err != nil {
					log.Warnf("actorID %d sectorNum %d taskType %s ResetAbortedSession error %w", *ti.ActorID, *ti.SectorNum, ti.TaskType, err)
					continue
				}
			}
			rm := map[string]interface{}{
				"ActorID":  *ti.ActorID,
				"SctorNum": *ti.SectorNum,
				"TaskType": ti.TaskType,
			}
			ret = append(ret, rm)
		}
	}
	return ret, nil
}
