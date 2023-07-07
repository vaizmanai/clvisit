package processor

import (
	"clvisit/common"
	"clvisit/service/vnc"
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	StaticMessageEmpty         = 0
	StaticMessageNetworkError  = 1
	StaticMessageProxyError    = 2
	StaticMessageAuthError     = 3
	StaticMessageVncError      = 4
	StaticMessageTimeoutError  = 5
	StaticMessageAbsentError   = 6
	StaticMessageTypeError     = 7
	StaticMessageAuthFail      = 8
	StaticMessageRegFail       = 9
	StaticMessageRegMail       = 10
	StaticMessageRegSuccessful = 11
	StaticMessageLocalReq      = 12
	StaticMessageLocalConn     = 13
	StaticMessageLocalDisconn  = 14
	StaticMessageLocalError    = 15

	TMessDeauth         = 0
	TMessVersion        = 1
	TMessAuth           = 2
	TMessLogin          = 3
	TMessNotification   = 4
	TMessRequest        = 5
	TMessConnect        = 6
	TMessDisconnect     = 7
	TMessReg            = 8
	TMessContact        = 9
	TMessContacts       = 10
	TMessLogout         = 11
	TMessConnectContact = 12
	TMessStatuses       = 13
	TMessStatus         = 14
	TMessInfoContact    = 15
	TMessInfoAnswer     = 16
	TMessManage         = 17
	TMessPing           = 18
	TMessContactReverse = 19
	TMessServers        = 20
	TMessStandardAlert  = 21

	TMessLocalTest          = 20 //
	TMessLocalInfo          = 21 //идентификатор и пароль, версию и т.п.
	TMessLocalConnect       = 22 //запрос о подключении
	TMessLocalNotification  = 23 //сообщение всплывающее
	TMessLocalInfoClient    = 24 //побочная информация, путь до внц клиента
	TMessLocalTerminate     = 25 //завершение коммуникатора
	TMessLocalReg           = 26 //регистрация профиля
	TMessLocalLogin         = 27 //вход в профиль
	TMessLocalContact       = 28 //создание, редактирование, удаление
	TMessLocalContacts      = 29 //весь список контактов профиля
	TMessLocalLogout        = 30 //выход из профиля
	TMessLocalConnContact   = 31 //подключение к контакту из профиля
	TMessLocalMessage       = 32 //система сообщений
	TMessLocalExec          = 33 //запуск приложения, например, внс клиента
	TMessLocalStatus        = 34 //
	TMessLocalListVNC       = 35 //
	TMessLocalInfoContact   = 36 //
	TMessLocalInfoAnswer    = 37 //
	TMessLocalManage        = 38 //
	TMessLocalSave          = 39 //
	TMessLocalOptionClear   = 40 //
	TMessLocalReload        = 41 //
	TMessLocalLog           = 42 //
	TMessLocalMymanage      = 43 //
	TMessLocalMyinfo        = 44 //
	TMessLocalInfoHide      = 45 //
	TMessLocalContReverse   = 46 //
	TMessLocalOptionsUi     = 47 //
	TMessLocalProxy         = 48 //
	TMessLocalStandardAlert = 49
)

