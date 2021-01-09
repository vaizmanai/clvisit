package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	httpClient http.Client
)

func init() {
	httpClient = http.Client{
		Timeout: time.Second * 30,
	}
}

func processDeauth(_ Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел отказ на авторизацию")

	if myClient.Conn != nil {
		_ = (*myClient.Conn).Close()
	}
}

func processAuth(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел ответ на авторизацию")
	if len(message.Messages) < 2 {
		logAdd(MessError, "Не правильное кол-во полей")
		return
	}

	myClient.Pid = message.Messages[0]
	myClient.Salt = message.Messages[1]
	if len(message.Messages) > 2 {
		myClient.Token = message.Messages[2]
	}

	if len(options.Pass) == 0 {
		logAdd(MessInfo, "Сгенерировали новый пароль")

		if DefaultNumberPassword {
			options.Pass = encXOR(randomNumber(LengthPass), myClient.Pid)
		} else {
			options.Pass = encXOR(randomString(LengthPass), myClient.Pid)
		}

		saveOptions()
	}

	sendMessageToLocalCons(TMessLocalInfo, myClient.Pid, getPass(), myClient.Version,
		options.HttpServerClientType+"://"+options.HttpServerClientAdr+":"+options.HttpServerClientPort,
		options.HttpServerType+"://"+options.HttpServerAdr+":"+options.HttpServerPort,
		options.ProfileLogin, options.ProfilePass, myClient.Token)
}

func processLogin(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел ответ на вход в учетную запись")

	sendMessageToLocalCons(TMessLocalLogin)
}

func processNotification(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришло уведомление")
	if len(message.Messages) != 1 {
		logAdd(MessError, "Не правильное кол-во полей")
	}

	sendMessageToLocalCons(TMessLocalNotification, message.Messages[0])
}

func processConnect(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел запрос на подключение")
	if len(message.Messages) < 7 {
		logAdd(MessError, "Не правильное кол-во полей")
	}

	digest := message.Messages[0]
	salt := message.Messages[1]
	code := message.Messages[2]
	tconn := message.Messages[3]
	ctype := message.Messages[4]
	address := message.Messages[6]
	if len(address) < 1 {
		address = options.DataServerAdr
	}

	if getSHA256(getPass()+salt) != digest && ctype == "server" {
		logAdd(MessError, "Не верный пароль")
		sendMessage(TMessNotification, message.Messages[5], "Аутентификация провалилась!") //todo убрать
		sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageAuthError))
		return
	}

	if flagReinstallVnc || options.ActiveVncId == -1 {
		logAdd(MessError, "Не готов VNC")
		sendMessage(TMessNotification, message.Messages[5], "Не готов VNC!") //todo убрать
		sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageVncError))
		return
	}

	if tconn == "simple" {
		logAdd(MessInfo, "Запускаем простой тип подключения")
		if ctype == "server" {
			go connectVisit(address+":"+options.DataServerPort, options.LocalAdrVNC+":"+arrayVnc[options.ActiveVncId].PortServerVNC, code, false, 1) //тот кто передает трансляцию
		} else {
			go connectVisit(address+":"+options.DataServerPort, options.LocalAdrVNC+":"+options.PortClientVNC, code, false, 2) //тот кто получает трансляцию
			sendMessageToSocket(uiClient, TMessLocalExec, parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].Name+"_"+arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator)+strings.Replace(arrayVnc[options.ActiveVncId].CmdStartClient, "%adr", options.LocalAdrVNC+":"+options.PortClientVNC, 1))
			sendMessageToLocalCons(TMessLocalStandardAlert, fmt.Sprint(StaticMessageLocalConn))
		}
	} else {
		logAdd(MessInfo, "Неизвестный тип подключения")
		sendMessage(TMessNotification, message.Messages[5], "Неизвестный тип подключения!") //todo удалить
		sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageTypeError))
	}
}

func processDisconnect(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел запрос на отключение")
	if len(message.Messages) <= 1 {
		logAdd(MessError, "Не правильное кол-во полей")
		return
	}
	sendMessageToLocalCons(TMessLocalStandardAlert, fmt.Sprint(StaticMessageLocalDisconn))
}

