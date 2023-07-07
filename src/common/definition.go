package common

import (
	log "github.com/sirupsen/logrus"
	"os"
)

const (
	WhiteLabelName                 = "revisit"
	WhiteLabelFileName             = "admin.exe"
	WhiteLabelCommunicatorFileName = "communicator.exe"
	RevisitVersion                 = "1.20"

	DefaultMainServerName = "server.rvisit.net"
	DefaultNumberPassword = false

	WaitCountRestartSrv = 10
	WaitCount           = 15
	WaitIdle            = 500
	WaitAfterConnect    = 250
	WaitSendMess        = 50
	WaitPing            = 10
	MaxEncPass          = 48
	LengthPass          = 6
	HttpTimeout         = 30

	optionsFile  = "options.cfg"
	logName      = "log.txt"
	maxLogFileMb = 50
)

var (
	//наша домашняя папка
	parentPath string

	//файл для хранения лога
	logFile *os.File

	//конфиг тут храним
	Options options

	Flags = flags{
		ReInstallVNC:    false,
		Reload:          false,
		Terminated:      false,
		Password:        false,
		ChRefreshAgents: make(chan bool),
	}

	//текстовая расшифровка статических сообщений
	MessStaticText = []string{
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
)

type flags struct {
	ReInstallVNC    bool
	Reload          bool
	Terminated      bool
	Password        bool
	ChRefreshAgents chan bool
}

type options struct {
	ServerAddress  string
	MainServerPort string //сервер основной
	DataServerPort string //сервер для коммутации vnc
	HttpServerType string //веб сервер основного
	HttpServerPort string //веб сервер основного

	//сервер для общения с UI
	LocalServerAdr  string
	LocalServerPort string

	//локальный веб сервер для управления
	HttpServerClientAdr  string
	HttpServerClientPort string
	HttpServerClientType string

	//локальный VNC
	LocalAdrVNC   string
	PortClientVNC string

	//строка для прокси соедиения
	Proxy string

	//размер буфера для всех операций с сокетами
	SizeBuff int32

	//максимальный уровень логов
	LogLevel log.Level

	//пароль
	Pass      string
	CleanPass string `json:"-"`

	//данные для автовхода профиля
	ProfileLogin string
	ProfilePass  string

	//опции для UI
	OptionsUI string

	ActiveVncId int
}
