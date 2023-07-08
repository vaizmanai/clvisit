package main

import (
	"clvisit/common"
	"clvisit/service/web"
	"fmt"
	"github.com/webview/webview"
)

func openWindow() {
	w := webview.New(false)
	defer w.Destroy()
	w.SetTitle(fmt.Sprintf("%s %s", common.WhiteLabelName, common.RevisitVersion))
	w.SetSize(600, 470, webview.HintFixed)
	w.Navigate(fmt.Sprintf("http://%s:%s?token=%s", common.Options.HttpServerClientAdr, common.Options.HttpServerClientPort, web.GetToken()))
	w.Run()
}
