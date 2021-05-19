package miner

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/filecoin-project/go-bitfield"
	"gitlab.ns/lotus-worker/util"
	"gitlab.ns/lotus-worker/worker"
	"golang.org/x/xerrors"
)

func (m *Miner) QueryMinerPoStInfo(actorID int64) (util.MinerPoStInfo, error) {
	miner, err := util.NsNewIDAddress(uint64(actorID))
	if err != nil {
		return util.MinerPoStInfo{}, err
	}

	buf := new(bytes.Buffer)
	if err := miner.MarshalCBOR(buf); err != nil {
		log.Error(xerrors.Errorf("failed to marshal address to cbor: %w", err))
		return util.MinerPoStInfo{}, err
	}

	di, err := m.LotusApi.StateMinerProvingDeadline(context.TODO(), miner, util.NsTipSetKey{})
	if err != nil {
		return util.MinerPoStInfo{}, err
	}
	partitions, err := m.LotusApi.StateMinerPartitions(context.TODO(), miner, di.Index, util.NsTipSetKey{})
	if err != nil {
		return util.MinerPoStInfo{}, err
	}

	ts, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		log.Error(xerrors.Errorf("WinPoStServer failed ChainGetTipSetByHeight: %w", err))
		return util.MinerPoStInfo{}, err
	}
	key := ts.Key()
	rand, err := m.LotusApi.ChainGetRandomnessFromBeacon(context.TODO(), key, util.NsDomainSeparationTag_WindowedPoStChallengeSeed, di.Challenge, buf.Bytes())
	if err != nil {
		log.Errorf("WinPoStServer SearchPartitions actorID %v height %v di.Challenge %d ChainGetRandomnessFromBeacon error %v", actorID, ts.Height(), di.Challenge, err)
		return util.MinerPoStInfo{}, err
	}
	return util.MinerPoStInfo{Di: *di, Partitions: partitions, Rand: util.NsPoStRandomness(rand)}, nil
}

func (m *Miner) CheckRecoveries(actorID int64, addrInfo string, addrType string, dlIdx uint64) (string, error) {

	faulty := uint64(0)
	params := &util.NsDeclareFaultsRecoveredParams{
		Recoveries: []util.NsRecoveryDeclaration{},
	}

	minerID, err := util.NsNewIDAddress(uint64(actorID))
	if err != nil {
		log.Errorf("CheckRecoveries NsNewIDAddress actorID %v deadline %d error %v", actorID, dlIdx, err)
		return "", err
	}

	ts, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		log.Errorf("CheckRecoveries ChainHead actorID %v deadline %d error %v", actorID, dlIdx, err)
		return "", err
	}

	partitions, err := m.LotusApi.StateMinerPartitions(context.TODO(), minerID, dlIdx, ts.Key())
	if err != nil {
		log.Errorf("CheckRecoveries StateMinerPartitions actorID %v deadline %d error %v", actorID, dlIdx, err)
		return "", err
	}

	for partIdx, partition := range partitions {
		unrecovered, err := bitfield.SubtractBitField(partition.FaultySectors, partition.RecoveringSectors)
		if err != nil {
			log.Errorf("CheckRecoveries bitfield.SubtractBitField actorID %v deadline %d error %v", actorID, dlIdx, err)
			return "", err
		}

		uc, err := unrecovered.Count()
		if err != nil {
			log.Errorf("CheckRecoveries unrecovered.Count actorID %v deadline %d error %v", actorID, dlIdx, err)
			return "", err
		}

		if uc == 0 {
			continue
		}

		faulty += uc

		params.Recoveries = append(params.Recoveries, util.NsRecoveryDeclaration{
			Deadline:  dlIdx,
			Partition: uint64(partIdx),
			Sectors:   unrecovered,
		})
	}

	if len(params.Recoveries) == 0 {
		return "", xerrors.Errorf("len(params.Recoveries)==0")
	}
	enc, aerr := util.NsactSerializeParams(params)
	if aerr != nil {
		log.Errorf("CheckRecoveries NsactSerializeParams actorID %v deadline %d error %v", actorID, dlIdx, err)
		return "", err
	}
	retCid := ""
	if addrInfo != "" && addrType == "pri" {
		prihex := addrInfo
		workerAddr, err := util.GenerateAddrByHexPri(prihex)
		if err != nil {
			log.Errorf("CheckRecoveries GenerateAddrByHexPri actorID %v deadline %d error %v", actorID, dlIdx, err)
			return "", err
		}
		nonce, err := m.checkNonce(workerAddr)
		if err != nil {
			log.Errorf("CheckRecoveries checkNonce actorID %v deadline %d error %v", actorID, dlIdx, err)
			return "", err
		}

		msg := util.NsMessage{
			To:     minerID,
			From:   workerAddr,
			Method: util.NsMethods.DeclareFaultsRecovered,
			Params: enc,
			Value:  util.NsNewInt(0),
			Nonce:  nonce,
		}

		msgptr, err := m.LotusApi.GasEstimateMessageGas(context.TODO(), &msg, nil, util.NsTipSetKey{})
		if err != nil {
			log.Errorf("CheckRecoveries GasEstimateMessageGas actorID %v deadline %d error %v", actorID, dlIdx, err)
			return "", err
		}
		msg = *msgptr

		log.Infof("SubmitRecovery message %v", msg)
		sigMsg, err := util.GenerateUtilSigMsg(m.LotusApi, prihex, msg)
		if err != nil {
			log.Errorf("CheckRecoveries GenerateUtilSigMsg actorID %v deadline %d error %v", actorID, dlIdx, err)
			return "", err
		}

		cid, err := m.LotusApi.MpoolPush(context.TODO(), sigMsg)
		if err != nil {
			log.Errorf("CheckRecoveries MpoolPush actorID %v deadline %d error %v", actorID, dlIdx, err)
			return "", err
		}
		retCid = cid.String()
	} else {
		msgCID, err := m.SendMsg(addrInfo, minerID.String(), int64(util.NsMethods.DeclareFaultsRecovered), "0", "0", enc, nil)
		if err != nil {
			log.Errorf("CheckRecoveries MpoolPush actorID %v deadline %d error %v", actorID, dlIdx, err)
			return "", err
		}
		retCid = msgCID
	}

	return retCid, nil
}

