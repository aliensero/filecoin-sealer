package miner

import (
	"bytes"
	"context"
	"strings"

	"github.com/google/uuid"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func (m *Miner) SendPreCommit(actorID int64, sectorNum int64, deposit string, fee string) (string, error) {

	session := uuid.New().String()
	taskType := util.PRECOMMIT
	reqInfo := util.RequestInfo{
		ActorID:   actorID,
		SectorNum: sectorNum,
		Session:   session,
		TaskType:  taskType,
	}

	taskInfo, err := m.QueryTask(reqInfo)

	if err != nil && !strings.Contains(err.Error(), "record not found") {
		return session, err
	}
	if err != nil && strings.Contains(err.Error(), "record not found") {
		log.Warnf("miner SendPreCommit record not found")
		return "", xerrors.Errorf("miner SendPreCommit %v", err)
	}

	m.ReqSession.Store(session, nil)
	defer func() {
		if err != nil {
			err := m.RecieveTaskResult(actorID, taskInfo.SectorNum, taskType, session, true, []byte(err.Error()))
			if err != nil {
				log.Errorf("SendPreCommit actorID %d sectorNum %d error %v", actorID, taskInfo.SectorNum, err)
			}
			m.ReqSession.Delete(session)
		}
	}()

	tipset, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		return session, err
	}

	minerAddr, err := util.NsNewIDAddress(uint64(actorID))
	if err != nil {
		return session, err
	}

	mi, err := m.LotusApi.StateMinerInfo(context.TODO(), minerAddr, tipset.Key())
	if err != nil {
		return session, err
	}

	expiration := taskInfo.TicketEpoch + m.SectorExpiration

	fromStr := mi.Worker.String()

	sealedCID, err := util.Parse(taskInfo.CommR)
	if err != nil {
		return session, err
	}

	params := &util.NsSectorPreCommitInfo{
		Expiration:   util.NsChainEpoch(expiration),
		SectorNumber: util.NsSectorNum(taskInfo.SectorNum),
		SealProof:    util.NsRegisteredSealProof(taskInfo.ProofType),

		SealedCID:     sealedCID,
		SealRandEpoch: util.NsChainEpoch(taskInfo.TicketEpoch),
	}

	enc := new(bytes.Buffer)
	if err := params.MarshalCBOR(enc); err != nil {
		return session, err
	}
	msgCID, err := m.SendMsg(fromStr, minerAddr.String(), int64(util.NsMethods.PreCommitSector), deposit, fee, enc.Bytes(), func(isErr bool, result string) {
		err := m.RecieveTaskResult(actorID, taskInfo.SectorNum, taskType, session, isErr, []byte(result))
		m.ReqSession.Delete(session)
		log.Warnf("SendPreCommit actorID %d sectorNum %d result %s error %v", actorID, taskInfo.SectorNum, result, err)
	})
	reqInfo = util.RequestInfo{
		ActorID:   taskInfo.ActorID,
		SectorNum: taskInfo.SectorNum,
		TaskType:  taskInfo.TaskType,
		Session:   session,
	}
	_, errReserve := m.UpdateTaskLog(reqInfo, msgCID)
	if errReserve != nil {
		log.Warnf("SendPreCommit actorID %d sectorNum %d msgCID %s update tasklog error %v", actorID, taskInfo.SectorNum, msgCID, errReserve)
	}
	return msgCID, err
}

