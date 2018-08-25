package main

import (
	"net"
	"container/list"
	"os"
	"sync"
)

const(
	REVISIT_VERSION = "0.96"

	DEFAULT_MAIN_SERVER_NAME = "server.rvisit.net"
	DEFAULT_DATA_SERVER_NAME = "data.rvisit.net"
	DEFAULT_HTTP_SERVER_NAME = "web.rvisit.net"
	DEFAULT_NUMBER_PASSWORD = false

	//DEFAULT_MAIN_SERVER_NAME = "178.46.164.51"
	//DEFAULT_DATA_SERVER_NAME = "178.46.164.51"
	//DEFAULT_HTTP_SERVER_NAME = "178.46.164.51"
	//DEFAULT_NUMBER_PASSWORD = true

	WAIT_COUNT_RESTART_SRV = 10
	WAIT_COUNT             = 15
	WAIT_IDLE              = 500
	WAIT_AFTER_CONNECT     = 250
	WAIT_SEND_MESS         = 50
	WAIT_HELPER_CYCLE      = 5
	WAIT_PING              = 10
	MAX_ENC_PASS           = 48
	LENGTH_PASS            = 6

	OPTIONS_FILE = "options.cfg"
	VNCLIST_FILE = "vnc.list"
	VNC_FOLDER = "vnc"
	LOG_NAME = "log.txt"

	MESS_ERROR = 1
	MESS_INFO = 2
	MESS_DETAIL = 3
	MESS_FULL = 4

	TMESS_DEAUTH = 0
	TMESS_VERSION = 1
	TMESS_AUTH = 2
	TMESS_LOGIN = 3
	TMESS_NOTIFICATION = 4
	TMESS_REQUEST = 5
	TMESS_CONNECT = 6
	TMESS_DISCONNECT = 7
	TMESS_REG = 8
	TMESS_CONTACT = 9
	TMESS_CONTACTS = 10
	TMESS_LOGOUT = 11
	TMESS_CONNECT_CONTACT = 12
	TMESS_STATUSES = 13
	TMESS_STATUS = 14
	TMESS_INFO_CONTACT = 15
	TMESS_INFO_ANSWER = 16
	TMESS_MANAGE = 17
	TMESS_PING = 18
	TMESS_CONTACT_REVERSE = 19

	TMESS_LOCAL_TEST = 20 			 //
	TMESS_LOCAL_INFO = 21 			 //идентификатор и пароль, версию и т.п.
	TMESS_LOCAL_CONNECT = 22 		 //запрос о подключении
	TMESS_LOCAL_NOTIFICATION = 23 	 //сообщение всплывающее
	TMESS_LOCAL_INFO_CLIENT = 24 	 //побочная информация, путь до внц клиента
	TMESS_LOCAL_TERMINATE = 25 		 //завершение коммуникатора
	TMESS_LOCAL_REG = 26 			 //регистрация профиля
	TMESS_LOCAL_LOGIN = 27 			 //вход в профиль
	TMESS_LOCAL_CONTACT = 28 		 //создание, редактирование, удаление
	TMESS_LOCAL_CONTACTS = 29 		 //весь список контактов профиля
	TMESS_LOCAL_LOGOUT = 30 		 //выход из профиля
	TMESS_LOCAL_CONN_CONTACT = 31	 //подключение к контакту из профиля
	TMESS_LOCAL_MESSAGE = 32 		 //система сообщений
	TMESS_LOCAL_EXEC = 33 			 //запуск приложения, например, внс клиента
	TMESS_LOCAL_STATUS = 34			 //
	TMESS_LOCAL_LISTVNC = 35		 //
	TMESS_LOCAL_INFO_CONTACT = 36	 //
	TMESS_LOCAL_INFO_ANSWER = 37	 //
	TMESS_LOCAL_MANAGE = 38			 //
	TMESS_LOCAL_SAVE = 39			 //
	TMESS_LOCAL_OPTION_CLEAR = 40	 //
	TMESS_LOCAL_RELOAD = 41			 //
	TMESS_LOCAL_LOG = 42			 //
	TMESS_LOCAL_MYMANAGE = 43		 //
	TMESS_LOCAL_MYINFO = 44			 //
	TMESS_LOCAL_INFO_HIDE = 45		 //
	TMESS_LOCAL_CONT_REVERSE = 46 	 //
	TMESS_LOCAL_OPTIONS_UI = 47 	 //
)