func (m *Miner) ReFindPoStTable(actorID int64) ([]int64, error) {
	db := m.Db
	var taskInfos []util.DbTaskInfo
	whr := "select a.actor_id,a.sector_num,b.worker_id,comm_r,cache_dir_path,sealed_sector_path,proof_type from db_task_infos as a left join (select actor_id,worker_id,sector_num from db_task_logs where id in (select max(id) from db_task_logs where state=2 and task_type='C1' group by sector_num,actor_id)) as b on a.actor_id=b.actor_id and a.sector_num=b.sector_num where a.actor_id= ? and a.task_type='PROVING';"
	if err := m.Db.Raw(whr, actorID).Scan(&taskInfos).Error; err != nil {
		return []int64{}, err
	}
	tx := db.Begin()
	retSec := make([]int64, len(taskInfos)+1)
	retSec[0] = int64(len(taskInfos))
	for i, t := range taskInfos {
		sectorNum := t.SectorNum
		commR := t.CommR
		cacheDirPath := t.CacheDirPath
		sealedSectorPath := t.SealedSectorPath
		workerID := t.WorkerID
		proofType := t.ProofType
		postInfo := util.DbPostInfo{
			ActorID:          actorID,
			SectorNum:        *sectorNum,
			CommR:            commR,
			CacheDirPath:     cacheDirPath,
			SealedSectorPath: sealedSectorPath,
			ProofType:        proofType,
			WorkerID:         workerID,
			State:            util.SUCCESS,
		}
		if err := tx.Where("actor_id = ? and sector_num = ? and worker_id = ? and state = ?", actorID, sectorNum, workerID, util.SUCCESS).FirstOrCreate(&postInfo).Error; err != nil {
			log.Warnf("ReFindPoStTable actorID %d sectorNum %d workerID %s error %v", actorID, sectorNum, workerID, err)
			continue
		}
		retSec[i+1] = *sectorNum
	}
	tx.Commit()
	return retSec, nil
}

type SyncPoStMap struct {
	L        sync.Mutex
	PostMap  map[util.NsAddress]map[int]chan util.PoStTransfer
	CloseMap map[util.NsAddress]chan interface{}
}

