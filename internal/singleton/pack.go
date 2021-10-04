package singleton

import "sync"

var packSingleton *PackSingleton

type PackSingleton struct {
	packs map[[32]byte]string
	sync.RWMutex
}

func initPackSingleton() {
	packSingleton = &PackSingleton{
		packs: make(map[[32]byte]string),
	}
}

func AddPack(packHash [32]byte, packName string) {
	packSingleton.packs[packHash] = packName
}

func GetPack(packHash [32]byte) string {
	packSingleton.RLock()

	value, ok := packSingleton.packs[packHash]
	if !ok {
		return ""
	}

	return value
}

func DeletePack(packHash [32]byte) {
	packSingleton.Lock()

	delete(packSingleton.packs, packHash)

	packSingleton.Unlock()
}

func IsExistPack(packHash [32]byte) bool {
	_, ok := packSingleton.packs[packHash]
	if !ok {
		return false
	}

	return true
}
