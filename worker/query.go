package worker

import (
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func (w *Worker) queryTask(reqInfo util.RequestInfo) (util.DbTaskInfo, error) {
	if w.MinerApi == nil {
		w.ConnMiner()
		return util.DbTaskInfo{}, xerrors.Errorf("w.MinerApi is nil")
	}
	qr, err := w.MinerApi.QueryTask(reqInfo)
	if err != nil {
		w.ConnMiner()
		return util.DbTaskInfo{}, err
	}

	if qr.ResultCode == util.Err {
		return util.DbTaskInfo{}, xerrors.Errorf(qr.Err)
	}
	return qr.Results[0], nil
}

// func (w *Worker) queryRetry(reqInfo util.RequestInfo) (util.DbTaskInfo, error) {
// 	if w.MinerApi == nil {
// 		return util.DbTaskInfo{}, xerrors.Errorf("w.MinerApi is nil")
// 	}
// 	qr, err := w.MinerApi.QueryTask(reqInfo)
// 	if err != nil {
// 		return util.DbTaskInfo{}, err
// 	}

// 	if qr.ResultCode == util.Err {
// 		return util.DbTaskInfo{}, xerrors.Errorf(qr.Err)
// 	}

// 	return qr.Results[0], nil
// }

// func (w *Worker) queryRetryByState(reqInfo util.RequestInfo) (util.DbTaskInfo, error) {
// 	if w.MinerApi == nil {
// 		return util.DbTaskInfo{}, xerrors.Errorf("w.MinerApi is nil")
// 	}
// 	qr, err := w.MinerApi.QueryTask(reqInfo)
// 	if err != nil {
// 		return util.DbTaskInfo{}, err
// 	}

// 	if qr.ResultCode == util.Err {
// 		return util.DbTaskInfo{}, xerrors.Errorf(qr.Err)
// 	}

// 	return qr.Results[0], nil
// }

func (w *Worker) queryOnly(taskInfo util.DbTaskInfo) ([]util.DbTaskInfo, error) {
	if w.MinerApi == nil {
		return nil, xerrors.Errorf("w.MinerApi is nil")
	}
	qr, err := w.MinerApi.QueryOnly(taskInfo)
	if err != nil {
		return nil, err
	}

	if qr.ResultCode == util.Err {
		return nil, xerrors.Errorf(qr.Err)
	}
	return qr.Results, nil
}

func (w *Worker) queryPoSt(reqInfo util.RequestInfo) ([]util.DbPostInfo, error) {
	if w.MinerApi == nil {
		return nil, xerrors.Errorf("websocket routine exiting")
	}
	qr, err := w.MinerApi.QueryToPoSt(reqInfo)
	if err != nil {
		return nil, err
	}

	if qr.ResultCode == util.Err {
		return nil, xerrors.Errorf(qr.Err)
	}

	return qr.Results, nil
}

func (w *Worker) queryMinerPoStInfo(actorID int64) (util.MinerPoStInfo, error) {
	if w.MinerApi == nil {
		return util.MinerPoStInfo{}, xerrors.Errorf("websocket routine exiting")
	}
	return w.MinerApi.QueryMinerPoStInfo(actorID)
}
