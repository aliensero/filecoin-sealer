package miner

import (
	"gitlab.ns/lotus-worker/util"
)

func (m *Miner) GenerateAddress(addrType util.NsKeyType) (string, error) {
	pri, err := util.GeneratePriKeyHex(addrType)
	if err != nil {
		return "", err
	}
	addr, err := util.GenerateAddrByHexPri(pri)
	if err != nil {
		return "", err
	}
	addressInfo := util.DbAddressInfo{
		PrivateKey: pri,
		Address:    addr.String(),
	}
	err = m.Db.Create(&addressInfo).Error
	if err != nil {
		return "", err
	}
	return addr.String(), nil
}
