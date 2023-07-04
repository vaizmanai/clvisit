package web

import (
	"clvisit/common"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
)

func Thread() {
	myRouter := mux.NewRouter().StrictSlash(true)

	apiRouter := myRouter.PathPrefix("/api/v1").Subrouter()
	apiRouter.HandleFunc("/contacts", handleContacts).Methods(http.MethodGet)
	apiRouter.HandleFunc("/options", handleOptions).Methods(http.MethodGet)
	apiRouter.HandleFunc("/profile", handleProfile).Methods(http.MethodGet)
	apiRouter.HandleFunc("/", handleAPI).Methods(http.MethodGet)

	myRouter.PathPrefix("/").HandlerFunc(handleResource)

	var err error
	if err = http.ListenAndServe(fmt.Sprintf("%s:%s", common.Options.HttpServerClientAdr, common.Options.HttpServerClientPort), myRouter); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	//action := r.URL.Query().Get("action")

	//if action == "connect" {
	//	log.Infof("попытка подключения")
	//
	//	pid := r.URL.Query().Get("pid")
	//	pass := r.URL.Query().Get("pass")
	//	if len(pid) > 0 && len(pass) > 0 {
	//		if sendMessage(TMessRequest, pid, getSHA256(pass+myClient.Salt)) {
	//			sendMessageToLocalCons(TMessLocalExec, parentPath+VNCFolder+string(os.PathSeparator)+common.ArrayVnc[common.Options.ActiveVncId].Name+"_"+common.ArrayVnc[common.Options.ActiveVncId].Version+string(os.PathSeparator)+strings.Replace(common.ArrayVnc[common.Options.ActiveVncId].CmdStartClient, "%adr", common.Options.LocalAdrVNC+":"+common.Options.PortClientVNC, 1))
	//			return
	//		}
	//	} else {
	//		log.Errorf("не хватает полей")
	//	}
	//} else if action == "configvnc" {
	//	log.Infof("запускаем панель vnc сервера")
	//
	//	if checkForAdmin() {
	//		sendMessageToLocalCons(TMessLocalExec, common.ParentPath+VNCFolder+string(os.PathSeparator)+common.ArrayVnc[common.Options.ActiveVncId].Name+"_"+common.ArrayVnc[common.Options.ActiveVncId].Version+string(os.PathSeparator)+common.ArrayVnc[common.Options.ActiveVncId].CmdManageServer)
	//	} else {
	//		sendMessageToLocalCons(TMessLocalExec, common.ParentPath+VNCFolder+string(os.PathSeparator)+common.ArrayVnc[common.Options.ActiveVncId].Name+"_"+common.ArrayVnc[common.Options.ActiveVncId].Version+string(os.PathSeparator)+common.ArrayVnc[common.Options.ActiveVncId].CmdManageServerUser)
	//	}
	//	return
	//} else if action == "connectcont" {
	//	log.Infof("попытка подключения из профиля")
	//
	//	id := r.URL.Query().Get("id")
	//	if len(id) > 0 {
	//		processLocalConnectContact(createMessage(TMessLocalConnContact, id), nil)
	//		sendMessageToLocalCons(TMessLocalExec, parentPath+VNCFolder+string(os.PathSeparator)+common.ArrayVnc[common.Options.ActiveVncId].Name+"_"+common.ArrayVnc[common.Options.ActiveVncId].Version+string(os.PathSeparator)+strings.Replace(common.ArrayVnc[common.Options.ActiveVncId].CmdStartClient, "%adr", common.Options.LocalAdrVNC+":"+common.Options.PortClientVNC, 1))
	//		return
	//	}
	//} else {
	//	log.Errorf("нет такого действия")
	//}

	http.Error(w, "bad request", http.StatusBadRequest)
}

func handleContacts(w http.ResponseWriter, r *http.Request) {
	//resp, err := httpClient.Get(common.Options.HttpServerType + "://" + common.Options.HttpServerAdr + ":" + common.Options.HttpServerPort + "/resource/client/contacts.html")
	//if err == nil {
	//	body, err := ioutil.ReadAll(resp.Body)
	//	if err == nil {
	//		body = common.PageReplace(body, "$menu", addMenu())
	//		b, err := json.Marshal(myClient.Profile.Contacts)
	//		if err == nil {
	//			body = common.PageReplace(body, "$contacts", string(b))
	//		}
	//
	//		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	//		_, _ = w.Write(body)
	//		return
	//	}
	//	_ = resp.Body.Close()
	//}

	http.Error(w, "error connection to server", http.StatusBadGateway)
}

func handleOptions(w http.ResponseWriter, r *http.Request) {
	//resp, err := httpClient.Get(common.common.Options.HttpServerType + "://" + common.Options.HttpServerAdr + ":" + common.Options.HttpServerPort + "/resource/client/common.Options.html")
	//if err == nil {
	//	body, err := ioutil.ReadAll(resp.Body)
	//	if err == nil {
	//		body = common.PageReplace(body, "$menu", addMenu())
	//		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	//		_, _ = w.Write(body)
	//		return
	//	}
	//	_ = resp.Body.Close()
	//}

	http.Error(w, "error connection to server", http.StatusBadGateway)
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	//resp, err := httpClient.Get(common.Options.HttpServerType + "://" + common.Options.HttpServerAdr + ":" + common.Options.HttpServerPort + "/resource/client/profile.html")
	//if err == nil {
	//	body, err := ioutil.ReadAll(resp.Body)
	//	if err == nil {
	//		body = common.PageReplace(body, "$menu", addMenu())
	//		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	//		_, _ = w.Write(body)
	//		return
	//	}
	//	_ = resp.Body.Close()
	//}

	http.Error(w, "error connection to server", http.StatusBadGateway)
}

func handleResource(w http.ResponseWriter, r *http.Request) {
	//resp, err := httpClient.Get(common.Options.HttpServerType + "://" + common.Options.HttpServerAdr + ":" + common.Options.HttpServerPort + r.RequestURI)
	//if err == nil {
	//	body, err := ioutil.ReadAll(resp.Body)
	//	if err == nil {
	//		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	//		_, _ = w.Write(body)
	//		return
	//	}
	//	_ = resp.Body.Close()
	//}

	http.Error(w, "not found", http.StatusNotFound)
}
