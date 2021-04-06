package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"golang.org/x/exp/mmap"
)

func main() {
	at, err := mmap.Open("/var/tmp/filecoin-parents/v28-sdr-parent-652bae61e906c0732e9eb95b1217cfa6afcce221ff92a8aedf62fa778fa765bc.cache")
	if err != nil {
		fmt.Println(err)
		return
	}
	// buf := make([]byte, 14*4)
	// at.ReadAt(buf, 63*14*4)
	// for i := 0; i < 14; i++ {
	// 	start := i * 4
	// 	end := start + 4
	// 	node := binary.LittleEndian.Uint32(buf[start:end])
	// 	fmt.Println(node)
	// }
	var pb byte = 9
	provider_id := make([]byte, 32)
	for i := 0; i < 32; i++ {
		provider_id[i] = pb
	}
	var tb byte = 1
	ticket := make([]byte, 32)
	for i := 0; i < 32; i++ {
		ticket[i] = tb
	}
	commd := []byte{252, 126, 146, 130, 150, 229, 22, 250, 173, 233, 134, 178, 143, 146, 212, 74, 79, 36, 185, 53, 72, 82, 35, 55, 106, 121, 144, 39, 188, 24, 248, 51}
	porepseed := []byte{99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99, 99}
	sector_id := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	fmt.Println(provider_id)
	fmt.Println(sector_id)
	fmt.Println(ticket)
	fmt.Println(commd)
	fmt.Println(porepseed)
	sha := sha256.New()
	sha.Write(provider_id)
	sha.Write(sector_id)
	sha.Write(ticket)
	sha.Write(commd)
	sha.Write(porepseed)
	rid := sha.Sum(nil)
	fmt.Println(rid)

	var layerIndex uint32 = 1
	var node uint64 = 0
	lbuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lbuf, layerIndex)
	nbuf := make([]byte, 8)
	binary.BigEndian.PutUint64(nbuf, node)
	sha2 := sha256.New()
	fmt.Println(lbuf)
	fmt.Println(nbuf)
	res := make([]byte, 20)
	for i := 0; i < 20; i++ {
		res[i] = 0
	}
	sha2.Write(rid)
	sha2.Write(lbuf)
	sha2.Write(nbuf)
	sha2.Write(res)
	n0 := sha2.Sum(nil)
	fmt.Println(n0)
	ni := &NodeInfo{}
	w := make(chan map[uint32][]byte)
	r := make(chan map[uint32][]byte)
	ni.NewNi(w, r, at, 1)
	node = 1
	binary.BigEndian.PutUint64(nbuf, node)
	go ni.Start(rid, lbuf, nbuf, uint32(node), 6)
	w <- map[uint32][]byte{uint32(0): n0}
	n1 := <-r
	fmt.Println(n1)
}

type NodeInfo struct {
	parents     [][]byte
	cacheMap    *mmap.ReaderAt
	receiveData chan map[uint32][]byte
	readyCnt    int
	nodeID      uint64
	setCache    []uint32
	writeDate   chan map[uint32][]byte
}

func (ni *NodeInfo) NewNi(r chan map[uint32][]byte, w chan map[uint32][]byte, reader *mmap.ReaderAt, nodeID uint64) {

	ni.parents = make([][]byte, 0, 14)
	ni.cacheMap = reader
	ni.receiveData = r
	ni.writeDate = w
	ni.nodeID = nodeID
	ni.setCache = make([]uint32, 0, 14)
	offset := nodeID * 14 * 4
	buf := make([]byte, 56)
	ni.cacheMap.ReadAt(buf, int64(offset))
	for i := 0; i < 14; i++ {
		start := i * 4
		end := start + 4
		node := binary.LittleEndian.Uint32(buf[start:end])
		ni.setCache = append(ni.setCache, node)
	}
}

func (ni *NodeInfo) Start(replacat_id []byte, lbuf []byte, nbuf []byte, node uint32, end int) {

	sha := sha256.New()
	sha.Write(replacat_id)
	sha.Write(lbuf)
	sha.Write(nbuf)
	res := make([]byte, 20)
	for i := 0; i < 20; i++ {
		res[i] = 0
	}
	sha.Write(res)
	r1 := sha.Sum(nil)
	for {
		if ni.readyCnt == end {
			break
		}
		data := <-ni.receiveData
		for k, v := range data {
			for _, n := range ni.setCache {
				if n == k {
					ni.parents = append(ni.parents, v)
					ni.readyCnt++
				}
			}
		}
		fmt.Println(ni.parents)
		rs := make([][]byte, 6)
		for i := 0; i < 6; i++ {
			sha.Reset()
			for _, p := range ni.parents {
				sha.Write(p)
			}
			rs[i] = sha.Sum(nil)
		}
		sha.Reset()
		sha.Write(ni.parents[0])
		r2 := sha.Sum(nil)
		sha.Reset()
		sha.Write(r1)
		for i := 0; i < 6; i++ {
			sha.Write(rs[i])
		}
		sha.Write(r2)
		hr := sha.Sum(nil)
		fmt.Println(hr)
		ni.writeDate <- map[uint32][]byte{node: hr}
	}
}