func processContacts(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришли контакты")
	dec, err := url.PathUnescape(message.Messages[0])
	if err == nil {
		contact := Contact{}
		err = json.Unmarshal([]byte(dec), &contact)
		if dec != "null" {
			if err == nil {
				myClient.Profile.Contacts = &contact
				b, err := json.Marshal(contact)
				if err == nil {
					sendMessageToLocalCons(TMessLocalContacts, url.PathEscape(string(b)))
				}
			} else {
				fmt.Println(err)
			}
		}
	}
}

func processContact(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришло изменение контакта")

	sendMessageToLocalCons(TMessLocalContact, message.Messages...)
}

func processStatus(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел статус контакта")

	sendMessageToLocalCons(TMessLocalStatus, message.Messages...)
}

func processInfoContact(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел запрос на информацию")

	if getSHA256(getPass()+message.Messages[2]) == message.Messages[1] {
		hostname, _ := os.Hostname()
		sendMessage(TMessInfoAnswer, message.Messages[0], fmt.Sprint(options.ActiveVncId), hostname, GetInfoOS(), RevisitVersion)
		return
	}

	logAdd(MessError, "Не правильные контрольные данные")
}

func processInfoAnswer(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел ответ запроса на информацию")

	sendMessageToLocalCons(TMessLocalInfoAnswer, message.Messages...)
}

func processManage(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел запрос на управление")

	//Message[0] who called(pid)
	//Message[1] digest
	//Message[2] salt
	//Message[3] act

	if getSHA256(getPass()+message.Messages[2]) == message.Messages[1] {

		if message.Messages[3] == "revnc" {
			i, err := strconv.Atoi(message.Messages[4])
			if err == nil {
				go processVNC(i)
				return
			}
			logAdd(MessError, "Не получилось обновить VNC")
			return
		} else if message.Messages[3] == "update" {
			updateMe()
			return
		} else if message.Messages[3] == "reload" {
			reloadMe()
			return
		} else if message.Messages[3] == "restart" {
			restartSystem()
			return
		}

		logAdd(MessError, "Что-то пошло не так")
		return
	}

	logAdd(MessError, "Не правильные контрольные данные")
}

func processPing(message Message, conn *net.Conn) {
	//logAdd(MessInfo, "Пришел пинг")
}

func processStandardAlert(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришло стандартное уведомление")

	if len(message.Messages) > 0 {
		i, err := strconv.Atoi(message.Messages[0])
		if err == nil {
			logAdd(MessInfo, "Текст уведомления: "+messStaticText[i])
			sendMessageToSocket(uiClient, TMessLocalStandardAlert, message.Messages[0])
		}
	}
}

func processServers(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришла информации по агентам")

	if len(message.Messages) == 2 {
		fl := message.Messages[0]
		if fl == "true" || fl == "false" {
			if fl == "true" {
				logAdd(MessInfo, "Агент "+message.Messages[1]+" включился")
				var agent Agent
				agent.Address = message.Messages[1]
				agent.Metric = updateAgentMetric(agent.Address)
				agents = append(agents, agent)
			} else {
				logAdd(MessInfo, "Агент "+message.Messages[1]+" выключился")
				for i := 0; i < len(agents); i++ {
					if agents[i].Address == message.Messages[1] {
						agents[i] = agents[len(agents)-1]
						agents = agents[:len(agents)-1]
						i = 0
					}
				}
			}
			sortAgents()
			return
		}
	}

	logAdd(MessInfo, "Новый список агентов, кол-во: "+fmt.Sprint(len(message.Messages)))
	agents = make([]Agent, len(message.Messages))
	for i := 0; i < len(message.Messages); i++ {
		var agent Agent
		agent.Address = message.Messages[i]
		agent.Metric = -1
		agents[i] = agent
	}

	if chRefreshAgents == nil {
		go refreshAgents()
	} else {
		chRefreshAgents <- true
	}
}

