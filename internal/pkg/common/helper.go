package common

import (
	"runtime/debug"
	"time"
)

func HelperService() {
	for {
		time.Sleep(time.Minute)

		debug.FreeOSMemory()
		RotateLogFiles()
	}
}
