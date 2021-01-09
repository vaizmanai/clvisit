package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	parentPath, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	parentPath = parentPath + string(os.PathSeparator)
	_ = os.Chdir(parentPath)

	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)

	server := flag.String("server", "", "custom server")
	proxy := flag.String("proxy", "", "proxy server")
	password := flag.String("password", "", "static password")
	clean := flag.Bool("clean-all", false, "clean all options and settings")
	closeFlag := flag.Bool("closeFlag-all", false, "closeFlag all processes")
	reload := flag.Bool("reload", false, "reload communicator and UI")
	debug := flag.Bool("debug", false, "debug flag")
	flag.Parse()

	logAdd(MessInfo, "Запустился коммуникатор reVisit версии "+RevisitVersion)

	if !loadOptions() { //пробуем загрузить настройки, если они есть
		defaultOptions()
	}

	if *server != "" {
		options.MainServerAdr = *server
		options.DataServerAdr = *server
		options.HttpServerAdr = *server
	}
	if *proxy != "" {
		options.Proxy = *proxy
	}
	if *password != "" {
		options.Pass = *password
		flagPassword = true
	}
	if *debug {
		options.FDebug = true
	}
	if *clean {
		logAdd(MessInfo, "Пробуем удалить reVisit")
		loadListVNC()
		closeAllVNC()
		_, myName := filepath.Split(os.Args[0])
		closeProcess(myName)
		closeProcess("reVisit.exe")
		closeProcess("revisit.exe")
		_ = os.RemoveAll(parentPath + "vnc")
		_ = os.Remove("options.cfg")
		_ = os.Remove("vnc.list")
		return
	}
	if *closeFlag {
		logAdd(MessInfo, "Пробуем закрыть все процессы reVisit")
		loadListVNC()
		closeAllVNC()
		_, myName := filepath.Split(os.Args[0])
		closeProcess(myName)
		closeProcess("reVisit.exe")
		closeProcess("revisit.exe")
		return
	}
	if *reload {
		loadListVNC()
		closeAllVNC()
		_, myName := filepath.Split(os.Args[0])
		closeProcess(myName)
		options.ActiveVncId = -1
		reloadMe()
		return
	}

	go processVNC(options.ActiveVncId) //здесь запускаем VNC сервер
	go localDataServer()               //здесь ждем соединения от локального vnc клиента
	go httpServer()                    //там у нас располагаться должно много всего, но в будущем(заявки, доп настройки)
	go localServer()                   //здесь общаемся с UI мордой
	go mainClient()                    //здесь общаемся с главным сервером

	killSignal := <-interrupt
	switch killSignal {
	case os.Interrupt:
		logAdd(MessInfo, "got SIGINT...")
	case syscall.SIGTERM:
		logAdd(MessInfo, "got SIGTERM...")
	}

	terminateMe(true)
}