func processLocalTest(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел локальный тест")

	sendMessageToSocket(conn, message.TMessage, "test")
}

func processLocalInfo(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на реквизиты")

	if connections.count > 0 {
		sendMessageToSocket(conn, message.TMessage, "XX:XX:XX:XX", "*****", myClient.Version,
			options.HttpServerClientType+"://"+options.HttpServerClientAdr+":"+options.HttpServerClientPort,
			options.HttpServerType+"://"+options.HttpServerAdr+":"+options.HttpServerPort,
			options.ProfileLogin, options.ProfilePass, myClient.Token)
	} else {
		if len(message.Messages) > 0 {
			if message.Messages[0] == "random" {
				logAdd(MessInfo, "Сгенерировали новый пароль")

				if DefaultNumberPassword {
					options.Pass = encXOR(randomNumber(LengthPass), myClient.Pid)
				} else {
					options.Pass = encXOR(randomString(LengthPass), myClient.Pid)
				}
			} else {
				options.Pass = encXOR(message.Messages[0], myClient.Pid)
			}
			saveOptions()
		}

		sendMessageToSocket(conn, message.TMessage, myClient.Pid, getPass(), myClient.Version,
			options.HttpServerClientType+"://"+options.HttpServerClientAdr+":"+options.HttpServerClientPort,
			options.HttpServerType+"://"+options.HttpServerAdr+":"+options.HttpServerPort,
			options.ProfileLogin, options.ProfilePass, myClient.Token)
	}
}

func processLocalConnect(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на подключение")
	printAgentsMetric()

	uiClient = conn
	if len(agents) > 0 && agents[0].Metric != -1 {
		sendMessage(TMessRequest, message.Messages[0], getSHA256(message.Messages[1]+myClient.Salt), "", agents[0].Address)
	} else {
		sendMessage(TMessRequest, message.Messages[0], getSHA256(message.Messages[1]+myClient.Salt))
	}
}

func processLocalInfoClient(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос о vnc клиенте")

	if options.ActiveVncId != -1 {
		if checkForAdmin() {
			sendMessageToSocket(conn, message.TMessage,
				parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].Name+"_"+arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator)+strings.Replace(arrayVnc[options.ActiveVncId].CmdStartClient, "%adr", options.LocalAdrVNC+":"+options.PortClientVNC, 1),
				parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].Name+"_"+arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].CmdManageServer)
		} else {
			sendMessageToSocket(conn, message.TMessage,
				parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].Name+"_"+arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator)+strings.Replace(arrayVnc[options.ActiveVncId].CmdStartClient, "%adr", options.LocalAdrVNC+":"+options.PortClientVNC, 1),
				parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].Name+"_"+arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].CmdManageServerUser)
		}
	}
}

func processTerminate(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел запрос на завершение и удаление")

	if message.Messages[0] == "1" {
		terminateMe(true)
	} else {
		sendMessageToSocket(conn, message.TMessage)
		terminateMe(false)
	}

}

func processLocalReg(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на регистрацию учетки")

	uiClient = conn
	sendMessage(TMessReg, message.Messages[0])
}

func processLocalLogin(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на вход в учетку")

	if len(myClient.Pid) == 0 {
		logAdd(MessInfo, "Ещё не готовы к авторизации в профиль")
		return
	}

	uiClient = conn
	if message.Messages[2] == "1" {
		logAdd(MessInfo, "Сохраним данные для входа в профиль")
		options.ProfileLogin = message.Messages[0]
		options.ProfilePass = message.Messages[1]
		saveOptions()
	} else {
		logAdd(MessInfo, "Удалим данные для входа в профиль")
		options.ProfileLogin = ""
		options.ProfilePass = ""
		saveOptions()
	}
	sendMessage(TMessLogin, message.Messages[0], getSHA256(message.Messages[1]+myClient.Salt))
}

func processLocalContact(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос управления контактом")

	digest := ""
	if len(message.Messages[4]) > 0 {
		digest = getSHA256(message.Messages[4] + myClient.Salt)
	}
	sendMessage(TMessContact, message.Messages[0], message.Messages[1], message.Messages[2], message.Messages[3], digest, message.Messages[5])
}

