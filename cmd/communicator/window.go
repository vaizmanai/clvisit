//go:build webui

package main

import (
	"clvisit/internal/pkg/common"
	"clvisit/internal/pkg/web"
	"flag"
	"fmt"
	"github.com/webview/webview"
)

var (
	standalone = flag.Bool("standalone", true, "show web ui window")    //При закрытии окна - должен закрыться коммуникатор
	window     = flag.Bool("window", false, "start only web ui window") //Используем для подключения к запущенному сервису
)

func openWindow() {
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle(fmt.Sprintf("%s %s", common.WhiteLabelName, common.RevisitVersion))
	w.SetSize(600, 470, webview.HintFixed)
	w.Navigate(fmt.Sprintf("http://%s:%s?token=%s", common.Options.HttpServerClientAdr, common.Options.HttpServerClientPort, web.GetToken()))
	w.Run()
}
