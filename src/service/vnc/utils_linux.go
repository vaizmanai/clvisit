package vnc

import (
	"clvisit/common"
	log "github.com/sirupsen/logrus"
	"os/exec"
)

//apt install x11vnc
//apt install xtightvncviewer

func handleVNC(_ int) {
	//на текущий момент только одна комбинация поддерживается
	arrayVNC = []VNC{
		{
			FileServer:           "x11vnc",
			FileClient:           "vncviewer",
			CmdStartServer:       "x11vnc -auth guess -forever -loop -noxdamage -repeat -localhost -rfbport 32801 -shared -display :0",
			CmdStopServer:        "killall -9 x11vnc",
			CmdInstallServer:     "echo install server",
			CmdRemoveServer:      "echo remove server",
			CmdConfigServer:      "echo config server",
			CmdManageServer:      "echo manage server",
			CmdStartServerUser:   "x11vnc -auth guess -forever -loop -noxdamage -repeat -localhost -rfbport 32801 -shared -display :0",
			CmdStopServerUser:    "killall -9 x11vnc",
			CmdInstallServerUser: "echo install user server",
			CmdRemoveServerUser:  "echo remove user server",
			CmdConfigServerUser:  "echo config user server",
			CmdManageServerUser:  "echo remove user server",
			CmdStartClient:       "vncviewer %adr",
			CmdStopClient:        "killall -9 vncviewer",
			CmdInstallClient:     "echo install client",
			CmdRemoveClient:      "echo remove client",
			CmdConfigClient:      "echo config client",
			CmdManageClient:      "echo manage client",
			PortServerVNC:        "32801",
			Link:                 "",
			Name:                 "System VNC",
			Version:              "none",
			Description:          "",
		},
	}
	common.Options.ActiveVncId = 0

	if _, err := exec.LookPath(arrayVNC[common.Options.ActiveVncId].FileServer); err != nil {
		log.Fatalf("couldn't find vnc server: %s", err.Error())
	}

	if _, err := exec.LookPath(arrayVNC[common.Options.ActiveVncId].FileClient); err != nil {
		log.Fatalf("couldn't find vnc client: %s", err.Error())
	}
}

func GetActiveFolder() string {
	return common.GetParentFolder()
}
