package file

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func CopyFile(destinationPath string, sourcePath string, destinationPerm os.FileMode) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(destinationPath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(destinationPath), FileToDirPerm(destinationPerm)); err != nil {
			return err
		}
		destination, err = os.Create(destinationPath)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}

	err = os.Chmod(destinationPath, destinationPerm)
	if err != nil {
		return err
	}

	return nil
}

func FileToDirPerm(filePerm os.FileMode) os.FileMode {
	return (filePerm & 0666) | ((filePerm >> 2) & 0111)
}
