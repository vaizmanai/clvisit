package main

import (
	"clvisit/common"
	services "clvisit/service"
	"clvisit/service/processor"
	"clvisit/service/vnc"
	"clvisit/service/web"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	if err := common.LoadOptions(); err != nil {
		log.Errorf("не получилось открыть настройки: %s", err.Error())
	}

	flag.StringVar(&common.Options.ServerAddress, "server", common.Options.ServerAddress, "server address")
	flag.StringVar(&common.Options.Proxy, "proxy", common.Options.Proxy, "proxy address")
	pass := flag.String("password", "", "static password")
	clean := flag.Bool("clean-all", false, "clean all options and settings")
	closeFlag := flag.Bool("closeFlag-all", false, "closeFlag all processes")
	reload := flag.Bool("reload", false, "reload communicator and UI")
	flag.Parse()

	log.Infof("запустился коммуникатор %s версии %s", common.WhiteLabelName, common.RevisitVersion)

	if *pass != "" {
		processor.SetPass(*pass)
	}
	if *clean {
		log.Infof("пробуем удалить %s", common.WhiteLabelName)
		vnc.LoadListVNC()
		vnc.CloseAllVNC()
		_, myName := filepath.Split(os.Args[0])
		common.CloseProcess(myName)
		common.CloseProcess(common.WhiteLabelFileName)
		_ = os.RemoveAll(fmt.Sprintf("%s%s", common.GetParentFolder(), "vnc"))
		_ = os.Remove("options.cfg")
		_ = os.Remove("vnc.list")
		return
	}
	if *closeFlag {
		log.Infof("пробуем закрыть все процессы %s", common.WhiteLabelName)
		vnc.LoadListVNC()
		vnc.CloseAllVNC()
		_, myName := filepath.Split(os.Args[0])
		common.CloseProcess(myName)
		common.CloseProcess(common.WhiteLabelFileName)
		return
	}
	if *reload {
		vnc.LoadListVNC()
		vnc.CloseAllVNC()
		_, myName := filepath.Split(os.Args[0])
		common.CloseProcess(myName)
		common.Options.ActiveVncId = -1
		processor.ReloadMe()
		return
	}

	go func() {
		vnc.ProcessVNC(common.Options.ActiveVncId) //здесь запускаем VNC сервер
		processor.SendInfo()
	}()
	go processor.DataThread() //здесь ждем соединения от локального vnc клиента
	go web.Thread()           //там у нас располагаться должно много всего, но в будущем(заявки, доп настройки)
	go processor.Thread()     //здесь общаемся с UI мордой
	go processor.MainClient() //здесь общаемся с главным сервером
	go services.HelperService()

	killSignal := <-interrupt
	switch killSignal {
	case os.Interrupt:
		log.Infof("got SIGINT...")
	case syscall.SIGTERM:
		log.Infof("got SIGTERM...")
	}

	processor.TerminateMe(true)
}