var (
	//считаем сколько у нас всего соединений
	connections Connections

	//ссылки на сокеты для локального сервера, который ждет локальный vnc viewer
	peerBuff1 *pConn
	peerBuff2 *pConn

	//ui который запросил трансляцию
	uiClient *net.Conn

	//храним здесь информацию о себе: идентификатор, пароль, сокеты и т.п.
	myClient Client

	//тут храним все локальные панели
	localConnections = list.New()

	//функции для обработки сообщений
	processing = []ProcessingMessage{
		{TMessDeauth, processDeauth},
		{TMessVersion, nil},
		{TMessAuth, processAuth},
		{TMessLogin, processLogin},
		{TMessNotification, processNotification},
		{TMessRequest, nil},
		{TMessConnect, processConnect},
		{TMessDisconnect, processDisconnect},
		{TMessReg, nil},
		{TMessContact, processContact},
		{TMessContacts, processContacts}, //10
		{TMessLogout, nil},
		{TMessConnectContact, nil},
		{TMessStatuses, nil},
		{TMessStatus, processStatus},
		{TMessInfoContact, processInfoContact},
		{TMessInfoAnswer, processInfoAnswer},
		{TMessManage, processManage},
		{TMessPing, processPing},
		{TMessContactReverse, nil},
		{TMessServers, processServers}, //20
		{TMessStandardAlert, processStandardAlert}}

	//функции для обработки локальных сообщений
	localProcessing = []ProcessingMessage{
		20: {TMessLocalTest, processLocalTest}, //20
		{TMessLocalInfo, processLocalInfo},
		{TMessLocalConnect, processLocalConnect},
		{TMessLocalNotification, nil},
		{TMessLocalInfoClient, processLocalInfoClient},
		{TMessLocalTerminate, processTerminate},
		{TMessLocalReg, processLocalReg},
		{TMessLocalLogin, processLocalLogin},
		{TMessLocalContact, processLocalContact},
		{TMessLocalContacts, processLocalContacts},
		{TMessLocalLogout, processLocalLogout}, //30
		{TMessLocalConnContact, processLocalConnectContact},
		{TMessLocalMessage, nil},
		{TMessLocalExec, nil},
		{TMessLocalStatus, nil},
		{TMessLocalListVNC, processLocalListVNC},
		{TMessLocalInfoContact, processLocalInfoContact},
		{TMessLocalInfoAnswer, nil},
		{TMessLocalManage, processLocalManage},
		{TMessLocalSave, processLocalSave},
		{TMessLocalOptionClear, processLocalOptionClear}, //40
		{TMessLocalReload, nil},
		{TMessLocalLog, nil},
		{TMessLocalMymanage, processLocalMyManage},
		{TMessLocalMyinfo, processLocalMyInfo},
		{TMessLocalInfoHide, nil},
		{TMessLocalContReverse, processLocalContactReverse},
		{TMessLocalOptionsUi, processLocalOptionsUI},
		{TMessLocalProxy, processLocalProxy},
		{TMessLocalStandardAlert, nil}}
)

type Client struct {
	Serial  string `json:"-"`
	Pid     string
	Pass    string //только для веб клиента
	Version string
	Salt    string `json:"-"`
	Token   string `json:"-"`
	Profile common.Profile

	Conn      *net.Conn     `json:"-"`
	LocalServ *net.Listener `json:"-"`
	DataServ  *net.Listener `json:"-"`
	WebServ   *net.Listener `json:"-"`
}

type Connections struct {
	mutex sync.Mutex
	count int
}

type pConn struct {
	Pointer *net.Conn
}

type ProcessingMessage struct {
	TMessage   int
	Processing func(message Message, conn *net.Conn, ctx context.Context)
}

type Message struct {
	TMessage int
	Messages []string
}

func GetClient() Client {
	myClient.Pass = GetPass()
	return myClient
}

func Connect(pid, pass string) bool {
	processLocalConnect(Message{Messages: []string{pid, pass}}, nil, context.Background())
	return true
}

func GetPass() string {
	for {
		if len(myClient.Pid) == 0 {
			//это не даст удаленной системе подключиться к нам
			return "***" + common.RandomString(2)
		}

		if len(common.Options.CleanPass) > 0 {
			return common.Options.CleanPass
		}

		if len(common.Options.Pass) > 0 {
			pass, success := common.DecXOR(common.Options.Pass, myClient.Pid)
			if success == true {
				common.Options.CleanPass = pass
				return pass
			}
		}

		log.Error("не получилось расшифровать пароль")

		if common.DefaultNumberPassword {
			common.Options.Pass = common.EncXOR(common.RandomNumber(common.LengthPass), myClient.Pid)
		} else {
			common.Options.Pass = common.EncXOR(common.RandomString(common.LengthPass), myClient.Pid)
		}

		log.Infof("сгенерировали новый пароль")
		common.SaveOptions()
		time.Sleep(time.Second)
	}
}

