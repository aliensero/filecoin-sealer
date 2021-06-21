package worker

import (
	"fmt"
	"time"

	"github.com/filecoin-project/go-bitfield"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

const (
	PUB = "pub"
	PRI = "pri"
)

func (w *Worker) AddPoStActor(actorID int64, addrinfo string, addrtype string) (string, error) {
	w.PoStMiner[actorID] = util.ActorPoStInfo{AddrInfo: addrinfo, AddrType: addrtype}
	log.Infof("AddPoStActor f0%v", actorID)
	return fmt.Sprintf("f0%d", actorID), nil
}

func (w *Worker) DisplayPoStActor() (string, error) {
	key := "PoSt actors:"
	for k := range w.PoStMiner {
		key = key + " " + fmt.Sprintf("f0%d", k)
	}
	return key, nil
}

func (w *Worker) WinPoStServer() {

	for {
		for a, info := range w.PoStMiner {
			ta := a
			tinfo := info

			minerPoStInfo, err := w.queryMinerPoStInfo(ta)
			if err != nil {
				log.Warnf("WinPoStServer actorID %v QueryMinerPoStInfo error %v", ta, err)
				w.CheckMiner()
				time.Sleep(3 * time.Second)
			}
			di := minerPoStInfo.Di
			if di.Open != info.DiOpen && di.CurrentEpoch >= di.Open && di.CurrentEpoch < di.Close {
				log.Infof("WinPoStServer actorID %v deadline %v StateMinerProvingDeadline", a, di)
				go w.SearchPartitions(di, minerPoStInfo, ta, info.AddrInfo, info.AddrType)
				tinfo.DiOpen = di.Open
				w.PoStMiner[ta] = tinfo
			} else {
				log.Warnf("WinPoStServer already exec actorID %v deadline %v error %v", a, di, err)
			}
		}
		time.Sleep(30 * time.Second)
	}
}

func (w *Worker) SearchPartitions(di util.NsDeadLineInfo, minerPoStInfo util.MinerPoStInfo, actorID int64, addrInfo string, addrType string) error {

	id := uint64(actorID)

	partitions := minerPoStInfo.Partitions
	log.Infof("WinPoStServer SearchPartitions len(partitions)=%d", len(partitions))
	if len(partitions) == 0 {
		return xerrors.Errorf("WinPoStServer len(partitions) == 0")
	}
	for partInx, part := range partitions {
		go w.GeneratePoStProof(di, id, part, minerPoStInfo.Rand, addrInfo, addrType, partInx)
	}
	return nil
}

func (w *Worker) GeneratePoStProof(di util.NsDeadLineInfo, minerID uint64, partition util.NsPartition, randomness util.NsPoStRandomness, addrInfo string, addrType string, partInx int) {

	// toProve, err := bitfield.SubtractBitField(partition.LiveSectors, partition.FaultySectors)
	// if err != nil {
	// 	log.Errorf("GeneratePoStProof bitfield.SubtractBitField ")
	// }
	var err error
	var addr util.NsAddress
	var toProve bitfield.BitField
	var repoProve bitfield.BitField
	var fault bitfield.BitField
	var proofs []util.VanillaProof
	var PoStProofType util.NsRegisteredPoStProof
	fcs := (*util.NsFallbackChallenges)(nil)

	addr, err = util.NsNewIDAddress(minerID)
	if err != nil {
		log.Warnf("PostErr NsNewIDAddress workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		err = xerrors.Errorf("PostErr NsNewIDAddress workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		return
	}

	for i := 0; i < 5; i++ {
		if w.MinerApi != nil && w.MinerApi.CheckServer() {
			break
		}
		err := w.ConnMiner()
		if err != nil {
			log.Errorf("PoStErr w.queryPoSt workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		}
		time.Sleep(3 * time.Second)
	}

	defer func() {
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}
		trnmsg := util.PoStTransfer{
			WorkerID:         w.WorkerID,
			HostName:         w.HostName,
			Actor:            addr,
			MinerID:          minerID,
			AddrInfo:         addrInfo,
			AddrType:         addrType,
			Dealine:          di.Index,
			CurrentEpoch:     int64(di.CurrentEpoch),
			CloseEpoch:       int64(di.Close),
			Challenge:        int64(di.Challenge),
			AllProveBitfield: toProve,
			ToProveBitfield:  repoProve,
			FaultBitfield:    fault,
			Proofs:           proofs,
			PartitionIndex:   partInx,
			Randomness:       randomness,
			PoStProofType:    PoStProofType,
			Err:              errMsg,
		}
		if w.MinerApi != nil {
			minerHandle := w.MinerApi.StartSubmitPoSt(trnmsg)
			log.Infof("PoStInfo result minerID %d deadline %d partitionIndex %d error %v submit to miner %s", minerID, di.Index, partInx, err, minerHandle)
		} else {
			log.Infof("PoStInfo result minerID %d deadline %d partitionIndex %d error %v", minerID, di.Index, partInx, err)
		}
	}()

	toProve, err = bitfield.SubtractBitField(partition.LiveSectors, partition.FaultySectors)
	if err != nil {
		err = xerrors.Errorf("PoStErr bitfield.SubtractBitField(partition.LiveSectors,partition.FaultySectors) workerid %s hostname %s minerID %v deadline %d error %v", w.WorkerID, w.HostName, minerID, di.Index, err)
		return
	}
	toProve, err = bitfield.MergeBitFields(toProve, partition.RecoveringSectors)
	if err != nil {
		err = xerrors.Errorf("PoStErr bitfield.MergeBitFields(partition.LiveSectors,partition.FaultySectors) workerid %s hostname %s minerID %v deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		return
	}
	whr := ""
	sset := make([]util.NsSectorNum, 0, 1024)
	toProve.ForEach(func(sid uint64) error {
		if whr == "" {
			whr = fmt.Sprintf("%d", sid)
		} else {
			whr = fmt.Sprintf("%s,%d", whr, sid)
		}
		sset = append(sset, util.NsSectorNum(sid))
		return nil
	})
	reqInfo := util.RequestInfo{
		ActorID:  int64(minerID),
		WorkerID: w.WorkerID,
		Sectors:  whr,
	}
	postInfos, err := w.queryPoSt(reqInfo)
	if err != nil || len(postInfos) == 0 {
		log.Errorf("PoStInfo QueryToPoSt minerID %d deadline %d partitionIdex %d len(postInfos) %d error %v", minerID, di.Index, partInx, len(postInfos), err)
		err = xerrors.Errorf("PoStInfo QueryToPoSt minerID %d deadline %d partitionIdex %d len(postInfos) %d error %v", minerID, di.Index, partInx, len(postInfos), err)
		return
	}
	log.Infof("PoStInfo QueryToPoSt minerID %d deadline %d partitionIdex %d len(postInfos) %d", minerID, di.Index, partInx, len(postInfos))
	tMap := make(map[uint64]util.DbPostInfo)
	repoProve = bitfield.New()
	for _, postInfo := range postInfos {
		pi := postInfo
		repoProve.Set(uint64(pi.SectorNum))
		sectorNum := uint64(pi.SectorNum)
		tMap[sectorNum] = pi
	}
	slen, err := repoProve.Count()
	if err != nil {
		log.Errorf("PostErr repoProve.Count workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		err = xerrors.Errorf("PostErr repoProve.Count workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err)
		return
	}

	fault = bitfield.New()
	proofs = make([]util.VanillaProof, 0, slen)

	randomness[31] &= 0x3f

	trepoProve := repoProve
	trepoProve.ForEach(func(sid uint64) error {
		cid, err1 := util.NsCidDecode(tMap[sid].CommR)
		if err1 != nil {
			err = xerrors.Errorf("PostErr repoProve.ForEach workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, minerID, di.Index, partInx, err1)
			return err1
		}

		if fcs == nil {
			PoStProofType = util.NsRegisteredPoStProof(tMap[sid].ProofType)
			fcs, err1 = util.NsGeneratePoStFallbackSectorChallenges(
				util.NsRegisteredPoStProof(PoStProofType),
				util.NsActorID(minerID),
				randomness,
				sset,
			)
			if err1 != nil {
				err = xerrors.Errorf("PostErr NsGeneratePoStFallbackSectorChallenges workerid %s hostName %s minerID %d deadline %d partitionIndex %d error %v", w.WorkerID, w.HostName, err1)
				return err1
			}
			log.Infof("PoStInfo minerID %d deadline %d partitionIndex %d challenge %v", di.Index, partInx, fcs)
		}

		s := util.NsSectorInfo{
			SealProof:    util.NsRegisteredSealProof(tMap[sid].ProofType),
			SectorNumber: util.NsSectorNum(tMap[sid].SectorNum),
			SealedCID:    cid,
		}
		pii := util.NsPrivateSectorInfo{
			CacheDirPath:     tMap[sid].CacheDirPath,
			PoStProofType:    PoStProofType,
			SealedSectorPath: tMap[sid].SealedSectorPath,
			SectorInfo:       s,
		}
		p, err := util.NsGenerateSingleVanillaProof(
			pii,
			fcs.Challenges[util.NsSectorNum(sid)],
		)
		if err != nil {
			repoProve.Unset(uint64(sid))
			fault.Set(uint64(sid))
			log.Warnf("PoStErr NsGenerateSingleVanillaProof minerID %d deadline %d partitionIndex %d sectorNum %d error %v", minerID, di.Index, partInx, sid, err)
			return nil
		}
		vp := util.VanillaProof{
			SectorNum: sid,
			Proof:     p,
		}
		proofs = append(proofs, vp)
		return nil
	})

}

func (w *Worker) RetryWinPoSt(actorID int64, addrInfo string, addrTyep string) (string, error) {

	minerPoStInfo, err := w.queryMinerPoStInfo(actorID)
	if err != nil {
		log.Warnf("actorID %v QueryMinerPoStInfo error %v", actorID, err)
		time.Sleep(3 * time.Second)
	}
	di := minerPoStInfo.Di
	if di.CurrentEpoch >= di.Open && di.CurrentEpoch < di.Close {
		err := w.SearchPartitions(di, minerPoStInfo, actorID, addrInfo, addrTyep)
		if err != nil {
			return "", err
		}
	} else {
		return "", xerrors.Errorf("actorID %d di.CurrentEpoch(%d) < di.Open(%d) || di.CurrentEpoch(%d) > di.Close(%d)", actorID, di.CurrentEpoch, di.Open, di.CurrentEpoch, di.Close)
	}
	return fmt.Sprintf("actorID %d di.Index %d di.CurrentEpoch %d di.Open %d di.Close %d ", actorID, di.Index, di.CurrentEpoch, di.Open, di.Close), nil
}
