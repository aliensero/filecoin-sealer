package miner

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func (m *Miner) SendPreCommitByPrivatKey(prihex string, actorID int64, sectorNum int64, deposit string) (string, error) {

	session := uuid.New().String()
	taskType := util.PRECOMMIT
	reqInfo := util.RequestInfo{
		ActorID:   actorID,
		SectorNum: sectorNum,
		Session:   session,
		TaskType:  taskType,
	}
	var err error
	depositFIL, err := util.ParseFIL(deposit)
	if err != nil {
		return "", err
	}

	minerAddr, err := util.NsNewIDAddress(uint64(actorID))
	if err != nil {
		return "", err
	}

	waddr, err := util.GenerateAddrByHexPri(prihex)
	if err != nil {
		return "", err
	}

	taskInfo, err := m.QueryTask(reqInfo)

	if err != nil && !strings.Contains(err.Error(), "record not found") {
		return session, err
	}
	if err != nil && strings.Contains(err.Error(), "record not found") {
		log.Warnf("miner SendPreCommitByPrivatKey record not found")
		return "", xerrors.Errorf("miner SendPreCommitByPrivatKey %v", err)
	}
	m.ReqSession.Store(session, nil)
	defer func() {
		if err != nil {
			log.Errorf("sendMsg defer error %v", err)
			err1 := m.RecieveTaskResult(actorID, *taskInfo.SectorNum, taskType, session, true, []byte(err.Error()))
			if err1 != nil {
				log.Errorf("SendPreCommitByPrivatKey actorID %d sectorNum %d error %v", actorID, *taskInfo.SectorNum, err1)
			}
			m.ReqSession.Delete(session)
		}
	}()

	nonce, err := m.checkNonce(waddr)
	if err != nil {
		return "", err
	}
	expiration := taskInfo.TicketEpoch + m.SectorExpiration

	sealedCID, err := util.Parse(taskInfo.CommR)
	if err != nil {
		return session, err
	}

	params := &util.NsSectorPreCommitInfo{
		Expiration:   util.NsChainEpoch(expiration),
		SectorNumber: util.NsSectorNum(*taskInfo.SectorNum),
		SealProof:    util.NsRegisteredSealProof(taskInfo.ProofType),

		SealedCID:     sealedCID,
		SealRandEpoch: util.NsChainEpoch(taskInfo.TicketEpoch),
	}
	enc := new(bytes.Buffer)
	if err := params.MarshalCBOR(enc); err != nil {
		return session, err
	}

	msg := util.NsMessage{
		To:     minerAddr,
		From:   waddr,
		Value:  util.Nsbig(depositFIL),
		Method: util.NsMethodNum(6),
		Nonce:  nonce,
		Params: enc.Bytes(),
	}
	cbvarSession := session
	cbvaractorID := actorID
	cbvarsectorNum := *taskInfo.SectorNum
	cbvartaskType := taskType
	msgCID, err := m.sendMsgByPrivatKey(prihex, msg, func(isErr bool, result string) {
		err := m.RecieveTaskResult(cbvaractorID, cbvarsectorNum, cbvartaskType, cbvarSession, isErr, []byte(result))
		log.Warnf("cbvarsectorNum % d cbvarSession %v", cbvarsectorNum, cbvarSession)
		m.ReqSession.Delete(cbvarSession)
		log.Warnf("SendPreCommitByPrivatKey actorID %d sectorNum %d result %s error %v", cbvaractorID, cbvarsectorNum, result, err)
	})

	if err != nil {
		return msgCID, err
	}

	reqInfo = util.RequestInfo{
		ActorID:   *taskInfo.ActorID,
		SectorNum: *taskInfo.SectorNum,
		TaskType:  taskInfo.TaskType,
		Session:   session,
	}
	_, errReserve := m.UpdateTaskLog(reqInfo, msgCID)
	if errReserve != nil {
		log.Warnf("SendPreCommitByPrivatKey actorID %d sectorNum %d msgCID %s update tasklog error %v", actorID, *taskInfo.SectorNum, msgCID, errReserve)
	}

	return msgCID, err
}

