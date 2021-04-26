package miner

import (
	"context"
	"fmt"

	"github.com/fatih/color"
	"gitlab.ns/lotus-worker/util"
	"golang.org/x/xerrors"
)

func (m *Miner) StateSectorPreCommitInfo(actorID string, sectorNum int64) (util.NsSectorPreCommitOnChainInfo, error) {
	tipset, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		return util.NsSectorPreCommitOnChainInfo{}, err
	}
	minerAddr, err := util.NsNewFromString(actorID)
	if err != nil {
		return util.NsSectorPreCommitOnChainInfo{}, err
	}
	return m.LotusApi.StateSectorPreCommitInfo(context.TODO(), minerAddr, util.NsSectorNum(sectorNum), tipset.Key())
}

func (m *Miner) StateSectorCommitInfo(actorID string, sectorNum int64) (*util.NsSectorOnChainInfo, error) {
	tipset, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		return nil, err
	}
	minerAddr, err := util.NsNewFromString(actorID)
	if err != nil {
		return nil, err
	}
	return m.LotusApi.StateSectorGetInfo(context.TODO(), minerAddr, util.NsSectorNum(sectorNum), tipset.Key())
}

func (m *Miner) StateSectorPartition(actorID string, sectorNum int64) (*util.NsSectorLocation, error) {
	tipset, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		return nil, err
	}
	minerAddr, err := util.NsNewFromString(actorID)
	if err != nil {
		return nil, err
	}
	return m.LotusApi.StateSectorPartition(context.TODO(), minerAddr, util.NsSectorNum(sectorNum), tipset.Key())
}

func (m *Miner) MinerInfo(actorID uint64) (string, error) {
	maddr, err := util.NsNewIDAddress(actorID)
	if err != nil {
		return "", err
	}

	head, err := m.LotusApi.ChainHead(context.TODO())
	if err != nil {
		return "", xerrors.Errorf("getting chain head: %w", err)
	}

	mact, err := m.LotusApi.StateGetActor(context.TODO(), maddr, head.Key())
	if err != nil {
		return "", err
	}

	stor := util.NsActorStore(context.TODO(), util.NsNewAPIBlockstore(m.LotusApi))

	mas, err := util.NsMinerLoad(stor, mact)
	if err != nil {
		return "", err
	}

	cd, err := m.LotusApi.StateMinerProvingDeadline(context.TODO(), maddr, head.Key())
	if err != nil {
		return "", xerrors.Errorf("getting miner info: %w", err)
	}

	balance := util.NsFIL(mact.Balance).Short()
	fmt.Printf("Miner: %s\n", color.BlueString("%s", maddr))
	fmt.Printf("Balance: %s\n", color.BlueString("%s", balance))

	proving := uint64(0)
	faults := uint64(0)
	recovering := uint64(0)
	curDeadlineSectors := uint64(0)

	if err := mas.ForEachDeadline(func(dlIdx uint64, dl util.NsDeadline) error {
		return dl.ForEachPartition(func(partIdx uint64, part util.NsMinerPartition) error {
			if bf, err := part.LiveSectors(); err != nil {
				return err
			} else if count, err := bf.Count(); err != nil {
				return err
			} else {
				proving += count
				if dlIdx == cd.Index {
					curDeadlineSectors += count
				}
			}

			if bf, err := part.FaultySectors(); err != nil {
				return err
			} else if count, err := bf.Count(); err != nil {
				return err
			} else {
				faults += count
			}

			if bf, err := part.RecoveringSectors(); err != nil {
				return err
			} else if count, err := bf.Count(); err != nil {
				return err
			} else {
				recovering += count
			}

			return nil
		})
	}); err != nil {
		return "", xerrors.Errorf("walking miner deadlines and partitions: %w", err)
	}

	var faultPerc float64
	if proving > 0 {
		faultPerc = float64(faults*10000/proving) / 100
	}

	ret := fmt.Sprintf("Balance: %s; Current Epoch: %d;Challenge Epoch: %d; Proving Period Boundary: %d; Faults: %d (%.2f%%); Recovering: %d; Deadline Index: %d; Deadline Sectors: %d", balance, cd.CurrentEpoch, cd.Challenge, cd.PeriodStart%cd.WPoStProvingPeriod, faults, faultPerc, recovering, cd.Index, curDeadlineSectors)

	fmt.Printf("Current Epoch:           %d\n", cd.CurrentEpoch)

	fmt.Printf("Proving Period Boundary: %d\n", cd.PeriodStart%cd.WPoStProvingPeriod)

	fmt.Printf("Faults:      %d (%.2f%%)\n", faults, faultPerc)
	fmt.Printf("Recovering:  %d\n", recovering)

	fmt.Printf("Deadline Index:       %d\n", cd.Index)
	fmt.Printf("Deadline Sectors:     %d\n", curDeadlineSectors)
	return ret, nil
}