func SetPass(pass string) {
	common.Options.CleanPass = pass
	common.Options.Pass = common.EncXOR(pass, myClient.Pid)
}

func TerminateMe(term bool) {
	if localConnections.Len() > 1 && !term {
		log.Infof("отказываемся выходить так как несколько ui панелей")
		return
	}

	common.Flags.Terminated = true

	sendMessageToLocalCons(TMessLocalTerminate)

	log.Infof("выходим из коммуникатора")

	vnc.CloseVNC()
	os.Exit(0)
}

func UpdateMe() bool {
	log.Errorf("собираемся получить актуальную версию")

	if err := os.Remove(fmt.Sprintf("%sold_%s", common.GetParentFolder(), common.WhiteLabelFileName)); err != nil {
		log.Warnf("не получилось удалить старый временный файл: %s", err.Error())
	}
	if err := os.Remove(fmt.Sprintf("%sold_%s", common.GetParentFolder(), common.WhiteLabelCommunicatorFileName)); err != nil {
		log.Warnf("не получилось удалить старый временный файл: %s", err.Error())
	}

	resp, err := getHttpClient().Get(fmt.Sprintf("%s/resource/%s", common.GetHttpServerAddress(), common.WhiteLabelFileName))
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Errorf("не получилось получить с сервера новую версию: %s", err.Error())
		return false
	}

	f, err := os.OpenFile(fmt.Sprintf("%snew_%s", common.GetParentFolder(), common.WhiteLabelFileName), os.O_CREATE, 0)
	if err != nil {
		log.Errorf("не получилось создать временный файл: %s", err.Error())
		return false
	}

	buff, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("не получилось прочитать ответ с сервера: %s", err.Error())
		return false
	}
	_ = resp.Body.Close()

	_, err = f.Write(buff)
	if err != nil {
		log.Errorf("не получилось получить записать новую версию: %s", err.Error())
		return false
	}
	_ = f.Close()

	_, myName := filepath.Split(os.Args[0])
	err = os.Rename(fmt.Sprintf("%s%s", common.GetParentFolder(), myName), fmt.Sprintf("%sold_%s", common.GetParentFolder(), common.WhiteLabelCommunicatorFileName))
	if err != nil {
		log.Errorf("не получилось получить переименовать файл: %s", err.Error())
		return false
	}

	err = os.Rename(fmt.Sprintf("%s%s", common.GetParentFolder(), common.WhiteLabelFileName), fmt.Sprintf("%sold_%s", common.GetParentFolder(), common.WhiteLabelFileName))
	if err != nil {
		log.Errorf("не получилось получить переименовать файл: %s", err.Error())
		err = os.Rename(fmt.Sprintf("%sold_%s", common.GetParentFolder(), common.WhiteLabelCommunicatorFileName), fmt.Sprintf("%s%s", common.GetParentFolder(), myName))
		if err != nil {
			log.Errorf("не получилось получить откатить файл: %s", err.Error())
			return false
		}
		log.Infof("откатились назад")
		return false
	}

	_, err = exec.Command(fmt.Sprintf("%snew_%s", common.GetParentFolder(), common.WhiteLabelFileName), "-extract").Output()
	if err != nil {
		log.Errorf("не получилось распаковать коммуникатор: %s", err.Error())
		err = os.Rename(fmt.Sprintf("%sold_%s", common.GetParentFolder(), common.WhiteLabelCommunicatorFileName), fmt.Sprintf("%s%s", common.GetParentFolder(), myName))
		if err != nil {
			log.Errorf("не получилось получить откатить файл: %s", err.Error())
			return false
		}
		err = os.Rename(fmt.Sprintf("%sold_%s", common.GetParentFolder(), common.WhiteLabelFileName), fmt.Sprintf("%s%s", common.GetParentFolder(), common.WhiteLabelFileName))
		if err != nil {
			log.Errorf("не получилось получить откатить файл: %s", err.Error())
			return false
		}
		log.Infof("откатились назад")
		return false
	}

	err = os.Rename(fmt.Sprintf("%snew_%s", common.GetParentFolder(), common.WhiteLabelFileName), fmt.Sprintf("%s%s", common.GetParentFolder(), common.WhiteLabelFileName))
	if err != nil {
		log.Errorf("не получилось переименовать новый клиент, оставим старый: %s", err.Error())
		err = os.Rename(fmt.Sprintf("%sold_%s", common.GetParentFolder(), common.WhiteLabelFileName), fmt.Sprintf("%s%s", common.GetParentFolder(), common.WhiteLabelFileName))
		if err != nil {
			log.Errorf("не получилось получить откатить файл: %s", err.Error())
			return false
		}
		log.Infof("попробуем запуститься с новым коммуникатором")
	}

	ReloadMe()

	return true
}

