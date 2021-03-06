package miner

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/jinzhu/gorm"
	"gitlab.ns/lotus-worker/util"
)

func init() {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("miner-struct recover error %v", err)
		}
	}()
	_ = logging.SetLogLevel("miner-struct", "DEBUG")
}

var log = logging.Logger("miner-struct")

type Miner struct {
	Db *gorm.DB
	// LotusApi          *util.LotusAPI
	LotusApi          util.LotusAPI
	LotusUrl          string
	LotusToken        string
	WaitSeedEpoch     int64
	SectorExpiration  int64
	SyncPoStMap       *SyncPoStMap
	ReqSession        sync.Map
	querypc1lock      sync.Mutex
	querypc2lock      sync.Mutex
	queryc1lock       sync.Mutex
	queryc2lock       sync.Mutex
	queryprelock      sync.Mutex
	queryseedlock     sync.Mutex
	querycommitlock   sync.Mutex
	requerypc1lock    sync.Mutex
	requerypc2lock    sync.Mutex
	requeryc1lock     sync.Mutex
	requeryc2lock     sync.Mutex
	requeryprelock    sync.Mutex
	requeryseedlock   sync.Mutex
	requerycommitlock sync.Mutex

	PoStMiner map[int64]util.ActorPoStInfo
	ActorTask map[int64]string
}

// type ActorNonceLock struct {
// 	mut   sync.Mutex
// 	nonce uint64
// }

// func (anl *ActorNonceLock) set(nonce uint64) {
// 	anl.mut.Lock()
// 	defer anl.mut.Unlock()
// 	anl.nonce = nonce
// }

// func (anl *ActorNonceLock) increment() {
// 	anl.mut.Lock()
// 	defer anl.mut.Unlock()
// 	anl.nonce += 1
// }

// func (anl *ActorNonceLock) decrement() {
// 	anl.mut.Lock()
// 	defer anl.mut.Unlock()
// 	if anl.nonce > 0 {
// 		anl.nonce -= 1
// 	}
// }

func (m *Miner) CheckServer() bool {
	return true
}

func (m *Miner) AddTask(actorID int64, sectorNum int64, taskType string, minerPaht string, unsealedPath string, sealerProof string, proofType int, pieceCIDstr string) (int64, error) {

	f := "t0"
	// if util.NsCurrentNetwork == util.NsMainNet {
	// 	f = "f0"
	// }
	cachePath := fmt.Sprintf("%s/%s%d/cache/s-%s%d-%d", minerPaht, f, actorID, f, actorID, sectorNum)
	SealedPath := fmt.Sprintf("%s/%s%d/sealed/s-%s%d-%d", minerPaht, f, actorID, f, actorID, sectorNum)

	taskInfo := util.DbTaskInfo{
		ActorID:          &actorID,
		SectorNum:        &sectorNum,
		TaskType:         taskType,
		CacheDirPath:     cachePath,
		SealedSectorPath: SealedPath,
		StagedSectorPath: unsealedPath,
	}
	if pieceCIDstr != "" {
		taskInfo.PieceStr = pieceCIDstr
	}
	if sealerProof != "" {
		taskInfo.SealerProof = sealerProof
	}

	if proofType != -1 {
		taskInfo.ProofType = int64(proofType)
	}
	if err := m.Db.Create(&taskInfo).Error; err != nil {
		return 0, err
	}
	return *taskInfo.SectorNum, nil
}

func (m *Miner) UpdateTask(actorID int64, sectorNum int64, taskType string, state int64) (util.DbTaskInfo, error) {

	var taskInfo util.DbTaskInfo

	if err := m.Db.Where("actor_id = ? and sector_num = ?", actorID, sectorNum).Model(taskInfo).Updates(map[string]interface{}{
		"actor_id":   actorID,
		"sector_num": sectorNum,
		"task_type":  taskType,
		"state":      state,
	}).First(&taskInfo).Error; err != nil {
		return taskInfo, err
	}

	return taskInfo, nil
}

func (m *Miner) UpdateTaskLog(reqInfo util.RequestInfo, result string) (string, error) {

	actorID := reqInfo.ActorID
	sectorNum := reqInfo.SectorNum
	taskType := reqInfo.TaskType
	session := reqInfo.Session

	if err := m.Db.Where("actor_id = ? and sector_num = ? and task_type = ? and req_id = ?", actorID, sectorNum, taskType, session).Model(util.DbTaskLog{}).Updates(map[string]interface{}{"result": result}).Error; err != nil {
		return result, err
	}

	return result, nil
}

