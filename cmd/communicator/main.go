package main

import (
	"clvisit/internal/pkg/common"
	"clvisit/internal/pkg/processor"
	"clvisit/internal/pkg/vnc"
	"clvisit/internal/pkg/web"
	"flag"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	if err := common.LoadOptions(); err != nil {
		log.Warnf("не получилось открыть настройки: %s", err.Error())
		common.SetDefaultOptions()
	}

	flag.StringVar(&common.Options.ServerAddress, "server", common.Options.ServerAddress, "server address")
	flag.StringVar(&common.Options.Proxy, "proxy", common.Options.Proxy, "proxy address")
	pass := flag.String("password", "", "static password")
	clean := flag.Bool("clean-all", false, "clean all options and settings")
	closeFlag := flag.Bool("closeFlag-all", false, "closeFlag all processes")
	reload := flag.Bool("reload", false, "reload communicator and UI")
	flag.Parse()

	log.Infof("запустился коммуникатор %s версии %s", common.WhiteLabelName, common.RevisitVersion)

	if *window {
		openWindow() //web ui
		return
	}
	if *pass != "" {
		processor.SetPass(*pass)
	}
	if *clean {
		log.Infof("пробуем удалить %s", common.WhiteLabelName)
		vnc.CloseAllVNC()
		vnc.Clean()
		common.Clean()
		return
	}
	if *closeFlag {
		log.Infof("пробуем закрыть все процессы %s", common.WhiteLabelName)
		vnc.CloseAllVNC()
		common.Close()
		return
	}
	if *reload {
		vnc.CloseAllVNC()
		common.Reload()
		processor.ReloadMe()
		return
	}

	go func() {
		vnc.ProcessVNC(common.Options.ActiveVncId) //здесь запускаем VNC сервер
		processor.SendInfo()
	}()
	go processor.DataThread()  //здесь ждем соединения от локального vnc клиента
	go web.Thread(*standalone) //там у нас располагаться должно много всего, но в будущем(заявки, доп настройки)
	go processor.Thread()      //здесь общаемся с UI мордой
	go processor.MainClient()  //здесь общаемся с главным сервером
	go common.HelperService()
	if *standalone {
		log.Infof("запуск в режиме standalone")
		go openWindow() //web ui
	}

	killSignal := <-interrupt
	switch killSignal {
	case os.Interrupt:
		log.Infof("got SIGINT...")
	case syscall.SIGTERM:
		log.Infof("got SIGTERM...")
	}

	processor.TerminateMe(true)
}
