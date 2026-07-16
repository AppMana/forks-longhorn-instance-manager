package util

import "os"

func GetFileChangeTime(fileName string) (string, error) {
	fileInfo, err := os.Stat(fileName)
	if err != nil {
		return "", err
	}
	return fileInfo.ModTime().String(), nil
}