func (m *Miner) GetTicket(actorID int64, sectorNum int64) ([]byte, error) {
	minerAddr, err := util.NsNewIDAddress(uint64(actorID))
	if err != nil {
		return []byte{}, err
	}

	tipset, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		log.Errorf("GetTicket: api ChainHead error %v", err)
		return []byte{}, err
	}
	epoch := tipset.Height()
	tok := tipset.Key()

	ticketEpoch := epoch - util.NsSealRandomnessLookback
	buf := new(bytes.Buffer)
	if err := minerAddr.MarshalCBOR(buf); err != nil {
		return []byte{}, err
	}

	rand, err := m.LotusApi.ChainGetRandomnessFromTickets(context.TODO(), tok, util.NsDomainSeparationTag_SealRandomness, ticketEpoch, buf.Bytes())
	if err != nil {
		return []byte{}, err
	}

	if err := m.Db.Where("actor_id = ? and sector_num = ?", actorID, sectorNum).Model(&util.DbTaskInfo{}).Updates(map[string]interface{}{"ticket_hex": hex.EncodeToString(util.NsSealRandomness(rand)), "ticket_epoch": int64(ticketEpoch)}).Error; err != nil {
		return []byte{}, err
	}

	return util.NsSealRandomness(rand), nil
}

func (m *Miner) GetSeedRand(actorID int64, sectorNum int64) (string, error) {

	session := uuid.New().String()
	taskType := util.SEED
	reqInfo := util.RequestInfo{
		ActorID:   actorID,
		SectorNum: sectorNum,
		Session:   session,
		TaskType:  taskType,
	}

	qr, err := m.QueryTask(reqInfo)
	if err != nil {
		return session, err
	}
	if qr.ResultCode == util.Err {
		return qr.ToString(), nil
	}
	taskInfo := qr.Results[0]

	go func() {

		var err error
		var ret util.TaskResult
		defer func() {
			if err != nil {
				err = m.RecieveTaskResult(actorID, *taskInfo.SectorNum, taskInfo.TaskType, session, true, util.TaskResult{CommString: err.Error()})
				if err != nil {
					log.Warn("GetSeed m.RecieveTaskResult error %v", err)
				}
			} else {
				err = m.RecieveTaskResult(actorID, *taskInfo.SectorNum, taskInfo.TaskType, session, false, ret)
				if err != nil {
					log.Warn("GetSeed m.RecieveTaskResult error %v", err)
				}
			}
			m.ReqSession.Delete(session)
		}()

		tipset, err := m.LotusApi.ChainHead(context.TODO())
		if err != nil {
			return
		}

		minerAddr, err := util.NsNewIDAddress(uint64(actorID))
		if err != nil {
			return
		}

		buf := new(bytes.Buffer)
		if err := minerAddr.MarshalCBOR(buf); err != nil {
			return
		}

		pci, err := m.LotusApi.StateSectorPreCommitInfo(context.TODO(), minerAddr, util.NsSectorNum(*taskInfo.SectorNum), tipset.Key())
		if err != nil {
			return
		}
		// if &pci == nil {
		// 	for {
		// 		pci, err = m.LotusApi.StateSectorPreCommitInfo(context.TODO(), minerAddr, util.NsSectorNum(*taskInfo.SectorNum), tipset.Key())
		// 		if err != nil {
		// 			return
		// 		}
		// 		if &pci == nil {
		// 			time.Sleep(30 * time.Second)
		// 			continue
		// 		}
		// 		break
		// 	}
		// }

		// randHeight := pci.PreCommitEpoch + policy.GetPreCommitChallengeDelay()
		randHeight := pci.PreCommitEpoch + util.NsChainEpoch(m.WaitSeedEpoch)
		m.ReqSession.Store(session, nil)
		for randHeight > tipset.Height() {
			log.Debugf("actorID %v sectorNum %d randHeight %d grantThan current tipset height %d", minerAddr, *taskInfo.SectorNum, randHeight, tipset.Height())
			time.Sleep(6 * 30 * time.Second)
			tipset, err = m.LotusApi.ChainHead(context.TODO())
			if err != nil {
				return
			}
		}

		rand, err := m.LotusApi.ChainGetRandomnessFromBeacon(context.TODO(), tipset.Key(), util.NsDomainSeparationTag_InteractiveSealChallengeSeed, randHeight, buf.Bytes())
		if err != nil {
			return
		}
		ret.SeedEpoch = int64(tipset.Height())
		ret.SeedHex = hex.EncodeToString(rand)
	}()
	return session, nil

}