func (m *Miner) SendCommitByPrivatKey(prihex string, actorID int64, sectorNum int64, deposit string) (string, error) {

	session := uuid.New().String()
	taskType := util.COMMIT
	reqInfo := util.RequestInfo{
		ActorID:   actorID,
		SectorNum: sectorNum,
		Session:   session,
		TaskType:  taskType,
	}

	var err error
	depositFIL, err := util.ParseFIL(deposit)
	if err != nil {
		return "", err
	}
	minerAddr, err := util.NsNewIDAddress(uint64(actorID))
	if err != nil {
		return "", err
	}

	waddr, err := util.GenerateAddrByHexPri(prihex)
	if err != nil {
		return "", err
	}

	taskInfo, err := m.QueryTask(reqInfo)

	if err != nil && !strings.Contains(err.Error(), "record not found") {
		return session, err
	}
	if err != nil && strings.Contains(err.Error(), "record not found") {
		log.Warnf("miner SendCommitByPrivatKey record not found")
		return "", xerrors.Errorf("miner SendCommitByPrivatKey %v", err)
	}

	m.ReqSession.Store(session, nil)
	defer func() {
		if err != nil {
			log.Errorf("sendMsg defer error %v", err)
			err1 := m.RecieveTaskResult(actorID, *taskInfo.SectorNum, taskType, session, true, []byte(err.Error()))
			if err1 != nil {
				log.Errorf("SendCommitByPrivatKey actorID %d sectorNum %d error %v", actorID, *taskInfo.SectorNum, err1)
			}
			m.ReqSession.Delete(session)
		}
	}()

	nonce, err := m.checkNonce(waddr)
	if err != nil {
		return session, err
	}

	proof := taskInfo.Proof

	enc := new(bytes.Buffer)
	params := &util.NsProveCommitSectorParams{
		SectorNumber: util.NsSectorNum(*taskInfo.SectorNum),
		Proof:        proof,
	}

	if err := params.MarshalCBOR(enc); err != nil {
		return session, err
	}

	msg := util.NsMessage{
		To:     minerAddr,
		From:   waddr,
		Value:  util.Nsbig(depositFIL),
		Method: util.NsMethodNum(7),
		Nonce:  nonce,
		Params: enc.Bytes(),
	}
	cbvarSession := session
	cbvaractorID := actorID
	cbvarsectorNum := *taskInfo.SectorNum
	cbvartaskType := taskType
	msgCID, err := m.sendMsgByPrivatKey(prihex, msg, func(isErr bool, result string) {
		err := m.RecieveTaskResult(cbvaractorID, cbvarsectorNum, cbvartaskType, cbvarSession, isErr, []byte(result))
		log.Warnf("cbvarsectorNum % d cbvarSession2 %v", cbvarsectorNum, cbvarSession)
		m.ReqSession.Delete(cbvarSession)
		log.Warnf("SendCommitByPrivatKey actorID %d sectorNum %d result %s error %v", actorID, cbvarsectorNum, result, err)
	})

	if err != nil {
		return msgCID, err
	}

	reqInfo = util.RequestInfo{
		ActorID:   *taskInfo.ActorID,
		SectorNum: *taskInfo.SectorNum,
		TaskType:  taskInfo.TaskType,
		Session:   cbvarSession,
	}
	_, errReserve := m.UpdateTaskLog(reqInfo, msgCID)
	if errReserve != nil {
		log.Warnf("SendCommitByPrivatKey actorID %d sectorNum %d msgCID %s update tasklog error %v", actorID, *taskInfo.SectorNum, msgCID, errReserve)
	}
	return msgCID, err
}

func (m *Miner) sendMsgByPrivatKey(prihex string, msg util.NsMessage, cbs ...func(bool, string)) (string, error) {

	ts, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		log.Errorf("sendMsgByPrivatKey m.LoutsApi.ChainHead error %v", err)
		return "", err
	}

	msgptr, err := m.LotusApi.GasEstimateMessageGas(context.TODO(), &msg, nil, ts.Key())
	if err != nil {
		log.Errorf("sendMsgByPrivatKey GasEstimateMessageGas error %v", err)
		return "", err
	}
	msg = *msgptr
	sigMsg, err := util.GenerateUtilSigMsg(m.LotusApi, prihex, msg)
	if err != nil {
		log.Errorf("Miner sendMsgByPrivatKey GenerateUtilSigMsg error %v", err)
		return "", err
	}
	cid, err := m.LotusApi.MpoolPush(context.TODO(), sigMsg)
	if err != nil {
		log.Errorf("Miner sendMsgByPrivatKey MpoolPush error %v", err)
		return "", err
	}
	if len(cbs) != 0 {
		for _, cb := range cbs {
			go func() {
				receipt, err := m.LotusApi.StateGetReceipt(context.TODO(), cid, util.NsTipSetKey{})
				for i := 0; i < 10; i++ {
					if err == nil && receipt != nil {
						break
					}
					receipt, err = m.LotusApi.StateGetReceipt(context.TODO(), cid, util.NsTipSetKey{})
					time.Sleep(30 * time.Second)
				}
				if err != nil {
					cb(true, err.Error())
					return
				}
				if receipt == nil {
					cb(true, fmt.Sprintf("Miner sendMsgByPrivatKey cid %v StateGetReceipt is nil", cid))
					return
				}
				switch receipt.ExitCode {
				case util.NsexitcodeOk:
					// this is what we expect
				case util.NsexitcodeSysErrInsufficientFunds:
					fallthrough
				case util.NsexitcodeSysErrOutOfGas:
					// gas estimator guessed a wrong number / out of funds
					cb(true, xerrors.Errorf("SysErrOutOfGas").Error())
					return
				default:
					cb(true, xerrors.Errorf("submitting sector proof failed exit=%d", receipt.ExitCode).Error())
					return
				}
				log.Warnf("Miner sendMsgByPrivatKey StateWaitMsg cid %s error %v", cid.String(), err)
				cb(false, cid.String())
			}()
		}
	}
	log.Warnf("Miner sendMsgByPrivatKey StateWaitMsg len(cbs) %d", len(cbs))
	return cid.String(), nil
}

