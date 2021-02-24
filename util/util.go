package util

import (
	"net"
	"strings"
)

func LocalIp() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	var ip string
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
			}
		}
	}
	return ip, nil
}

func TaskFailedHandle(taskLog DbTaskLog) {
	log.Warnf("TaskFailedHandle actorID %d sectorNum %d taskType %s", taskLog.ActorID, taskLog.SectorNum, taskLog.TaskType)
	db := taskLog.Db
	var logs []DbTaskLog
	err := db.Where("actor_id = ? and sector_num = ? and state = ? and result = ?", taskLog.ActorID, taskLog.SectorNum, ERROR, taskLog.Result).Find(&logs).Error
	if err != nil {
		log.Errorf("TaskFailedHandle actorID %d sectorNum %d error %v", taskLog.ActorID, taskLog.SectorNum, err)
		return
	}
	var fails []DbTaskFailed
	db.Where("task_type = ?", taskLog.TaskType).Find(&fails)
	retry := false
	next := false
	var state int64 = RETRY
	for _, f := range fails {
		if strings.Contains(taskLog.Result, f.ErrOutline) {
			if f.ExecType == EXECRETRY {
				if int64(len(logs)) < f.RetryCnt {
					retry = true
					state = f.UpdateState
					break
				}
			} else if f.ExecType == EXECNEXT {
				next = true
				state = f.UpdateState
				break
			}
		}
	}
	if retry {
		db.Where("actor_id = ? and sector_num = ?", taskLog.ActorID, taskLog.SectorNum).Model(DbTaskInfo{}).Update(map[string]interface{}{"state": state})
	}
	if next {
		db.Where("actor_id = ? and sector_num = ?", taskLog.ActorID, taskLog.SectorNum).Model(DbTaskInfo{}).Update(map[string]interface{}{"state": state, "task_type": NextTask[taskLog.TaskType]})
		if NextTask[taskLog.TaskType] == PROVING {
			go UpdatePoStInfo(taskLog)
		}
	}
}

func RegisterPoSt(taskLog DbTaskLog) {
	actorID := taskLog.ActorID
	sectorNum := taskLog.SectorNum
	workerID := taskLog.WorkerID
	poStInfo := DbPostInfo{
		ActorID:   actorID,
		SectorNum: sectorNum,
		WorkerID:  workerID,
		State:     INIT,
	}
	db := taskLog.Db
	err := db.Create(&poStInfo).Error
	if err != nil {
		log.Errorf("RegisterPoSt PoStInfo actorID %d sectorNum %d workerID %s error %v", actorID, sectorNum, workerID, err)
	}
	log.Infof("RegisterPoSt PoStInfo actorID %d sectorNum %d workerID %s", actorID, sectorNum, workerID)
}

func UpdatePoStInfo(taskLog DbTaskLog) {
	actorID := taskLog.ActorID
	sectorNum := taskLog.SectorNum
	workerID := taskLog.WorkerID
	db := taskLog.Db

	var taskInfo DbTaskInfo
	err := db.Where("actor_id = ? and sector_num = ?", actorID, sectorNum).Select("comm_r,cache_dir_path,sealed_sector_path,proof_type").First(&taskInfo).Error
	if err != nil {
		log.Errorf("UpdatePoStInfo PoStInfo query DbTaskInfo actorID %d sectorNum %d workerID %s error %v", actorID, sectorNum, workerID, err)
		return
	}
	err = db.Where("actor_id = ? and sector_num = ? and state = 0", actorID, sectorNum).Model(&DbPostInfo{}).Update(map[string]interface{}{"state": SUCCESS, "proof_type": taskInfo.ProofType, "comm_r": taskInfo.CommR, "cache_dir_path": taskInfo.CacheDirPath, "sealed_sector_path": taskInfo.SealedSectorPath}).Error
	if err != nil {
		log.Errorf("UpdatePoStInfo PoStInfo actorID %d sectorNum %d workerID %s error %v", actorID, sectorNum, workerID, err)
	}
	log.Infof("UpdatePoStInfo PoStInfo actorID %d sectorNum %d workerID %s", actorID, sectorNum, workerID)
}
