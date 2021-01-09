package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
)

func httpServer() {
	http.Handle("/", http.RedirectHandler("/welcome", 301))
	http.HandleFunc("/welcome", handleWelcome)
	http.HandleFunc("/profile", handleProfile)
	http.HandleFunc("/options", handleOptions)
	http.HandleFunc("/contacts", handleContacts)

	http.HandleFunc("/api", handleAPI)
	http.HandleFunc("/resource/", handleResource)

	ln, err := net.Listen("tcp", options.HttpServerClientAdr+":"+options.HttpServerClientPort)
	if err != nil {
		logAdd(MessError, "httpServer не смог занять порт: "+fmt.Sprint(err))
		panic(err.Error())
	}

	myClient.WebServ = &ln
	_ = http.Serve(ln, nil)
	_ = ln.Close()
}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")

	if action == "connect" {
		logAdd(MessInfo, "Попытка подключения")

		pid := r.URL.Query().Get("pid")
		pass := r.URL.Query().Get("pass")
		if len(pid) > 0 && len(pass) > 0 {
			if sendMessage(TMessRequest, pid, getSHA256(pass+myClient.Salt)) {
				sendMessageToLocalCons(TMessLocalExec, parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].Name+"_"+arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator)+strings.Replace(arrayVnc[options.ActiveVncId].CmdStartClient, "%adr", options.LocalAdrVNC+":"+options.PortClientVNC, 1))
				return
			}
		} else {
			logAdd(MessError, "Не хватает полей")
		}
	} else if action == "configvnc" {
		logAdd(MessInfo, "Запускаем панель vnc сервера")

		if checkForAdmin() {
			sendMessageToLocalCons(TMessLocalExec, parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].Name+"_"+arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].CmdManageServer)
		} else {
			sendMessageToLocalCons(TMessLocalExec, parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].Name+"_"+arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].CmdManageServerUser)
		}
		return
	} else if action == "connectcont" {
		logAdd(MessInfo, "Попытка подключения из профиля")

		id := r.URL.Query().Get("id")
		if len(id) > 0 {
			processLocalConnectContact(createMessage(TMessLocalConnContact, id), nil)
			sendMessageToLocalCons(TMessLocalExec, parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].Name+"_"+arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator)+strings.Replace(arrayVnc[options.ActiveVncId].CmdStartClient, "%adr", options.LocalAdrVNC+":"+options.PortClientVNC, 1))
			return
		}
	} else {
		logAdd(MessError, "Нет такого действия")
	}

	http.Error(w, "bad request", http.StatusBadRequest)
}

func handleContacts(w http.ResponseWriter, r *http.Request) {
	resp, err := httpClient.Get(options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort + "/resource/client/contacts.html")
	if err == nil {
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			body = pageReplace(body, "$menu", addMenu())
			b, err := json.Marshal(myClient.Profile.Contacts)
			if err == nil {
				body = pageReplace(body, "$contacts", string(b))
			}

			w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			_, _ = w.Write(body)
			return
		}
		_ = resp.Body.Close()
	}

	http.Error(w, "error connection to server", http.StatusBadGateway)
}

func handleOptions(w http.ResponseWriter, r *http.Request) {
	resp, err := httpClient.Get(options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort + "/resource/client/options.html")
	if err == nil {
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			body = pageReplace(body, "$menu", addMenu())
			w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			_, _ = w.Write(body)
			return
		}
		_ = resp.Body.Close()
	}

	http.Error(w, "error connection to server", http.StatusBadGateway)
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	resp, err := httpClient.Get(options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort + "/resource/client/profile.html")
	if err == nil {
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			body = pageReplace(body, "$menu", addMenu())
			w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			_, _ = w.Write(body)
			return
		}
		_ = resp.Body.Close()
	}

	http.Error(w, "error connection to server", http.StatusBadGateway)
}

func handleWelcome(w http.ResponseWriter, r *http.Request) {
	resp, err := httpClient.Get(options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort + "/resource/client/welcome.html")
	if err == nil {
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			body = pageReplace(body, "$menu", addMenu())
			body = pageReplace(body, "$pid", myClient.Pid)
			body = pageReplace(body, "$pass", getPass())
			w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			_, _ = w.Write(body)
			return
		}
		_ = resp.Body.Close()
	}

	http.Error(w, "error connection to server", http.StatusBadGateway)
}

func handleResource(w http.ResponseWriter, r *http.Request) {
	resp, err := httpClient.Get(options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort + r.RequestURI)
	if err == nil {
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			_, _ = w.Write(body)
			return
		}
		_ = resp.Body.Close()
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func addMenu() string {
	out, err := json.Marshal(menus)
	if err == nil {
		return string(out)
	}

	return ""
}
