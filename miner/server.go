package miner

import (
	"time"

	logging "github.com/ipfs/go-log/v2"
)

var logserver = logging.Logger("miner-server")

func (m *Miner) AddToServer(actorID int64, privatKey string) error {
	m.ActorTask[actorID] = privatKey
	return nil
}

func (m *Miner) DeleteFromServer(actorID int64) error {
	delete(m.ActorTask, actorID)
	return nil
}

func (m *Miner) DisplayServer() (interface{}, error) {
	var as []int64
	for a := range m.ActorTask {
		as = append(as, a)
	}
	return as, nil
}

func (m *Miner) Server() {
	for {
		for actorID, tpk := range m.ActorTask {
			tactorID := actorID
			pk := tpk
			go func() {
				ret, err := m.GetSeedRand(tactorID, -1)
				if err != nil {
					log.Errorf("actor id %d GetSeedRand error %v", tactorID, err)
					return
				}
				log.Infof("actor id %d GetSeedRand result %s", tactorID, ret)
			}()
			go func() {
				ret, err := m.SendCommitByPrivatKey(pk, tactorID, -1, "0")
				if err != nil {
					log.Errorf("actor id %d SendPreCommit error %v", tactorID, err)
					return
				}
				log.Infof("actor id %d SendPreCommit result %s", tactorID, ret)
			}()
			go func() {
				ret, err := m.SendPreCommitByPrivatKey(pk, tactorID, -1, "0")
				if err != nil {
					log.Errorf("actor id %d SendCommit error %v", tactorID, err)
					return
				}
				log.Infof("actor id %d SendCommit result %s", tactorID, ret)
			}()
		}
		time.Sleep(30 * time.Second)
	}
}
