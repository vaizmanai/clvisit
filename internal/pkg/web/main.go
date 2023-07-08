package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/clvisit/internal/pkg/common"
	"github.com/vaizmanai/clvisit/internal/pkg/processor"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

var (
	//go:embed resources
	resources embed.FS
	token     = common.RandomString(common.LengthToken)
	lastPing  time.Time
	mutex     sync.Mutex
)

const (
	staticFolder = "resources"
)

func Thread(standalone bool) {
	if standalone {
		go func() {
			for {
				time.Sleep(time.Second * 2)
				mutex.Lock()
				if !lastPing.IsZero() {
					if time.Now().Sub(lastPing).Seconds() > 3 {
						mutex.Unlock()
						processor.TerminateMe(true)
					}
				}
				mutex.Unlock()
			}
		}()
	}

	myRouter := mux.NewRouter().StrictSlash(true)
	myRouter.Use(handleCORS)

	apiRouter := myRouter.PathPrefix("/api/v1").Subrouter()
	apiRouter.Use(handleDigest(standalone))
	apiRouter.HandleFunc("/options", handleOptions).Methods(http.MethodGet)
	apiRouter.HandleFunc("/info", handleInfo).Methods(http.MethodGet)
	apiRouter.HandleFunc("/alert", handleAlert(standalone)).Methods(http.MethodGet)
	apiRouter.HandleFunc("/connect/{pid}/{pass}", handleConnect).Methods(http.MethodGet) //todo pass to body

	subFileSystem, _ := fs.Sub(resources, staticFolder)
	staticServer := http.FileServer(http.FS(subFileSystem))
	myRouter.PathPrefix("/").HandlerFunc(staticServer.ServeHTTP)

	log.Debugf("starting web with token %s", token)
	if err := http.ListenAndServe(fmt.Sprintf("%s:%s", common.Options.HttpServerClientAdr, common.Options.HttpServerClientPort), myRouter); err != nil {
		panic(err)
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

func handleAlert(standalone bool) func(w http.ResponseWriter, _ *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		if standalone {
			mutex.Lock()
			lastPing = time.Now()
			mutex.Unlock()
		}
		_, _ = w.Write([]byte(common.GetAlert()))
	}
}

func handleDigest(standalone bool) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if standalone && r.URL.Query().Get("token") != token {
				http.Error(w, "not auth", http.StatusUnauthorized)
				return
			}

			h.ServeHTTP(w, r)
		})
	}
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
