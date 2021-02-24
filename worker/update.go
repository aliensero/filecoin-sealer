package worker

import (
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func (w *Worker) updateTaskInfo(actorID int64, sectorNum int64, taskType string, state int64) (util.DbTaskInfo, error) {
	if w.MinerApi == nil {
		return util.DbTaskInfo{}, xerrors.Errorf("w.MinerApi is nil")
	}
	return w.MinerApi.UpdateTask(actorID, sectorNum, taskType, state)
}
