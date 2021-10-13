package singleton

import (
	"bytes"
	"crypto/sha256"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"sync"
)

const siqFormat = ".siq.zip"

var packSingleton *PackSingleton

type PackSingleton struct {
	sync.RWMutex
	packs map[[32]byte]string
}

func initPackSingleton() {
	packSingleton = &PackSingleton{
		packs: make(map[[32]byte]string),
	}
}

func GetPacks() map[[32]byte]string {
	packSingleton.RLock()

	packs := packSingleton.packs

	packSingleton.RUnlock()

	return packs
}

func InitPacks(siqArchivesPath string) error {
	files, err := ioutil.ReadDir(siqArchivesPath)
	if err != nil {
		return err
	}

	for _, f := range files {
		if strings.Contains(f.Name(), siqFormat) {
			var fileReader *os.File

			fileReader, err = os.Open(siqArchivesPath + "/" + f.Name())
			if err != nil {
				return err
			}

			buf := bytes.NewBuffer(nil)
			if _, err = io.Copy(buf, fileReader); err != nil {
				return err
			}

			hash := sha256.Sum256(buf.Bytes())

			AddPack(hash, f.Name())
		}
	}

	return nil
}

func AddPack(packHash [32]byte, packName string) {
	packSingleton.Lock()

	packSingleton.packs[packHash] = packName

	packSingleton.Unlock()
}

func GetPack(packHash [32]byte) string {
	packSingleton.RLock()

	value, ok := packSingleton.packs[packHash]
	if !ok {
		return ""
	}

	packSingleton.RUnlock()

	return value
}

func DeletePack(packHash [32]byte) {
	packSingleton.Lock()

	delete(packSingleton.packs, packHash)

	packSingleton.Unlock()
}

func IsExistPack(packHash [32]byte) (ok bool) {
	packSingleton.RLock()

	_, ok = packSingleton.packs[packHash]

	packSingleton.RUnlock()

	return
}