var(
	//наша домашяя папка
	parentPath string

	//
	connections Connections

	//ссылки на сокеты для локального сервера, который ждет локальный vnc viewer
	peerBuff1 *pConn
	peerBuff2 *pConn

	//файл для хранения лога
	logFile *os.File

	//конфиг тут храним
	options = Options{
		FDebug: true,
		TypeLog: MESS_FULL }

	//храним здесь информацию о себе: идентификатор, пароль, сокеты и т.п.
	myClient Client

	//тут храним все локальные панели
	localConnections = list.New()

	//меню веб интерфейса
	menus = []itemMenu{
		{"Настройки", "/options"},
		{"Контакты", "/contacts"},
		{"Профиль", "/profile"},
		{"reVisit", "/"} }

	//текстовая расшифровка сообщений для логов
	messLogText = []string{
		"BLANK",
		"ERROR",
		"INFO",
		"DETAIL",
		"FULL" }

	//функции для обработки сообщений
	processing = []ProcessingMessage{
		{TMESS_DEAUTH, processDeauth},
		{TMESS_VERSION, nil},
		{TMESS_AUTH, processAuth},
		{TMESS_LOGIN, processLogin},
		{TMESS_NOTIFICATION, processNotification},
		{TMESS_REQUEST, nil},
		{TMESS_CONNECT, processConnect},
		{TMESS_DISCONNECT, processDisconnect},
		{TMESS_REG, nil},
		{TMESS_CONTACT, processContact},
		{TMESS_CONTACTS, processContacts}, 				//10
		{TMESS_LOGOUT, nil},
		{TMESS_CONNECT_CONTACT, nil},
		{TMESS_STATUSES, nil},
		{TMESS_STATUS, processStatus},
		{TMESS_INFO_CONTACT, processInfoContact},
		{TMESS_INFO_ANSWER, processInfoAnswer},
		{TMESS_MANAGE, processManage},
		{TMESS_PING, processPing},
		{TMESS_CONTACT_REVERSE, nil},

		20:
		{TMESS_LOCAL_TEST, processLocalTest}, 			//20
		{TMESS_LOCAL_INFO, processLocalInfo},
		{TMESS_LOCAL_CONNECT, processLocalConnect},
		{TMESS_LOCAL_NOTIFICATION, nil},
		{TMESS_LOCAL_INFO_CLIENT, processLocalInfoClient},
		{TMESS_LOCAL_TERMINATE, processTerminate},
		{TMESS_LOCAL_REG, processLocalReg},
		{TMESS_LOCAL_LOGIN, processLocalLogin},
		{TMESS_LOCAL_CONTACT, processLocalContact},
		{TMESS_LOCAL_CONTACTS, processLocalContacts},
		{TMESS_LOCAL_LOGOUT, processLocalLogout},		//30
		{TMESS_LOCAL_CONN_CONTACT, processLocalConnectContact},
		{TMESS_LOCAL_MESSAGE, nil},
		{TMESS_LOCAL_EXEC, nil},
		{TMESS_LOCAL_STATUS, nil},
		{TMESS_LOCAL_LISTVNC, processLocalListVNC},
		{TMESS_LOCAL_INFO_CONTACT, processLocalInfoContact},
		{TMESS_LOCAL_INFO_ANSWER, nil},
		{TMESS_LOCAL_MANAGE, processLocalManage},
		{TMESS_LOCAL_SAVE, processLocalSave},
		{TMESS_LOCAL_OPTION_CLEAR, processLocalOptionClear}, //40
		{TMESS_LOCAL_RELOAD, nil},
		{TMESS_LOCAL_LOG, nil},
		{TMESS_LOCAL_MYMANAGE, processLocalMyManage},
		{TMESS_LOCAL_MYINFO, processLocalMyInfo},
		{TMESS_LOCAL_INFO_HIDE, nil},
		{TMESS_LOCAL_CONT_REVERSE, processLocalContactReverse},
		{TMESS_LOCAL_OPTIONS_UI, processLocalOptionsUI} }

	//список доступных vnc
	arrayVnc = []VNC{}

	//собственно помечаем занят уже у нас этот процесс или нет
	flagReinstallVnc = false

	//
	flagReload = false

	//
	flagTerminated = false

	//
	flagPassword = false
)

//структура для хранения конфигурируемых данных
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
	ProfilePass string

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
	TMessage int
	Processing func(message Message, conn *net.Conn)
}

type Message struct {
	TMessage int
	Messages []string
}

type VNC struct {
	FileServer string
	FileClient string

	CmdStartServer string
	CmdStopServer string
	CmdInstallServer string
	CmdRemoveServer string
	CmdConfigServer string
	CmdManageServer string

	CmdStartServerUser string
	CmdStopServerUser string
	CmdInstallServerUser string
	CmdRemoveServerUser string
	CmdConfigServerUser string
	CmdManageServerUser string

	CmdStartClient string
	CmdStopClient string
	CmdInstallClient string
	CmdRemoveClient string
	CmdConfigClient string
	CmdManageClient string

	PortServerVNC string
	Link string
	Name string
	Version string
	Description string
}

type Client struct {
	Serial	string
	Pid		string
	//Pass	string
	Version string
	Salt	string //используем эту соль для паролей
	Profile Profile

	Conn	  *net.Conn
	LocalServ *net.Listener
	DataServ  *net.Listener
	WebServ	  *net.Listener

	Code 	  string //for connection
}

type Profile struct {
	Email	string
	Pass	string

	Contacts *Contact
}

type Contact struct {
	Id      int
	Caption string
	Type	string	//node - контакт, fold - папка
	Pid     string
	//Digest string //приходит, но нам не интересны здесь
	//Salt	 string //приходит, но нам не интересны здесь

	Inner   *Contact
	Next    *Contact
}