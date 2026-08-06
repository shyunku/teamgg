package service

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"team.gg-server/util"
)

func LoadDDragonCentralImageFile(relativePath string) ([]byte, error) {
	path := fmt.Sprintf("%s/datafiles/data_dragon/%s/img%s",
		util.GetProjectRootDirectory(), LocalDataDragonVersion, relativePath)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func LoadDDragonImageFile(relativePath string) ([]byte, error) {
	path := fmt.Sprintf("%s/datafiles/data_dragon/%s/%s/img%s",
		util.GetProjectRootDirectory(), LocalDataDragonVersion, LocalDataDragonVersion, relativePath)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func LoadDDragonKorFile(dest any, relativePath string) error {
	path := fmt.Sprintf("%s/datafiles/data_dragon/%s/%s/data/ko_KR%s",
		util.GetProjectRootDirectory(), LocalDataDragonVersion, LocalDataDragonVersion, relativePath)
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	return json.Unmarshal(bytes, dest)
}
