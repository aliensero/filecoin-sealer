package miner

import (
	"context"

	"gitlab.ns/lotus-worker/util"
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