func (m *Miner) SendCommit(actorID int64, sectorNum int64, deposit string, fee string) (string, error) {

	session := uuid.New().String()
	taskType := util.COMMIT
	reqInfo := util.RequestInfo{
		ActorID:   actorID,
		SectorNum: sectorNum,
		Session:   session,
		TaskType:  taskType,
	}

	var err error
	taskInfo, err := m.QueryTask(reqInfo)
	if err != nil && !strings.Contains(err.Error(), "record not found") {
		return session, err
	}
	if err != nil && strings.Contains(err.Error(), "record not found") {
		log.Warnf("miner SendCommit record not found")
		return "", xerrors.Errorf("miner SendCommit %v", err)
	}
	tipset, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		return session, err
	}
	m.ReqSession.Store(session, nil)
	defer func() {
		if err != nil {
			err := m.RecieveTaskResult(actorID, taskInfo.SectorNum, taskType, session, true, []byte(err.Error()))
			if err != nil {
				log.Errorf("SendCommit m.SendMsg actorID %d sectorNum %d error %v", actorID, taskInfo.SectorNum, err)
			}
			m.ReqSession.Delete(session)
		}
	}()

	minerAddr, err := util.NsNewIDAddress(uint64(actorID))
	if err != nil {
		return session, err
	}

	mi, err := m.LotusApi.StateMinerInfo(context.TODO(), minerAddr, tipset.Key())
	if err != nil {
		return session, err
	}

	fromStr := mi.Worker.String()

	proof := taskInfo.Proof

	enc := new(bytes.Buffer)
	params := &util.NsProveCommitSectorParams{
		SectorNumber: util.NsSectorNum(taskInfo.SectorNum),
		Proof:        proof,
	}

	if err := params.MarshalCBOR(enc); err != nil {
		return session, err
	}
	msgCID, err := m.SendMsg(fromStr, minerAddr.String(), int64(util.NsMethods.ProveCommitSector), deposit, fee, enc.Bytes(), func(isErr bool, result string) {
		err := m.RecieveTaskResult(actorID, taskInfo.SectorNum, taskType, session, isErr, []byte(result))
		m.ReqSession.Delete(session)
		log.Warnf("SendCommit actorID %d sectorNum %d result %s error %v", actorID, taskInfo.SectorNum, result, err)
	})

	reqInfo = util.RequestInfo{
		ActorID:   taskInfo.ActorID,
		SectorNum: taskInfo.SectorNum,
		TaskType:  taskInfo.TaskType,
		Session:   session,
	}
	_, errReserve := m.UpdateTaskLog(reqInfo, msgCID)
	if errReserve != nil {
		log.Warnf("SendCommit actorID %d sectorNum %d msgCID %s update tasklog error %v", actorID, taskInfo.SectorNum, msgCID, errReserve)
	}
	return msgCID, err
}

func (m *Miner) SendMsg(from string, to string, method int64, deposit string, fee string, params []byte, cb func(isErr bool, msgCID string)) (string, error) {

	toFstr, err := util.NsNewFromString(to)
	if err != nil {
		return "", err
	}
	fromFstr, err := util.NsNewFromString(from)
	if err != nil {
		return "", err
	}

	depositFIL, err := util.ParseFIL(deposit)
	if err != nil {
		return "", err
	}

	feeFIL, err := util.ParseFIL(fee)
	if err != nil {
		return "", err
	}
	msg := util.NsMessage{
		To:     toFstr,
		From:   fromFstr,
		Value:  util.Nsbig(depositFIL),
		Method: util.NsMethodNum(method),
		Params: params,
	}
	sigture, err := m.LotusApi.MpoolPushMessage(context.TODO(), &msg, &util.NsMessageSendSpec{util.Nsbig(feeFIL)})
	if err != nil {
		return "", err
	}
	go func() {
		if cb == nil {
			return
		}
		mw, err := m.LotusApi.StateWaitMsg(context.TODO(), sigture.Cid(), 3)
		if err != nil {
			cb(true, err.Error())
			return
		}
		switch mw.Receipt.ExitCode {
		case util.NsexitcodeOk:
			// this is what we expect
		case util.NsexitcodeSysErrInsufficientFunds:
			fallthrough
		case util.NsexitcodeSysErrOutOfGas:
			// gas estimator guessed a wrong number / out of funds
			cb(true, xerrors.Errorf("SysErrOutOfGas").Error())
			return
		default:
			cb(true, xerrors.Errorf("submitting sector proof failed exit=%d", mw.Receipt.ExitCode).Error())
			return
		}
		cb(false, sigture.Cid().String())
	}()

	return sigture.Cid().String(), nil
}
