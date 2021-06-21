package miner

import (
	"strings"

	"github.com/jinzhu/gorm"
	"gitlab.ns/lotus-worker/util"
)

func (m *Miner) TaskFailedHandle(actorID int64, sectorNum int64, taskType string, result string) {
	log.Warnf("TaskFailedHandle actorID %d sectorNum %d taskType %s", actorID, sectorNum, taskType)
	var logs []util.DbTaskLog
	db := m.Db
	err := db.Where("actor_id = ? and sector_num = ? and state = ? and result = ?", actorID, sectorNum, util.ERROR, result).Find(&logs).Error
	if err != nil {
		log.Errorf("TaskFailedHandle actorID %d sectorNum %d error %v", actorID, sectorNum, err)
		return
	}
	var fails []util.DbTaskFailed
	db.Where("task_type = ?", taskType).Find(&fails)
	retry := false
	next := false
	var state int64 = util.RETRY
	for _, f := range fails {
		if strings.Contains(result, f.ErrOutline) {
			if f.ExecType == util.EXECRETRY {
				if int64(len(logs)) < f.RetryCnt {
					retry = true
					state = f.UpdateState
					break
				}
			} else if f.ExecType == util.EXECNEXT {
				next = true
				state = f.UpdateState
				break
			}
		}
	}
	if retry {
		db.Where("actor_id = ? and sector_num = ?", actorID, sectorNum).Model(util.DbTaskInfo{}).Update(map[string]interface{}{"state": state})
	}
	if next {
		db.Where("actor_id = ? and sector_num = ?", actorID, sectorNum).Model(util.DbTaskInfo{}).Update(map[string]interface{}{"state": state, "task_type": util.NextTask[taskType]})
	}
}

func (m *Miner) RegisterPoSt(actorID int64, sectorNum int64, workerID string) {
	poStInfo := util.DbPostInfo{
		ActorID:   actorID,
		SectorNum: sectorNum,
		WorkerID:  workerID,
		State:     util.INIT,
	}
	db := m.Db
	err := db.Create(&poStInfo).Error
	if err != nil {
		log.Errorf("RegisterPoSt PoStInfo actorID %d sectorNum %d workerID %s error %v", actorID, sectorNum, workerID, err)
	}
	log.Infof("RegisterPoSt PoStInfo actorID %d sectorNum %d workerID %s", actorID, sectorNum, workerID)
}

func (m *Miner) UpdatePoStInfo(actorID int64, sectorNum int64) {
	db := m.Db
	var taskInfo util.DbTaskInfo
	db2 := db.Where("actor_id = ? and sector_num = ?", actorID, sectorNum)
	err := db.Where("actor_id = ? and sector_num = ?", actorID, sectorNum).Select("comm_r,cache_dir_path,sealed_sector_path,proof_type").First(&taskInfo).Error
	if err != nil {
		log.Errorf("UpdatePoStInfo PoStInfo query DbTaskInfo actorID %d sectorNum %d workerID %s error %v", actorID, sectorNum, err)
		return
	}
	workerID := taskInfo.WorkerID
	var posti util.DbPostInfo
	err = db.Where("actor_id = ? and sector_num = ?", actorID, sectorNum).First(&posti).Error
	if gorm.ErrRecordNotFound == err {
		posti.ActorID = actorID
		posti.SectorNum = sectorNum
		posti.WorkerID = workerID
		posti.CommR = taskInfo.CommR
		posti.ProofType = taskInfo.ProofType
		posti.CacheDirPath = taskInfo.CacheDirPath
		posti.SealedSectorPath = taskInfo.SealedSectorPath
		posti.DeadlineInx = taskInfo.DeadlineInx
		posti.PartitionInx = taskInfo.PartitionInx
		posti.State = util.SUCCESS
		err = db.Create(&posti).Error
		log.Infof("UpdatePoStInfo PoStInfo create actorID %d sectorNum %d workerID %s error %v", actorID, sectorNum, workerID, err)
	} else {
		err = db.Where("actor_id = ? and sector_num = ?", actorID, sectorNum).Model(&util.DbPostInfo{}).Update(map[string]interface{}{"state": util.SUCCESS, "proof_type": taskInfo.ProofType, "comm_r": taskInfo.CommR, "cache_dir_path": taskInfo.CacheDirPath, "sealed_sector_path": taskInfo.SealedSectorPath, "deadline_inx": taskInfo.DeadlineInx, "partition_inx": taskInfo.PartitionInx}).Error
		log.Infof("UpdatePoStInfo PoStInfo actorID %d sectorNum %d workerID %s error %v", actorID, sectorNum, workerID, err)
	}
	var ti util.DbTaskInfo
	err = db2.First(&ti).Model(ti).Update("c1_out", nil).Error
	if err != nil {
		log.Errorf("update taskInfo c1_out to nil error %v", err)
	}
}
