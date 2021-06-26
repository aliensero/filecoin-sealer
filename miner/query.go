package miner

import (
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func (m *Miner) QueryOnly(taskInfo util.DbTaskInfo) (util.QueryTaskInfoResult, error) {
	var taskInfoRet []util.DbTaskInfo
	qr := util.QueryTaskInfoResult{}
	err := m.Db.Where(taskInfo).Find(&taskInfoRet).Error
	if err != nil {
		qr.ResultCode = util.Err
		qr.Err = err.Error()
		return qr, nil
	}
	qr.Results = taskInfoRet
	return qr, nil
}

func (m *Miner) QueryTask(reqInfo util.RequestInfo) (util.QueryTaskInfoResult, error) {

	actorID := reqInfo.ActorID
	taskType := reqInfo.TaskType
	workerID := reqInfo.WorkerID
	hostName := reqInfo.HostName
	reqID := reqInfo.Session
	listen := reqInfo.WorkerListen
	sectorNum := reqInfo.SectorNum
	session := reqInfo.Session

	switch taskType {
	case util.PC1:
		m.querypc1lock.Lock()
		defer m.querypc1lock.Unlock()
	case util.PC2:
		m.querypc2lock.Lock()
		defer m.querypc2lock.Unlock()
	case util.C1:
		m.queryc1lock.Lock()
		defer m.queryc1lock.Unlock()
	case util.C2:
		m.queryc2lock.Lock()
		defer m.queryc2lock.Unlock()
	case util.PRECOMMIT:
		m.queryprelock.Lock()
		defer m.queryprelock.Unlock()
	case util.SEED:
		m.queryseedlock.Lock()
		defer m.queryseedlock.Unlock()
	case util.COMMIT:
		m.querycommitlock.Lock()
		defer m.querycommitlock.Unlock()
	}

	taskInfo := util.DbTaskInfo{
		// ActorID:  actorID,
		// TaskType: taskType,
		// State:    0,
	}

	log.Infof("QueryTask reqInfo %v", reqInfo)

	qr := util.QueryTaskInfoResult{}
	tx := m.Db.Begin()
	if sectorNum == -1 {
		if taskType == util.PC2 || taskType == util.C1 {
			err := tx.Where("actor_id = ? and task_type = ? and state = ? and worker_id = ?", actorID, taskType, util.INIT, workerID).First(&taskInfo).Error
			if err != nil {
				tx.Rollback()
				qr.ResultCode = util.Err
				qr.Err = err.Error()
				return qr, nil
			}
		} else {
			err := tx.Where("actor_id = ? and task_type = ? and state = ?", actorID, taskType, util.INIT).First(&taskInfo).Error
			if err != nil {
				tx.Rollback()
				qr.ResultCode = util.Err
				qr.Err = err.Error()
				return qr, nil
			}
		}
	} else {
		err := tx.Where("actor_id = ? and sector_num = ?", actorID, sectorNum).First(&taskInfo).Error
		if err != nil {
			tx.Rollback()
			qr.ResultCode = util.Err
			qr.Err = err.Error()
			return qr, nil
		}
	}

	workerID1 := taskInfo.WorkerID
	if workerID != "" {
		workerID1 = workerID
	}

	if err := tx.Where(taskInfo).Model(taskInfo).Updates(map[string]interface{}{"state": util.RUNING, "worker_id": workerID1, "host_name": hostName, "worker_listen": listen, "last_req_id": session}).Error; err != nil {
		tx.Rollback()
		qr.ResultCode = util.Err
		qr.Err = err.Error()
		return qr, nil
	}

	taskLog := util.DbTaskLog{
		ActorID:      actorID,
		SectorNum:    *taskInfo.SectorNum,
		TaskType:     taskType,
		WorkerID:     workerID,
		HostName:     hostName,
		WorkerListen: listen,
		ReqID:        reqID,
		State:        util.RUNING,
	}
	if err := tx.Create(&taskLog).Error; err != nil {
		tx.Rollback()
		qr.ResultCode = util.Err
		qr.Err = err.Error()
		return qr, nil
	}
	tx.Commit()
	qr.Results = []util.DbTaskInfo{taskInfo}
	return qr, nil
}

func (m *Miner) QueryRetry(reqInfo util.RequestInfo) (util.QueryTaskInfoResult, error) {
	actorID := reqInfo.ActorID
	taskType := reqInfo.TaskType
	workerID := reqInfo.WorkerID

	switch taskType {
	case util.PC1:
		m.requerypc1lock.Lock()
		defer m.requerypc1lock.Unlock()
	case util.PC2:
		m.requerypc2lock.Lock()
		defer m.requerypc2lock.Unlock()
	case util.C1:
		m.requeryc1lock.Lock()
		defer m.requeryc1lock.Unlock()
	case util.C2:
		m.requeryc2lock.Lock()
		defer m.requeryc2lock.Unlock()
	case util.PRECOMMIT:
		m.requeryprelock.Lock()
		defer m.requeryprelock.Unlock()
	case util.SEED:
		m.requeryseedlock.Lock()
		defer m.requeryseedlock.Unlock()
	case util.COMMIT:
		m.requerycommitlock.Lock()
		defer m.requerycommitlock.Unlock()
	}
	qr := util.QueryTaskInfoResult{}
	var taskInfo util.DbTaskInfo
	err := m.Db.Where("actor_id = ? and task_type = ? and worker_id = ? and state = ?", actorID, taskType, workerID, util.RETRY).Select("actor_id,sector_num,task_type").First(&taskInfo).Error
	if err != nil {
		qr.ResultCode = util.Err
		qr.Err = err.Error()
		return qr, nil
	}
	err = m.Db.Where("actor_id = ? and task_type = ? and worker_id = ? and state = ? and sector_num = ?", actorID, taskType, workerID, util.RETRY, *taskInfo.SectorNum).Model(taskInfo).Update("state", util.RUNING).Error
	if err != nil {
		log.Errorf("Miner query actorID % sectorNum %d taskType %s retry error %v", actorID, *taskInfo.SectorNum, taskType, err)
		err = xerrors.Errorf("record not found")
		qr.ResultCode = util.Err
		qr.Err = err.Error()
		return qr, nil
	}
	qr.Results = []util.DbTaskInfo{taskInfo}
	return qr, nil
}

func (m *Miner) RetryTask(reqInfo util.RequestInfo) (util.QueryTaskInfoResult, error) {

	actorID := reqInfo.ActorID
	sectorNum := reqInfo.SectorNum
	taskType := reqInfo.TaskType
	reqID := reqInfo.Session
	workerListen := reqInfo.WorkerListen
	workerID := reqInfo.WorkerID
	hostName := reqInfo.HostName

	var state int64 = util.RUNING
	taskInfo := util.DbTaskInfo{
		TaskType:     taskType,
		State:        &state,
		WorkerListen: workerListen,
		WorkerID:     workerID,
		HostName:     hostName,
		LastReqID:    reqID,
	}

	qr := util.QueryTaskInfoResult{}
	tx := m.Db.Begin()

	if taskType == util.PC1 || taskType == util.PC2 || taskType == util.C1 {
		if err := tx.Where("actor_id = ? and sector_num = ? and worker_id = ?", actorID, sectorNum, workerID).Model(taskInfo).Updates(taskInfo).First(&taskInfo).Error; err != nil {
			tx.Rollback()
			qr.ResultCode = util.Err
			qr.Err = err.Error()
			return qr, nil
		}
	} else {
		if err := tx.Where("actor_id = ? and sector_num = ?", actorID, sectorNum).Model(taskInfo).Updates(taskInfo).First(&taskInfo).Error; err != nil {
			tx.Rollback()
			qr.ResultCode = util.Err
			qr.Err = err.Error()
			return qr, nil
		}
	}
	taskLog := util.DbTaskLog{
		ActorID:      actorID,
		SectorNum:    *taskInfo.SectorNum,
		TaskType:     taskType,
		WorkerID:     taskInfo.WorkerID,
		WorkerListen: workerListen,
		HostName:     hostName,
		ReqID:        reqID,
		State:        util.RUNING,
	}
	if err := tx.Create(&taskLog).Error; err != nil {
		tx.Rollback()
		qr.ResultCode = util.Err
		qr.Err = err.Error()
		return qr, nil
	}
	tx.Commit()
	qr.ResultCode = util.Ok
	qr.Results = []util.DbTaskInfo{taskInfo}
	return qr, nil
}

func (m *Miner) QueryToPoSt(reqInfo util.RequestInfo) (util.QueryPostInfoResult, error) {
	actorID := reqInfo.ActorID
	sectors := reqInfo.Sectors
	var postInfo []util.DbPostInfo
	err := m.Db.Where("actor_id = ? and state = ? and sector_num in ("+sectors+")", actorID, util.SUCCESS).Find(&postInfo).Error
	qr := util.QueryPostInfoResult{}
	if err != nil {
		qr.ResultCode = util.Err
		qr.Err = err.Error()
		return qr, nil
	}
	qr.Results = postInfo
	return qr, err
}
