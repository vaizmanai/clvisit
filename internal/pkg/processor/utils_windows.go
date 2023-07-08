package processor

import (
	"clvisit/internal/pkg/common"
	"fmt"
	log "github.com/sirupsen/logrus"
	"syscall"
)

func startProcess(name string) bool {
	var sI syscall.StartupInfo
	sI.ShowWindow = 1
	var pI syscall.ProcessInformation
	argv, _ := syscall.UTF16PtrFromString(fmt.Sprintf("%s%s", common.GetParentFolder(), name))

	if err := syscall.CreateProcess(nil, argv, nil, nil, false, 0, nil, nil, &sI, &pI); err != nil {
		log.Errorf("не получилось перезапустить коммуникатор: %s", err.Error())
		return false
	}

	return true
}