func (m *Miner) MotifyWaitSeedEpoch(epoch int64) int64 {
	m.WaitSeedEpoch = epoch
	return epoch
}

func (m *Miner) CheckSession(session string) bool {
	_, ok := m.ReqSession.Load(session)
	return ok
}

func (m *Miner) CheckWorkerSession(workerListen, session string) bool {

	workerUrl := "ws://" + workerListen + "/rpc/v0"
	close, workerApi, err := util.ConnectWorker(workerUrl)
	if err != nil {
		log.Errorf("CheckWorkerSession session %s error %v", session, err)
		return true
	}
	defer close()
	return workerApi.CheckSession(session)
}

func (m *Miner) RecieveTaskResult(actorID int64, sectorNum int64, taskType string, reqID string, isErr bool, result util.TaskResult) error {
	var state int64 = util.SUCCESS
	if isErr {
		state = util.ERROR
	}
	log.Infof("ActorID %d sectorNum %d taskType %s reqID %s isErr %v recieveTaskResult %v", actorID, sectorNum, taskType, reqID, isErr, result.CommString)
	taskLogWhr := util.DbTaskLog{
		ActorID:   actorID,
		SectorNum: sectorNum,
		TaskType:  taskType,
		ReqID:     reqID,
	}
	tx := m.Db.Begin()
	var state1 int64 = util.RUNING
	taskInfo := util.DbTaskInfo{
		ActorID:   &actorID,
		SectorNum: &sectorNum,
		TaskType:  taskType,
		LastReqID: reqID,
		State:     &state1,
	}
	if isErr {
		if err := tx.Where(taskInfo).Model(taskInfo).Updates(map[string]interface{}{"state": state, "last_req_id": reqID}).Error; err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Where(taskLogWhr).Model(&taskLogWhr).Updates(map[string]interface{}{"state": state, "result": result.Marshal()}).Error; err != nil {
			tx.Rollback()
			return err
		}
		go m.TaskFailedHandle(actorID, sectorNum, taskType, result.Marshal())
	} else {
		noCommBytes := result
		noCommBytes.CommBytes = nil
		if err := tx.Where(taskLogWhr).FirstOrInit(&taskLogWhr).Model(&taskLogWhr).Updates(map[string]interface{}{"state": state, "result": noCommBytes.Marshal()}).Error; err != nil {
			tx.Rollback()
			return err
		}

		if taskType == util.PC1 {
			if err := tx.Where(taskInfo).Model(taskInfo).Updates(map[string]interface{}{"task_type": util.NextTask[taskType], "state": util.INIT, "last_req_id": reqID, "phase1_output": result.CommBytes}).Error; err != nil {
				tx.Rollback()
				return err
			}

		} else if taskType == util.PC2 {
			sealedCID := result.SealedCID
			unsealedCID := result.UnsealedCID
			if err := tx.Where(taskInfo).Model(taskInfo).Updates(map[string]interface{}{"task_type": util.NextTask[taskType], "state": util.INIT, "last_req_id": reqID, "comm_d": unsealedCID, "comm_r": sealedCID}).Error; err != nil {
				tx.Rollback()
				return err
			}

		} else if taskType == util.PRECOMMIT {
			var queryTaskInfo util.DbTaskInfo
			if err := tx.Where(taskInfo).First(&queryTaskInfo).Model(queryTaskInfo).Updates(map[string]interface{}{"task_type": util.NextTask[taskType], "state": util.INIT, "last_req_id": reqID}).Error; err != nil {
				tx.Rollback()
				return err
			}
			go m.RegisterPoSt(actorID, sectorNum, queryTaskInfo.WorkerID)

		} else if taskType == util.SEED {

			seedEpoch := result.SeedEpoch
			seedHex := result.SeedHex

			if err := tx.Where(taskInfo).Model(taskInfo).Updates(map[string]interface{}{"task_type": util.NextTask[taskType], "state": util.INIT, "last_req_id": reqID, "seed_epoch": seedEpoch, "seed_hex": seedHex}).Error; err != nil {
				tx.Rollback()
				return err
			}

		} else if taskType == util.C1 {
			if err := tx.Where(taskInfo).Model(taskInfo).Updates(map[string]interface{}{"task_type": util.NextTask[taskType], "state": util.INIT, "last_req_id": reqID, "c1_out": result.CommBytes}).Error; err != nil {
				tx.Rollback()
				return err
			}
		} else if taskType == util.C2 {
			if err := tx.Where(taskInfo).Model(taskInfo).Updates(map[string]interface{}{"task_type": util.NextTask[taskType], "state": util.INIT, "last_req_id": reqID, "proof": result.CommBytes}).Error; err != nil {
				tx.Rollback()
				return err
			}
		} else if taskType == util.COMMIT {
			if err := tx.Where(taskInfo).Model(taskInfo).Updates(map[string]interface{}{"task_type": util.NextTask[taskType], "state": util.INIT, "last_req_id": reqID, "deadline_inx": result.DealineInx, "partition_inx": result.PartitionInx, "c1_out": nil}).Error; err != nil {
				tx.Rollback()
				return err
			}
		} else {
			if err := tx.Where(taskInfo).Model(taskInfo).Updates(map[string]interface{}{"task_type": util.NextTask[taskType], "state": util.INIT, "last_req_id": reqID}).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	if isErr {
		go m.TaskFailedHandle(actorID, sectorNum, taskType, result.Marshal())
	}
	if taskType == util.COMMIT {
		go m.UpdatePoStInfo(actorID, sectorNum)
	}
	return nil
}

func (m *Miner) QueryLastResult(actorID int64, sectorNum int64, lastReqID string) (util.DbTaskLog, error) {
	taskRuningOrm := util.DbTaskLog{
		ActorID:   actorID,
		SectorNum: sectorNum,
		State:     util.SUCCESS,
		ReqID:     lastReqID,
	}
	if err := m.Db.Where(taskRuningOrm).First(&taskRuningOrm).Error; err != nil {
		return util.DbTaskLog{}, err
	}
	return taskRuningOrm, nil
}

func (m *Miner) ClearProving() error {

	var taskInfos []util.DbTaskInfo
	if err := m.Db.Where("state=0 and task_type = ?", "PROVING").Find(&taskInfos).Error; err != nil {
		return err
	}
	for _, t := range taskInfos {
		err := os.RemoveAll(t.SealedSectorPath)
		if err != nil {
			log.Warnf("Miner ClearProving actorID %d sectorNum %s error %v", t.ActorID, t.SectorNum, err)
		}
		err = os.RemoveAll(t.CacheDirPath)
		if err != nil {
			log.Warnf("Miner ClearProving actorID %d sectorNum %s error %v", t.ActorID, t.SectorNum, err)
		}
		if err := m.Db.Where(t).Model(t).Update("state", 1).Error; err != nil {
			log.Warnf("Miner ClearProving actorID %d sectorNum %s update state error %v", t.ActorID, t.SectorNum, err)
		}
	}

	return nil
}

func (m *Miner) WorkerLogin(workerID string, hostName string, listen string) (string, error) {

	info := util.DbWorkerLogin{
		WorkerID: workerID,
		HostName: hostName,
		Listen:   listen,
	}
	err := m.Db.Create(&info).Error
	return info.WorkerID, err
}

type taskInfoJ struct {
	ActorID   int64
	SectorNum int64
	TaskType  string
	ReqID     string
}

func (m *Miner) ResetAbortedSession(actorID int64) ([]taskInfoJ, error) {

	var tijs []taskInfoJ
	err := m.Db.Table("db_task_infos").Select("actor_id,sector_num,task_type,last_req_id").Where("task_type in (?,?,?) and state = ? and actor_id = ?", util.PRECOMMIT, util.SEED, util.COMMIT, util.RUNING, actorID).Find(&tijs).Error
	if err != nil {
		return tijs, err
	}
	ret := make([]taskInfoJ, 0, len(tijs))
	for _, tj := range tijs {
		_, ok := m.ReqSession.Load(tj.ReqID)
		if !ok {
			_, err := m.UpdateTask(tj.ActorID, tj.SectorNum, tj.TaskType, util.INIT)
			if err != nil {
				log.Warnf("actorID % sectorNum %d taskType %s ResetAbortedSession error %v", tj.ActorID, tj.SectorNum, tj.TaskType, err)
				continue
			}
			ttj := tj
			ret = append(ret, ttj)
		}
	}
	return ret, nil

}
