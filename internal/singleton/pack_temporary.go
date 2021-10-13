package singleton

import "sync"

var packTemporarySingleton *PackTemporarySingleton

type PackTemporarySingleton struct {
	packs map[[32]byte]int
	sync.RWMutex
}

func initPackTemporarySingleton() {
	packTemporarySingleton = &PackTemporarySingleton{
		packs: make(map[[32]byte]int),
	}
}

func IncTemporaryPack(packHash [32]byte) {
	packTemporarySingleton.Lock()
	packTemporarySingleton.packs[packHash]++
	packTemporarySingleton.Unlock()
}

func DegTemporaryPack(packHash [32]byte) {
	packTemporarySingleton.Lock()
	packTemporarySingleton.packs[packHash]--

	if packTemporarySingleton.packs[packHash] == 0 {
		delete(packTemporarySingleton.packs, packHash)
	}

	packTemporarySingleton.Unlock()
}

func IsExistemporaryPack(packHash [32]byte) bool {
	packTemporarySingleton.RLock()
	defer packTemporarySingleton.RUnlock()

	count, ok := packTemporarySingleton.packs[packHash]
	if !ok || count == 0 {
		return false
	}

	return true
}