func ReloadMe() bool {
	log.Infof("перезапускаемся")

	common.Flags.Reload = true
	sendMessageToLocalCons(TMessLocalReload)

	if myClient.Conn != nil {
		_ = (*myClient.Conn).Close()
	}
	if myClient.LocalServ != nil {
		_ = (*myClient.LocalServ).Close()
	}
	if myClient.DataServ != nil {
		_ = (*myClient.DataServ).Close()
	}
	if myClient.WebServ != nil {
		_ = (*myClient.WebServ).Close()
	}

	vnc.CloseVNC()
	common.CloseLogFile()

	log.Infof("запускаем новый экземпляр коммуникатора")
	_ = os.Chdir(common.GetParentFolder())
	_, myName := filepath.Split(os.Args[0])
	if !startProcess(myName) {
		common.Flags.Reload = false
		vnc.ProcessVNC(common.Options.ActiveVncId)
		common.ReOpenLogFile()
		return false
	}

	log.Infof("вышли...")
	os.Exit(0)
	return true
}

func SendInfo() {
	sendMessageToLocalCons(TMessLocalInfoClient, fmt.Sprintf("%s%s", vnc.GetActiveFolder(), strings.ReplaceAll(vnc.GetActiveVNC().CmdStartClient, "%adr", common.GetVNCAddress())))
}

func createMessage(TMessage int, Messages ...string) Message {
	var mes Message
	mes.TMessage = TMessage
	mes.Messages = Messages
	return mes
}

func sendMessageToSocket(conn *net.Conn, TMessage int, Messages ...string) bool {
	time.Sleep(time.Millisecond * common.WaitSendMess) //чисто на всякий случай, чтобы не заспамить

	if conn == nil {
		log.Debugf("нет подключения к сокету")
		return false
	}

	if Messages == nil {
		Messages = []string{}
	}

	var mes Message
	mes.TMessage = TMessage
	mes.Messages = Messages

	out, err := json.Marshal(mes)
	if err == nil {
		if err = (*conn).SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			return false
		}
		if _, err = (*conn).Write(out); err == nil {
			return true
		}
	}
	return false
}

func sendMessageToLocalCons(TMessage int, Messages ...string) {
	//log.Debugf("попытка отправить сообщение в UI панель: " + fmt.Sprint(TMessage) + " " + fmt.Sprint(Messages))
	if localConnections.Front() == nil {
		//log.Debugf("нет запущенных UI панелей")
	}
	for e := localConnections.Front(); e != nil; e = e.Next() {
		conn := e.Value.(*net.Conn)
		sendMessageToSocket(conn, TMessage, Messages...)
	}
}

func sendMessage(TMessage int, Messages ...string) bool {
	return sendMessageToSocket(myClient.Conn, TMessage, Messages...)
}
