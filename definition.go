package main

import (
	"container/list"
	"net"
	"os"
	"sync"
)

const (
	RevisitVersion = "1.00"

	DefaultMainServerName = "server.rvisit.net"
	DefaultDataServerName = "data.rvisit.net"
	DefaultHttpServerName = "web.rvisit.net"
	DefaultNumberPassword = false

	WaitCountRestartSrv = 10
	WaitCount           = 15
	WaitIdle            = 500
	WaitAfterConnect    = 250
	WaitSendMess        = 50
	WaitPing            = 10
	WaitRefreshAgents   = 180
	MaxEncPass          = 48
	LengthPass          = 6
	ProxyTimeout        = 30

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

	OptionsFile = "options.cfg"
	VNCListFile = "vnc.list"
	VNCFolder   = "vnc"
	LogName     = "log.txt"

	MessError  = 1
	MessInfo   = 2
	MessDetail = 3
	MessFull   = 4

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
	//наша домашняя папка
	parentPath string

	//считаем сколько у нас всего соединений
	connections Connections

	//ссылки на сокеты для локального сервера, который ждет локальный vnc viewer
	peerBuff1 *pConn
	peerBuff2 *pConn

	//ui который запросил трансляцию
	uiClient *net.Conn

	//файл для хранения лога
	logFile *os.File

	//конфиг тут храним
	options = Options{
		FDebug:  true,
		TypeLog: MessFull}

	//храним здесь информацию о себе: идентификатор, пароль, сокеты и т.п.
	myClient Client

	//тут храним все локальные панели
	localConnections = list.New()

	//меню веб интерфейса
	menus = []itemMenu{
		{"Настройки", "/options"},
		{"Контакты", "/contacts"},
		{"Профиль", "/profile"},
		{"reVisit", "/"}}

	//текстовая расшифровка сообщений для логов
	messLogText = []string{
		"BLANK",
		"ERROR",
		"INFO",
		"DETAIL",
		"FULL"}

	//текстовая расшифровка статических сообщений
	messStaticText = []string{
		"пустое сообщение",
		"ошибка сети",
		"ошибка прокси",
		"ошибка авторизации",
		"ошибка VNC",
		"ошибка времени ожидания",
		"отсутствует пир",
		"не правильный тип подключения",
		"ошибка авторизации",
		"учетная запись занята",
		"не удалось отправить письмо",
		"регистрация выполнена"}

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

	//список доступных vnc
	arrayVnc []VNC

	//собственно помечаем занят уже у нас этот процесс или нет
	flagReinstallVnc = false

	//
	flagReload = false

	//
	flagTerminated = false

	//
	flagPassword = false

	//список доступных агентов
	agents []Agent

	//флаг для обновления агентов
	chRefreshAgents chan bool
)

// структура для хранения конфигурируемых данных
type Options struct {
	//реквизиты сервера основного
	MainServerAdr  string
	MainServerPort string

	//реквизиты сервер для коммутации vnc
	DataServerAdr  string
	DataServerPort string

	//реквизиты веб сервера основного
	HttpServerAdr  string
	HttpServerPort string
	HttpServerType string

	//реквизиты сервера для общения с UI
	LocalServerAdr  string
	LocalServerPort string

	//реквизиты веб сервера для управления
	HttpServerClientAdr  string
	HttpServerClientPort string
	HttpServerClientType string

	//реквизиты локального VNC
	LocalAdrVNC   string
	PortClientVNC string

	//строка для прокси соедиения
	Proxy string

	//размер буфера для всех операций с сокетами
	SizeBuff int32

	//очевидно что флаг для отладки
	FDebug bool

	//максимальный уровень логов
	TypeLog int

	//сохраним пароль в конфиге
	Pass string

	//активная версия vnc
	ActiveVncId int

	//данные для автовхода профиля
	ProfileLogin string
	ProfilePass  string

	//опции для UI
	OptionsUI string
}

type Connections struct {
	mutex sync.Mutex
	count int
}

type itemMenu struct {
	Capt string
	Link string
}

type pConn struct {
	Pointer *net.Conn
}

type ProcessingMessage struct {
	TMessage   int
	Processing func(message Message, conn *net.Conn)
}

type Message struct {
	TMessage int
	Messages []string
}

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

type Client struct {
	Serial string
	Pid    string
	//Pass	string
	Version string
	Salt    string //используем эту соль для паролей
	Token   string //используем для авторизации в веб
	Profile Profile

	Conn      *net.Conn
	LocalServ *net.Listener
	DataServ  *net.Listener
	WebServ   *net.Listener

	Code string //for connection
}

type Profile struct {
	Email string
	Pass  string

	Contacts *Contact
}

type Contact struct {
	Id      int
	Caption string
	Type    string //node - контакт, fold - папка
	Pid     string
	//Digest string //приходит, но нам не интересны здесь
	//Salt	 string //приходит, но нам не интересны здесь

	Inner *Contact
	Next  *Contact
}

type Agent struct {
	Metric  int
	Address string
}
