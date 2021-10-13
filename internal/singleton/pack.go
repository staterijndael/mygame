package singleton

import "sync"

var packSingleton *PackSingleton

type PackSingleton struct {
	packs map[[32]byte]struct{}
	sync.RWMutex
}

func initPackSingleton() {
	packSingleton = &PackSingleton{
		packs: make(map[[32]byte]struct{}),
	}
}

func AddPack(packHash [32]byte) {
	packSingleton.Lock()
	packSingleton.packs[packHash] = struct{}{}
	packSingleton.Unlock()
}

func IsExistPack(packHash [32]byte) bool {
	packSingleton.RLock()
	defer packSingleton.RUnlock()

	_, ok := packSingleton.packs[packHash]
	if !ok {
		return false
	}

	return true
}