func NewSyncPostMap() *SyncPoStMap {
	pmap := make(map[util.NsAddress]map[int]chan util.PoStTransfer)
	synp := new(SyncPoStMap)
	synp.PostMap = pmap
	synp.CloseMap = make(map[util.NsAddress]chan interface{})
	return synp
}

func (m *Miner) ResetSyncPoStMap() string {
	log.Warnf("PoStInfo befer reset m.SyncPoStMap %v", m.SyncPoStMap)
	m.SyncPoStMap = NewSyncPostMap()
	log.Warnf("PoStInfo after reset m.SyncPoStMap %v", m.SyncPoStMap)
	return "reset syncPoStMap"
}

func (m *Miner) StartSubmitPoSt(trnmsg util.PoStTransfer) string {

	workerid := trnmsg.WorkerID
	hostname := trnmsg.HostName
	actorID := trnmsg.Actor
	minerID := trnmsg.MinerID
	deadline := trnmsg.Dealine
	partInx := trnmsg.PartitionIndex

	log.Infof("PoStInfo StartSubmitPoSt SyncPoStMap %v workerid %s hostname %s actorID %v minerID %d deadline %d partitionIndex %d worker error【%s】", m.SyncPoStMap, workerid, hostname, actorID, minerID, deadline, partInx, trnmsg.Err)
	if trnmsg.Err != "" {
		return "miner not handle"
	}
	m.SyncPoStMap.L.Lock()
	defer m.SyncPoStMap.L.Unlock()
	partMap, ok := m.SyncPoStMap.PostMap[trnmsg.Actor]
	if ok {
		partChan, ok1 := partMap[trnmsg.PartitionIndex]
		if ok1 && partChan != nil {
			log.Infof("PoStInfo StartSubmitPoSt partChan %v ok1 %v", partChan, ok1)
			partChan <- trnmsg
		} else {
			d := (trnmsg.CloseEpoch - trnmsg.CurrentEpoch) * 30
			partChan = make(chan util.PoStTransfer, 1024)
			partMap[trnmsg.PartitionIndex] = partChan
			go m.submitPoSt(trnmsg.AddrInfo, trnmsg.AddrType, d, trnmsg.CurrentEpoch, partChan)
			partChan <- trnmsg
		}
	} else {
		d := (trnmsg.CloseEpoch - trnmsg.CurrentEpoch) * 30
		partChan := make(chan util.PoStTransfer, 1024)
		partMap = make(map[int]chan util.PoStTransfer)
		partMap[trnmsg.PartitionIndex] = partChan
		m.SyncPoStMap.PostMap[trnmsg.Actor] = partMap
		go m.submitPoSt(trnmsg.AddrInfo, trnmsg.AddrType, d, trnmsg.CurrentEpoch, partChan)
		partChan <- trnmsg
	}
	return fmt.Sprintf("PoStInfo actorID %v deadline %d partionIndex %d", actorID, deadline, partInx)

}

