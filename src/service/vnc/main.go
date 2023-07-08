package vnc

import (
	"clvisit/common"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

const (
	fileNameList       = "vnc.list"
	fileNameTmp        = "tmp.zip"
	folderNamePackages = "vnc"
)

var (
	httpClient *http.Client

	//список доступных vnc
	arrayVNC []VNC
)

type VNC struct {
	FileServer string
	FileClient string

	CmdStartServer   string
	CmdStopServer    string
	CmdInstallServer string
	CmdRemoveServer  string
	CmdConfigServer  string
	CmdManageServer  string

	CmdStartServerUser   string
	CmdStopServerUser    string
	CmdInstallServerUser string
	CmdRemoveServerUser  string
	CmdConfigServerUser  string
	CmdManageServerUser  string

	CmdStartClient   string
	CmdStopClient    string
	CmdInstallClient string
	CmdRemoveClient  string
	CmdConfigClient  string
	CmdManageClient  string

	PortServerVNC string
	Link          string
	Name          string
	Version       string
	Description   string
}

func (vnc VNC) String() string {
	return fmt.Sprintf("%s %s", vnc.Name, vnc.Version)
}

func getHttpClient() *http.Client {
	if httpClient == nil {
		httpClient = common.GetHttpClient()
	}
	return httpClient
}

func startVNC() {
	if len(arrayVNC) == 0 || common.Options.ActiveVncId == -1 {
		log.Infof("VNC серверы отсутствуют")
		return
	}

	log.Infof("готовим VNC сервер для запуска")
	if !common.CheckForAdmin() {
		log.Infof("у нас нет прав Администратора, запускаем обычную версию VNC сервера")

		if configVNCServerUser() {
			if installVNCServerUser() {
				go runVNCServerUser()
			}
		}
	} else {
		log.Infof("права Администратора доступны, запускаем службу для VNC сервера")

		if configVNCServer() {
			if installVNCServer() {
				go runVNCServer()
			}
		}
	}
}

func configVNCServer() bool {
	log.Infof("импортируем настройки сервера")

	if !actVNC(GetActiveVNC().CmdConfigServer) {
		log.Errorf("не получилось импортировать настройки")
		return true //todo change to false
	}

	log.Infof("импортировали настройки сервера")
	return true
}

func installVNCServer() bool {
	log.Infof("устанавливаем VNC сервер")

	if !actVNC(GetActiveVNC().CmdInstallServer) {
		log.Errorf("не получилось установить VNC сервер")
		return true //todo change to false
	}
	log.Infof("установили VNC сервер")
	return true
}

func runVNCServer() bool {
	log.Infof("запускаем VNC сервер")

	if _, pid := common.CheckExistsProcess(GetActiveVNC().FileServer); pid != 0 {
		log.Infof("VNC сервер уже запущен")
		return true
	}

	if !actVNC(GetActiveVNC().CmdStartServer) {
		log.Errorf("не получилось запустить VNC сервер")
		return false
	}
	log.Infof("запустился VNC сервер")
	return true
}

func stopVNCServer() bool {
	log.Infof("останавливаем VNC сервер")

	if !actVNC(GetActiveVNC().CmdStopServer) {
		log.Errorf("не получилось остановить VNC сервер")
		return true //todo change to false
	}
	log.Infof("остановили VNC сервер")
	return true
}

func uninstallVNCServer() bool {
	log.Infof("удаляем VNC сервер")

	if !actVNC(GetActiveVNC().CmdRemoveServer) {
		log.Errorf("не получилось удалить VNC сервер")
		return false
	}
	log.Infof("удалили VNC сервер")
	return true
}

func configVNCServerUser() bool {
	log.Infof("импортируем настройки сервера")

	if !actVNC(GetActiveVNC().CmdConfigServerUser) {
		log.Errorf("не получилось импортировать настройки")
		return true //todo change to false
	}

	log.Infof("импортировали настройки сервера")
	return true
}

func installVNCServerUser() bool {
	log.Infof("устанавливаем VNC сервер")

	if !actVNC(GetActiveVNC().CmdInstallServerUser) {
		log.Errorf("не получилось установить VNC сервер")
		return true //todo change to false
	}
	log.Infof("установили VNC сервер")
	return true
}

func runVNCServerUser() bool {
	log.Infof("запускаем VNC сервер")

	_, pid := common.CheckExistsProcess(GetActiveVNC().FileServer)
	if pid != 0 {
		log.Infof("VNC сервер уже запущен")
		return true
	}

	if !actVNC(GetActiveVNC().CmdStartServerUser) {
		log.Errorf("не получилось запустить VNC сервер")
		return false
	}
	log.Infof("завершился VNC сервер")
	return true
}

func stopVNCServerUser() bool {
	log.Infof("останавливаем VNC сервер")

	if !actVNC(GetActiveVNC().CmdStopServerUser) {
		log.Errorf("не получилось остановить VNC сервер")
		return true //todo change to false
	}
	log.Infof("остановили VNC сервер")
	return true
}

func uninstallVNCServerUser() bool {
	log.Infof("удаляем VNC сервер")

	if !actVNC(GetActiveVNC().CmdRemoveServerUser) {
		log.Errorf("не получилось удалить VNC сервер")
		return false
	}
	log.Infof("удалили VNC сервер")
	return true
}

func actVNC(cmd string) bool {
	if len(cmd) == 0 {
		log.Errorf("нет команды для этого")
		return true
	}

	log.Debugf("execute '%s'", cmd)
	args := strings.Split(cmd, " ")
	if len(args) == 0 {
		return false
	}

	_ = os.Chdir(GetActiveFolder())
	defer func() {
		_ = os.Chdir(common.GetParentFolder())
	}()

	if fs, err := os.Stat(args[0]); err == nil && fs.Size() > 0 {
		if !strings.Contains(cmd, string(os.PathSeparator)) {
			args[0] = fmt.Sprintf("%s%s", GetActiveFolder(), args[0])
		}
	} else {
		if path, err := exec.LookPath(args[0]); err != nil {
			log.Errorf("%s", err.Error())
			return false
		} else {
			args[0] = path
		}
	}

	out, err := exec.Command(args[0], args[1:]...).Output()
	if len(out) > 0 {
		log.Infof("%s", strings.TrimSpace(string(out)))
	}
	if err != nil {
		log.Errorf("%s", err.Error())
		return false
	}

	return true
}

func getAndExtractVNC(i int) bool {
	if i > len(arrayVNC) {
		log.Errorf("нет у нас такого VNC в списке (%d/%d)", i, len(arrayVNC))
		return false
	}

	if i < 0 {
		i = 0
	}

	log.Errorf("собираемся получить и включить %s", arrayVNC[i].String())

	resp, err := getHttpClient().Get(fmt.Sprintf("%s/%s", common.GetHttpServerAddress(), arrayVNC[i].Link))
	if err != nil {
		log.Errorf("не получилось получить с сервера VNC: %s", err.Error())
		return false
	}

	_ = os.MkdirAll(getVNCFolder(i), os.ModePerm)
	f, err := os.OpenFile(fmt.Sprintf("%s%s", getVNCFolder(i), fileNameTmp), os.O_CREATE, os.ModePerm)
	if err != nil {
		log.Errorf("не получилось получить с сервера VNC: %s", err.Error())
		return false
	}
	defer f.Close()

	buff, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("не получилось прочитать ответ с сервера VNC: %s", err.Error())
		return false
	}
	_ = resp.Body.Close()

	if _, err = f.Write(buff); err != nil {
		log.Errorf("не получилось записать ответ с сервера VNC: %s", err.Error())
		return false
	}
	_ = f.Close()

	log.Infof("получили архив с %s", arrayVNC[i].String())

	if common.ExtractZip(fmt.Sprintf("%s%s", getVNCFolder(i), fileNameTmp), getVNCFolder(i)) {
		common.Options.ActiveVncId = i
		return true
	}

	return false
}

