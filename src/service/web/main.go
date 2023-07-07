package web

import (
	"clvisit/common"
	"clvisit/service/processor"
	"clvisit/service/vnc"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"time"
)

var (
	token = common.RandomString(common.LengthToken)
)

func Thread() {
	myRouter := mux.NewRouter().StrictSlash(true)
	myRouter.Use(handleCORS)

	apiRouter := myRouter.PathPrefix("/api/v1").Subrouter()
	apiRouter.Use(handleDigest)
	apiRouter.HandleFunc("/options", handleOptions).Methods(http.MethodGet)
	apiRouter.HandleFunc("/info", handleInfo).Methods(http.MethodGet)
	apiRouter.HandleFunc("/ping", handlePing).Methods(http.MethodGet)
	apiRouter.HandleFunc("/quit", handleQuit).Methods(http.MethodGet)
	apiRouter.HandleFunc("/connect/{pid}/{pass}", handleConnect).Methods(http.MethodGet) //todo pass to body

	myRouter.PathPrefix("/").HandlerFunc(handleResource)

	log.Debugf("starting web with token %s", token)
	if err := http.ListenAndServe(fmt.Sprintf("%s:%s", common.Options.HttpServerClientAdr, common.Options.HttpServerClientPort), myRouter); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}

func GetToken() string {
	return token
}

func handleConnect(w http.ResponseWriter, r *http.Request) {
	pid := mux.Vars(r)["pid"]
	pass := mux.Vars(r)["pass"]

	if !processor.Connect(pid, pass) {
		http.Error(w, "couldn't connect", http.StatusServiceUnavailable)
	} else {
		_, _ = w.Write([]byte("ok"))
	}
}

func handleInfo(w http.ResponseWriter, r *http.Request) {
	b, _ := json.Marshal(processor.GetClient())
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

func handleOptions(w http.ResponseWriter, r *http.Request) {
	b, _ := json.Marshal(common.Options)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

func handlePing(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok"))
}

func handleQuit(w http.ResponseWriter, _ *http.Request) {
	vnc.CloseAllVNC()
	common.Close()
	_, _ = w.Write([]byte("ok"))
	go func() {
		time.Sleep(time.Second)
		os.Exit(0)
	}()
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

func handleDigest(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != token {
			http.Error(w, "not auth", http.StatusUnauthorized)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func handleCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Max-Age", "6400")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")

		if r.Method == http.MethodOptions {
			_, _ = w.Write([]byte("ok"))
			return
		}

		h.ServeHTTP(w, r)
	})
}
