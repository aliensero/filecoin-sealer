package worker

import (
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func (w *Worker) queryTask(reqInfo util.RequestInfo) (util.DbTaskInfo, error) {
	if w.MinerApi == nil {
		return util.DbTaskInfo{}, xerrors.Errorf("w.MinerApi is nil")
	}
	return w.MinerApi.QueryTask(reqInfo)
}

func (w *Worker) queryRetry(reqInfo util.RequestInfo) (util.DbTaskInfo, error) {
	if w.MinerApi == nil {
		return util.DbTaskInfo{}, xerrors.Errorf("w.MinerApi is nil")
	}
	return w.MinerApi.RetryTask(reqInfo)
}

func (w *Worker) queryRetryByState(reqInfo util.RequestInfo) (util.DbTaskInfo, error) {
	if w.MinerApi == nil {
		return util.DbTaskInfo{}, xerrors.Errorf("w.MinerApi is nil")
	}
	return w.MinerApi.QueryRetry(reqInfo)
}

func (w *Worker) queryOnly(taskInfo util.DbTaskInfo) ([]util.DbTaskInfo, error) {
	if w.MinerApi == nil {
		return []util.DbTaskInfo{}, xerrors.Errorf("w.MinerApi is nil")
	}
	return w.MinerApi.QueryOnly(taskInfo)
}

func (w *Worker) queryPoSt(reqInfo util.RequestInfo) ([]util.DbPostInfo, error) {
	if w.MinerApi == nil {
		return []util.DbPostInfo{}, xerrors.Errorf("websocket routine exiting")
	}
	return w.MinerApi.QueryToPoSt(reqInfo)
}

func (w *Worker) queryMinerPoStInfo(actorID int64) (util.MinerPoStInfo, error) {
	if w.MinerApi == nil {
		return util.MinerPoStInfo{}, xerrors.Errorf("websocket routine exiting")
	}
	return w.MinerApi.QueryMinerPoStInfo(actorID)
}
