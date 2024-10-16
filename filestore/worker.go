package filestore

import (
	"fmt"
	"os"
	"strconv"
	"sync"
)

var Semaphore chan struct{}
var Once sync.Once
var PoolSize int

func SetupImageConverterWorker() {
	Once.Do(initTheStuff)
}

func initTheStuff() {
	poolSizeStr := os.Getenv("IMAGE_WORKERS")
	if poolSizeStr == "" {
		poolSizeStr = "1"
	}
	var err error
	PoolSize, err = strconv.Atoi(poolSizeStr)
	if err != nil {
		panic(fmt.Errorf("Your IMAGE_WORKERS is set incorrectly. Make sure it defines an integer value!: %v", err))
	} else if PoolSize < 1 {
		panic("Your IMAGE_WORKERS is set incorrectly. You need at least one worker!")
	}

	Semaphore = make(chan struct{}, PoolSize)
}