func (m *Miner) submitPoSt(addrInfo string, addrType string, duration int64, current int64, trnmsgChan chan util.PoStTransfer) {
	d := time.Duration(duration)
	ac := time.After(d * time.Second)
	proofs := make([]util.VanillaProof, 0, 4396)
	var index int = -1
	var actor *util.NsAddress
	var deadline int64 = -1
	var challenge int64 = -1
	PoStProofType := (*util.NsRegisteredPoStProof)(nil)
	var randomness util.NsPoStRandomness
	var minerID int64 = -1
	allProveBitfield := bitfield.New()
	addProveBitfield := bitfield.New()
	faultBitfield := bitfield.New()
	closeCh := make(chan interface{}, 1)
	log.Infof("PoStInfo submitPoSt src duration %d duration %v", duration, d)
loop:
	for {
		select {
		case t := <-trnmsgChan:
			log.Infof("PoStInfo submitPoSt loop transaction error %v", t.Err)
			if index == -1 {
				index = t.PartitionIndex
			}
			if actor == nil {
				actor = &t.Actor
				m.SyncPoStMap.CloseMap[*actor] = closeCh
			}
			if deadline == -1 {
				deadline = int64(t.Dealine)
			}
			if challenge == -1 {
				challenge = t.Challenge
			}
			if PoStProofType == nil {
				PoStProofType = &t.PoStProofType
			}
			if randomness == nil {
				randomness = t.Randomness
			}
			if minerID == -1 {
				minerID = int64(t.MinerID)
			}
			acnt, err := allProveBitfield.Count()
			if err != nil {
				log.Errorf("PoStInfo submitPoSt allProveBitfield.Count() error %v", err)
				return
			}
			if acnt == 0 {
				allProveBitfield = t.AllProveBitfield
				acnt, err = allProveBitfield.Count()
				if err != nil {
					log.Errorf("PoStInfo submitPoSt allProveBitfield.Count() error %v", err)
					return
				}
			}
			addProveBitfield, err = bitfield.MergeBitFields(addProveBitfield, t.ToProveBitfield)
			if err != nil {
				log.Errorf("PoStInfo submitPoSt bitfield.MergeBitFields(addProveBitfield, t.ToProveBitfield) error %v", err)
				return
			}
			faultBitfield, err = bitfield.MergeBitFields(faultBitfield, t.FaultBitfield)
			if err != nil {
				log.Errorf("PoStInfo submitPoSt bitfield.MergeBitFields(faultBitfield, t.FaultBitfield) error %v", err)
				return
			}
			proofs = append(proofs, t.Proofs...)
			tcnt, err := addProveBitfield.Count()
			if err != nil {
				log.Errorf("PoStInfo submitPoSt addProveBitfield.Count() error %v", err)
				return
			}
			fcnt, err := faultBitfield.Count()
			if err != nil {
				log.Errorf("PoStInfo submitPoSt faultBitfield.Count() error %v", err)
				return
			}
			log.Infof("PoStInfo submitPoSt actorID %v deadline %d partitionIndex %d allProveBitfield count %d addProveBitfield %d faultBitfield %d", actor, deadline, index, acnt, tcnt, fcnt)
			if acnt == tcnt+fcnt {
				break loop
			}
		case <-ac:
			log.Errorf("PoStInfo timeout actorID %v deadline %d partitionIndex %d", actor, deadline, index)
			return
		case <-closeCh:
			log.Warnf("PoStInfo abort actorID %v deadline %d partitionIndex %d", actor, deadline, index)
			return
		}
	}
	if len(proofs) == 0 {
		log.Infof("PoStInfo submitPoSt proofs len(proofs) %d", len(proofs))
		return
	}
	{
		_, ok := m.SyncPoStMap.PostMap[*actor]
		if ok {
			_, ok := m.SyncPoStMap.PostMap[*actor][index]
			if ok {
				m.SyncPoStMap.PostMap[*actor][index] = nil
			}
			delete(m.SyncPoStMap.CloseMap, *actor)
		}
	}
	vanillaMap := make(map[uint64][]byte)
	for _, vp := range proofs {
		vanillaMap[vp.SectorNum] = vp.Proof
	}
	sortedProofs := make([][]byte, 0, len(proofs))
	allProveBitfield.ForEach(func(sid uint64) error {
		sortedProofs = append(sortedProofs, vanillaMap[sid])
		return nil
	})
	log.Warnf("PoStInfo start snarkproof actorID %v minerID %v deadline %d partitionIndex %d", actor, minerID, deadline, index)
	cpPoStType := *PoStProofType
	starkProofs, err := util.NsGenerateWindowPoStWithVanilla(
		cpPoStType,
		util.NsActorID(minerID),
		randomness,
		sortedProofs,
	)
	if err != nil {
		sectors := make([]uint64, 0, 3096)
		allProveBitfield.ForEach(func(sid uint64) error {
			sectors = append(sectors, sid)
			return nil
		})
		log.Errorf("PoStInfo NsGenerateWindowPoStWithVanilla actorID %v  minerID %v PoStProofType %d deadline %d partitionIndex %d len(proofs) %d sectors %v error %v", actor, minerID, cpPoStType, deadline, index, len(proofs), sectors, err)
		return
	}
	log.Warnf("PoStInfo finish snarkproof actorID %v minerID %v PoStProofType %d partitionIndex %d", actor, minerID, deadline, index)

	ts, err := m.LotusApi.ChainGetTipSetByHeight(context.TODO(), util.NsChainEpoch(challenge), util.NsTipSetKey{})
	if err != nil {
		log.Errorf("PoStInfo submitPoSt actor %v deadline %d partionIndex %d ChainGetTipSetByHeight error %v", actor, deadline, index, err)
		return
	}
	commRand, err := m.LotusApi.ChainGetRandomnessFromTickets(context.TODO(), ts.Key(), util.NsDomainSeparationTag_PoStChainCommit, util.NsChainEpoch(challenge), nil)
	if err != nil {
		log.Errorf("PoStInfo submitPoSt actor %v deadline %d partionIndex %d ChainGetRandomnessFromTickets error %v", actor, deadline, index, err)
		return
	}
	poStParams := &util.NsSubmitWindowedPoStParams{
		Deadline:         uint64(deadline),
		Partitions:       []util.NsPoStPartition{{Index: uint64(index)}},
		Proofs:           starkProofs,
		ChainCommitEpoch: util.NsChainEpoch(challenge),
		ChainCommitRand:  commRand,
	}

	enc, aerr := util.NsactSerializeParams(poStParams)
	if aerr != nil {
		log.Errorf("PoStInfo submitPoSt actor %v deadline %d partionIndex %d error %v", actor, deadline, index, aerr)
		return
	}

	toAddr := *actor
	retCid := ""
	if addrInfo != "" && addrType == worker.PRI {
		prihex := addrInfo
		workerAddr, err := util.GenerateAddrByHexPri(prihex)
		if err != nil {
			log.Errorf("PoStInfo submitPoSt StateMinerInfo actor %v deadline %d partionIndex %d error %v", actor, deadline, index, err)
			return
		}
		noce, err := m.checkNonce(workerAddr)
		if err != nil {
			log.Errorf("PoStInfo submitPoSt checkNonce actor %v deadline %d partionIndex %d error %v", actor, deadline, index, err)
			return
		}
		msg := util.NsMessage{
			To:     toAddr,
			From:   workerAddr,
			Method: util.NsMethods.SubmitWindowedPoSt,
			Params: enc,
			Value:  util.NsNewInt(0),
			Nonce:  noce,
		}
		msgptr, err := m.LotusApi.GasEstimateMessageGas(context.TODO(), &msg, nil, util.NsTipSetKey{})
		if err != nil {
			log.Errorf("PoStInfo submitPoSt GasEstimateMessageGas actor %v deadline %d partionIndex %d error %v", actor, deadline, index, err)
			return
		}
		msg = *msgptr
		log.Infof("PoStInfo submitPoSt message %v", msg)
		sigMsg, err := util.GenerateUtilSigMsg(m.LotusApi, prihex, msg)
		if err != nil {
			log.Errorf("PoStInfo submitPoSt actor %v deadline %d partionIndex %d error %v", actor, deadline, index, err)
			return
		}

		cid, err := m.LotusApi.MpoolPush(context.TODO(), sigMsg)
		if err != nil {
			log.Errorf("PoStInfo submitPoSt actor %v deadline %d partionIndex %d error %v", actor, deadline, index, err)
			return
		}
		retCid = cid.String()
	} else {
		msgCID, err := m.SendMsg(addrInfo, toAddr.String(), int64(util.NsMethods.SubmitWindowedPoSt), "0", "0", enc, nil)
		if err != nil {
			log.Errorf("PoStInfo submitPoSt actor %v deadline %d partionIndex %d error %v", actor, deadline, index, err)
			return
		}
		retCid = msgCID
	}

	log.Errorf("PoStInfo submitPoSt actor %v minerID %d deadline %d partionIndex %d cid %v", actor, minerID, deadline, index, retCid)
	return
}

func (m *Miner) AbortPoSt(actorID int64) (string, error) {
	actor, err := util.NsNewIDAddress(uint64(actorID))
	if err != nil {
		return "", err
	}
	c, ok := m.SyncPoStMap.CloseMap[actor]
	if ok {
		c <- nil
	} else {
		return "", xerrors.Errorf("actorID %v PoSt not exist", actor)
	}
	return fmt.Sprintf("abort actorID %v", actor), nil
}