func (m *Miner) WithDrawByPrivatKey(prihex string, actorID uint64, amtstr string) (string, error) {

	var err error
	addr, err := util.GenerateAddrByHexPri(prihex)
	if err != nil {
		return "", err
	}

	amt, err := util.ParseFIL(amtstr)
	if err != nil {
		return "", err
	}

	params, err := util.NsactSerializeParams(&util.NsWithdrawBalanceParams{
		AmountRequested: util.Nsbig(amt), // Default to attempting to withdraw all the extra funds in the miner actor
	})
	if err != nil {
		return "", err
	}

	nonce, err := m.checkNonce(addr)
	if err != nil {
		return "", err
	}

	msg := util.NsMessage{
		To:     addr,
		From:   addr,
		Value:  util.NsNewInt(0),
		Method: util.NsWithdrawBalance,
		Nonce:  nonce,
		Params: params,
	}
	return m.sendMsgByPrivatKey(prihex, msg)
}

func (m *Miner) TxByPrivatKey(prihex string, to string, amt string) (string, error) {

	var err error
	addr, err := util.GenerateAddrByHexPri(prihex)
	if err != nil {
		return "", err
	}

	toAddr, err := util.NsNewFromString(to)
	if err != nil {
		return "", err
	}

	val, err := util.ParseFIL(amt)
	if err != nil {
		return "", err
	}

	nonce, err := m.checkNonce(addr)
	if err != nil {
		return "", err
	}

	msg := util.NsMessage{
		To:     toAddr,
		From:   addr,
		Value:  util.Nsbig(val),
		Nonce:  nonce,
		Method: util.NsMethodNum(0),
	}
	return m.sendMsgByPrivatKey(prihex, msg)
}

func (m *Miner) CreateMiner(prihex, owner, worker string, sealProofType int64) (string, error) {

	var err error
	addr, err := util.NsNewFromString(owner)
	if err != nil {
		return "", err
	}
	nonce, err := m.checkNonce(addr)
	if err != nil {
		return "", err
	}

	sigMsg, err := util.GenerateCreateMinerSigMsg(m.LotusApi, prihex, owner, worker, sealProofType, nonce)
	if err != nil {
		return "", err
	}

	cid, err := m.LotusApi.MpoolPush(context.TODO(), sigMsg)
	if err != nil {
		return "", err
	}
	return cid.String(), nil
}

func (m *Miner) checkNonce(addr util.NsAddress) (uint64, error) {

	curts, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		return 0, err
	}
	nonce, err := m.LotusApi.MpoolGetNonce(context.TODO(), addr)
	if err != nil {
		act, err := m.LotusApi.StateGetActor(context.TODO(), addr, curts.Key())
		if err != nil {
			return 0, err
		}
		nonce = act.Nonce
	}
	log.Warnf("checkNonceAfter actorID %s,nonce %d", addr.String(), nonce)
	return nonce, nil
}

func (m *Miner) CreateMinerWithOutToken(prihex, owner, worker string, sealProofType int64) (string, error) {
	var err error
	curts, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		return "", err
	}
	addrowner, err := util.NsNewFromString(owner)
	if err != nil {
		return "", err
	}
	addrworker, err := util.NsNewFromString(worker)
	if err != nil {
		return "", err
	}
	nonce, err := m.LotusApi.MpoolGetNonce(context.TODO(), addrworker)
	if err != nil {
		act, err := m.LotusApi.StateGetActor(context.TODO(), addrworker, curts.Key())
		if err != nil {
			log.Infof("CreateMinerWithOutToken get nonce error %v", err)
			return "", err
		}
		nonce = act.Nonce
	}
	sigMsg, err := util.GenerateCreateMinerSigMsgByAddress(prihex, addrowner, addrworker, sealProofType, nonce)
	if err != nil {
		return "", err
	}
	log.Infof("CreateMinerWithOutToken sigture message %v", sigMsg)
	cid, err := m.LotusApi.MpoolPush(context.TODO(), sigMsg)
	if err != nil {
		return "", err
	}
	return cid.String(), nil
}

func StructToMap(obj interface{}) map[string]interface{} {
	obj1 := reflect.TypeOf(obj)
	obj2 := reflect.ValueOf(obj)

	var data = make(map[string]interface{})
	for i := 0; i < obj1.NumField(); i++ {
		data[obj1.Field(i).Name] = obj2.Field(i).Interface()
	}
	return data
}
