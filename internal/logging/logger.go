package logging

import (
	"io"
	"log"
	"os"
)

func New(path string) (*log.Logger, func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, func() {}, err
	}
	logger := log.New(io.MultiWriter(file), "", log.LstdFlags|log.LUTC)
	return logger, func() {
		_ = file.Close()
	}, nil
}