func processLocalContacts(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на обновления списка контактов")

	sendMessage(TMessContacts)
}

func processLocalLogout(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на выход")
	myClient.Profile.Contacts = nil

	options.ProfileLogin = ""
	options.ProfilePass = ""
	saveOptions()

	sendMessage(TMessLogout)
}

func processLocalConnectContact(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на подключение к контакту")

	uiClient = conn
	if len(agents) > 0 && agents[0].Metric != -1 {
		sendMessage(TMessConnectContact, message.Messages[0], agents[0].Address)
	} else {
		sendMessage(TMessConnectContact, message.Messages[0])
	}

	sendMessageToLocalCons(TMessLocalStandardAlert, fmt.Sprint(StaticMessageLocalReq))
}

func processLocalListVNC(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на список VNC")

	resp, err := httpClient.Get(options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort + "/api?make=listvnc")
	if err != nil {
		logAdd(MessError, "Не получилось получить с сервера VNC: "+err.Error())
		return
	}

	var buff []byte
	buff = make([]byte, options.SizeBuff*options.SizeBuff)
	n, err := resp.Body.Read(buff)
	defer resp.Body.Close()

	var listVNC []VNC
	err = json.Unmarshal(buff[:n], &listVNC)
	if err != nil {
		logAdd(MessError, "Не получилось получить с сервера VNC: "+err.Error())
		return
	}

	for _, x := range listVNC {
		sendMessageToSocket(conn, TMessLocalListVNC, x.Name+" "+x.Version, x.Link)
	}
}

func processLocalInfoContact(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на информацию о контакте")

	sendMessage(TMessInfoContact, message.Messages[0])
}

func processLocalManage(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на управление")

	sendMessage(TMessManage, message.Messages...)
}

func processLocalSave(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на сохранение опций")

	saveOptions()
}

func processLocalOptionClear(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на восстановление дефолтных опций")

	defaultOptions()
	processVNC(0)
}

func processLocalMyManage(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на своё управление")

	if message.Messages[0] == "revnc" {
		i, err := strconv.Atoi(message.Messages[1])
		if err == nil {
			go processVNC(i)
			return
		}
		logAdd(MessError, "Не получилось обновить VNC")
		return
	} else if message.Messages[0] == "update" {
		updateMe()
		return
	} else if message.Messages[0] == "reload" {
		reloadMe()
		return
	} else if message.Messages[0] == "restart" {
		restartSystem()
		return
	}
}

func processLocalMyInfo(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на свою информацию")

	hostname, _ := os.Hostname()
	sendMessageToLocalCons(TMessLocalInfoAnswer, "", fmt.Sprint(options.ActiveVncId), hostname, GetInfoOS(), RevisitVersion)
}

func processLocalContactReverse(message Message, _ *net.Conn) {
	logAdd(MessInfo, "Пришел локальный запрос на добавление в чужой профиль")

	hostname, _ := os.Hostname()
	sendMessage(TMessContactReverse, message.Messages[0], getSHA256(message.Messages[1]+myClient.Salt), hostname)
}

func processLocalOptionsUI(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел запрос на работу с опциями UI")

	if len(message.Messages) == 0 {
		sendMessageToSocket(conn, message.TMessage, options.OptionsUI)
	} else {
		options.OptionsUI = message.Messages[0]
		saveOptions()
	}
}

func processLocalProxy(message Message, conn *net.Conn) {
	logAdd(MessInfo, "Пришел запрос на настройку прокси")

	if len(message.Messages) == 2 {
		options.Proxy = message.Messages[0] + ":" + message.Messages[1]
		saveOptions()
		if myClient.Conn != nil {
			_ = (*myClient.Conn).Close()
		}
	} else {
		if strings.Contains(options.Proxy, ":") {
			proxy := strings.Split(options.Proxy, ":")
			sendMessageToSocket(conn, message.TMessage, proxy[0], proxy[1])
		} else {
			sendMessageToSocket(conn, message.TMessage, "", "")
		}
	}
}
