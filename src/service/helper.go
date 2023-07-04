package services

import (
	"clvisit/common"
	"runtime/debug"
	"time"
)

func HelperService() {
	for {
		time.Sleep(time.Minute)

		debug.FreeOSMemory()
		common.RotateLogFiles()
	}
}