func emergencyExitVNC(i int) {
	if i < 0 || i >= len(arrayVNC) {
		log.Errorf("нет такого VNC в списке")
		return
	}

	common.CloseProcess(arrayVNC[i].FileServer)

	common.CloseProcess(arrayVNC[i].FileClient)
}

func getVNCFolder(i int) string {
	if i < 0 || i >= len(arrayVNC) {
		return common.GetParentFolder()
	}

	return fmt.Sprintf("%s%s%s%s_%s%s",
		common.GetParentFolder(),
		folderNamePackages,
		string(os.PathSeparator),
		arrayVNC[i].Name,
		arrayVNC[i].Version,
		string(os.PathSeparator),
	)
}

func ProcessVNC(i int) {
	if common.Flags.ReInstallVNC {
		log.Errorf("уже кто-то запустил процесс переустановки VNC")
		return
	}

	common.Flags.ReInstallVNC = true //надеемся, что не будет у нас одновременных обращений

	//закроем текущую версию
	CloseVNC()

	//заполняем список vnc
	handleVNC(i)

	//надо бы добавить проверку установлен уже или нет сервер
	startVNC()
	common.Flags.ReInstallVNC = false
}

func CloseVNC() {
	if len(arrayVNC) == 0 || common.Options.ActiveVncId == -1 {
		log.Infof("VNC серверы отсутствуют")
		return
	}

	if !common.CheckForAdmin() {
		log.Infof("у нас нет прав Администратора")

		if stopVNCServerUser() {
			uninstallVNCServerUser()
		}
	} else {
		log.Infof("права Администратора не доступны")

		if stopVNCServer() {
			uninstallVNCServer()
		}
	}

	//контрольный вариант завершения процессов vnc сервера
	emergencyExitVNC(common.Options.ActiveVncId)
}

func CloseAllVNC() {
	for i, _ := range arrayVNC {
		log.Infof("пробуем закрыть %s", arrayVNC[i].String())
		emergencyExitVNC(i)
	}
}

func GetActiveVNC() VNC {
	if len(arrayVNC) == 0 || common.Options.ActiveVncId >= len(arrayVNC) {
		return VNC{}
	}
	return arrayVNC[common.Options.ActiveVncId]
}

func Clean() {
	_ = os.RemoveAll(fmt.Sprintf("%s%s", common.GetParentFolder(), folderNamePackages))
	_ = os.Remove(fileNameList)
}

func RunVNCClient() bool {
	log.Infof("запускаем VNC клиент")

	if !actVNC(strings.ReplaceAll(GetActiveVNC().CmdStartClient, "%adr", common.GetVNCAddress())) {
		log.Errorf("не получилось запустить VNC клиент")
		return false
	}
	log.Infof("завершился VNC клиент")
	return true
}
