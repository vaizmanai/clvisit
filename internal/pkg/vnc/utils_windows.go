package vnc

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/clvisit/internal/pkg/common"
	"io"
	"os"
	"time"
)

func saveListVNC() bool {
	log.Infof("пробуем сохранить список VNC")

	f, err := os.Create(fmt.Sprintf("%s%s", common.GetParentFolder(), fileNameList))
	if err != nil {
		log.Errorf("не получилось сохранить список VNC: %s", err.Error())
		return false
	}
	defer f.Close()

	buff, err := json.MarshalIndent(arrayVNC, "", "\t")
	if err != nil {
		log.Errorf("не получилось сохранить список VNC: %s", err.Error())
		return false
	}

	if _, err = f.Write(buff); err != nil {
		log.Errorf("не получилось сохранить список VNC: %s", err.Error())
		return false
	}

	return true
}

func loadListVNC() bool {
	log.Infof("пробуем загрузить список VNC")
	buff, err := os.ReadFile(fmt.Sprintf("%s%s", common.GetParentFolder(), fileNameList))
	if err != nil {
		log.Errorf("не получилось открыть список VNC: %s", err.Error())
		common.Options.ActiveVncId = -1
		return false
	}

	if err = json.Unmarshal(buff, &arrayVNC); err != nil {
		log.Errorf("не получилось открыть список VNC: %s", err.Error())
		return false
	}

	if len(arrayVNC) > 0 && common.Options.ActiveVncId < 0 {
		common.Options.ActiveVncId = 0
	}
	log.Infof("список VNC загружен")
	return true
}

func getListVNC() bool {
	log.Debugf("получим список VNC")

	resp, err := getHttpClient().Get(fmt.Sprintf("%s/api?make=listvnc", common.GetHttpServerAddress()))
	if err != nil {
		log.Errorf("не получилось получить с сервера VNC: %s", err.Error())
		return false
	}

	buff, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("не получилось прочитать ответ с сервера VNC: %s", err.Error())
		return false
	}
	_ = resp.Body.Close()

	if err = json.Unmarshal(buff, &arrayVNC); err != nil {
		log.Errorf("не получилось получить с сервера VNC: %s", err.Error())
		return false
	}

	if len(arrayVNC) > 0 && common.Options.ActiveVncId < 0 {
		common.Options.ActiveVncId = 0
	}

	log.Debugf("получили список VNC с сервера")
	return true
}

func handleVNC(i int) {
	for {
		//пробуем запустить vnc когда у нас уже есть подключение до сервера, если что можем загрузить новый vnc с сервера
		if !loadListVNC() || common.Options.ActiveVncId != i || common.Options.ActiveVncId > len(arrayVNC)-1 {
			if getListVNC() {
				if common.Options.ActiveVncId > len(arrayVNC)-1 {
					log.Errorf("нет такого VNC в списке")
					i = 0
				}

				if getAndExtractVNC(i) {
					log.Infof("обновили VNC")
					common.SaveOptions()
					saveListVNC()
					break
				}
				time.Sleep(time.Second)
				continue
			}
			time.Sleep(time.Second)
			continue
		}
		break
	}
}

func GetActiveFolder() string {
	return getVNCFolder(common.Options.ActiveVncId)
}
