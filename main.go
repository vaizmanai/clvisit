package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)



func main() {
	parentPath, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	parentPath = parentPath + string(os.PathSeparator)
	os.Chdir(parentPath)

	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)

	logAdd(MESS_INFO, "Запустился коммуникатор reVisit версии " + REVISIT_VERSION)

	if !loadOptions() { //пробуем загрузить настройки, если они есть
		defaultOptions()
	}

	for i, arg := range os.Args {
		if strings.Contains(arg, "server") {
			if len(os.Args) > i {
				options.MainServerAdr = os.Args[i+1]
				options.DataServerAdr = os.Args[i+1]
				options.HttpServerAdr = os.Args[i+1]
			}
		}
		if strings.Contains(arg, "proxy") {
			if len(os.Args) > i {
				options.Proxy = os.Args[i+1]
			}
		}
		if strings.Contains(arg, "password") {
			if len(os.Args) > i {
				options.Pass = os.Args[i+1]
				flagPassword = true
			}
		}
		if strings.Contains(arg, "clean-all") {
			logAdd(MESS_INFO, "Пробуем удалить reVisit")
			loadListVNC()
			closeAllVNC()
			_, myName := filepath.Split(os.Args[0])
			closeProcess(myName)
			closeProcess("reVisit.exe")
			closeProcess("revisit.exe")
			os.RemoveAll(parentPath + "vnc")
			os.Remove("options.cfg")
			os.Remove("vnc.list")
			return
		}
		if strings.Contains(arg, "close-all") {
			logAdd(MESS_INFO, "Пробуем закрыть все процессы reVisit")
			loadListVNC()
			closeAllVNC()
			_, myName := filepath.Split(os.Args[0])
			closeProcess(myName)
			closeProcess("reVisit.exe")
			closeProcess("revisit.exe")
			return
		}
		if strings.Contains(arg, "reload") {
			loadListVNC()
			closeAllVNC()
			_, myName := filepath.Split(os.Args[0])
			closeProcess(myName)
			options.ActiveVncId = -1
			reloadMe()
			return
		}
	}

	go processVNC(options.ActiveVncId) //здесь запускаем VNC сервер
	go localDataServer() //здесь ждем соединения от локального vnc клиента
	go httpServer() //там у нас располагаться должно много всего, но в будущем(заявки, доп настройки)
	go localServer() //здесь общаемся с UI мордой
	go mainClient() //здесь общаемся с главным сервером

	fmt.Scanln()
	terminateMe(true)
}
