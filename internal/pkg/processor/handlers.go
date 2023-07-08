package processor

import (
	"clvisit/internal/pkg/common"
	"clvisit/internal/pkg/vnc"
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

var (
	httpClient *http.Client
)

func getHttpClient() *http.Client {
	if httpClient == nil {
		httpClient = common.GetHttpClient()
	}
	return httpClient
}

func processDeauth(_ Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел отказ на авторизацию")

	if myClient.Conn != nil {
		_ = (*myClient.Conn).Close()
	}
}

func processAuth(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел ответ на авторизацию")
	if len(message.Messages) < 2 {
		log.Errorf("не правильное кол-во полей")
		return
	}

	myClient.Pid = message.Messages[0]
	myClient.Salt = message.Messages[1]
	if len(message.Messages) > 2 {
		myClient.Token = message.Messages[2]
	}

	if len(common.Options.Pass) == 0 {
		log.Infof("сгенерировали новый пароль")

		if common.DefaultNumberPassword {
			common.Options.Pass = common.EncXOR(common.RandomNumber(common.LengthPass), myClient.Pid)
		} else {
			common.Options.Pass = common.EncXOR(common.RandomString(common.LengthPass), myClient.Pid)
		}

		common.SaveOptions()
	}

	sendMessageToLocalCons(TMessLocalInfo, myClient.Pid, GetPass(), myClient.Version,
		common.GetLocalHttpServerAddress(),
		common.GetHttpServerAddress(),
		common.Options.ProfileLogin, common.Options.ProfilePass, myClient.Token)
}

func processLogin(_ Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел ответ на вход в учетную запись")

	sendMessageToLocalCons(TMessLocalLogin)
}

func processNotification(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришло уведомление")
	if len(message.Messages) != 1 {
		log.Errorf("не правильное кол-во полей")
	}

	sendMessageToLocalCons(TMessLocalNotification, message.Messages[0])
}

func processConnect(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел запрос на подключение")
	if len(message.Messages) < 7 {
		log.Errorf("не правильное кол-во полей")
	}

	digest := message.Messages[0]
	salt := message.Messages[1]
	code := message.Messages[2]
	tconn := message.Messages[3]
	ctype := message.Messages[4]
	address := message.Messages[6]
	if len(address) < 1 {
		address = common.Options.ServerAddress
	}

	if common.GetSHA256(GetPass()+salt) != digest && ctype == "server" {
		log.Errorf("не верный пароль")
		sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageAuthError))
		return
	}

	if common.Flags.ReInstallVNC || common.Options.ActiveVncId == -1 {
		log.Errorf("не готов VNC")
		sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageVncError))
		return
	}

	if tconn == "simple" {
		log.Infof("запускаем простой тип подключения")
		if ctype == "server" {
			//тот кто передает трансляцию
			go connectVisit(fmt.Sprintf("%s:%s", address, common.Options.DataServerPort), fmt.Sprintf("%s:%s", common.Options.LocalAdrVNC, vnc.GetActiveVNC().PortServerVNC), code, false, 1)
		} else {
			//тот кто получает трансляцию
			go connectVisit(fmt.Sprintf("%s:%s", address, common.Options.DataServerPort), fmt.Sprintf("%s:%s", common.Options.LocalAdrVNC, common.Options.PortClientVNC), code, false, 2)

			if uiClient != nil {
				//classic ui
				sendMessageToSocket(uiClient, TMessLocalExec, fmt.Sprintf("%s%s", vnc.GetActiveFolder(), strings.ReplaceAll(vnc.GetActiveVNC().CmdStartClient, "%adr", common.GetVNCAddress())))
			} else {
				//web ui
				go vnc.RunVNCClient()
			}

			sendMessageToLocalCons(TMessLocalStandardAlert, fmt.Sprint(StaticMessageLocalConn))
		}
	} else {
		log.Infof("неизвестный тип подключения")
		sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageTypeError))
	}
}

func processDisconnect(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел запрос на отключение")
	if len(message.Messages) <= 1 {
		log.Errorf("не правильное кол-во полей")
		return
	}
	sendMessageToLocalCons(TMessLocalStandardAlert, fmt.Sprint(StaticMessageLocalDisconn))
}

func processContacts(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришли контакты")
	if dec, err := url.PathUnescape(message.Messages[0]); err != nil {
		log.Errorf("escaped error: %s", err.Error())
		return
	} else {
		contact := common.Contact{}
		if err = json.Unmarshal([]byte(dec), &contact); dec != "null" {
			if err != nil {
				log.Errorf("unmarshaling contact: %s", err.Error())
			} else {
				myClient.Profile.Contacts = &contact
				if b, err := json.Marshal(contact); err != nil {
					log.Errorf("marshaling contact: %s", err.Error())
				} else {
					sendMessageToLocalCons(TMessLocalContacts, url.PathEscape(string(b)))
				}
			}
		}
	}
}

func processContact(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришло изменение контакта")

	sendMessageToLocalCons(TMessLocalContact, message.Messages...)
}

func processStatus(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел статус контакта")

	sendMessageToLocalCons(TMessLocalStatus, message.Messages...)
}

func processInfoContact(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел запрос на информацию")

	if common.GetSHA256(GetPass()+message.Messages[2]) == message.Messages[1] {
		hostname, _ := os.Hostname()
		sendMessage(TMessInfoAnswer, message.Messages[0], fmt.Sprint(common.Options.ActiveVncId), hostname, common.GetInfoOS(), common.RevisitVersion)
		return
	}

	log.Errorf("не правильные контрольные данные")
}

func processInfoAnswer(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел ответ запроса на информацию")

	sendMessageToLocalCons(TMessLocalInfoAnswer, message.Messages...)
}

func processManage(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел запрос на управление")

	//Message[0] who called(pid)
	//Message[1] digest
	//Message[2] salt
	//Message[3] act

	if common.GetSHA256(GetPass()+message.Messages[2]) == message.Messages[1] {
		if message.Messages[3] == "revnc" {
			i, err := strconv.Atoi(message.Messages[4])
			if err == nil {
				go func() {
					vnc.ProcessVNC(i)
					SendInfo()
				}()
				return
			}
			log.Errorf("не получилось обновить VNC")
			return
		} else if message.Messages[3] == "update" {
			UpdateMe()
			return
		} else if message.Messages[3] == "reload" {
			ReloadMe()
			return
		} else if message.Messages[3] == "restart" {
			common.RestartSystem()
			return
		}

		log.Errorf("что-то пошло не так")
		return
	}

	log.Errorf("не правильные контрольные данные")
}

func processPing(_ Message, _ *net.Conn, _ context.Context) {
	//log.Debugf("пришел пинг")
}

func processStandardAlert(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришло стандартное уведомление")

	if len(message.Messages) > 0 {
		if i, err := strconv.Atoi(message.Messages[0]); err != nil {
			log.Errorf("cannot parse: %s", err.Error())
		} else if i < len(common.MessStaticText) {
			log.Infof("текст уведомления: %s", common.MessStaticText[i])
			sendMessageToSocket(uiClient, TMessLocalStandardAlert, message.Messages[0])
			common.SetAlert(common.MessStaticText[i])
		} else {
			log.Warnf("cannot find status message")
		}
	}
}

func processServers(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришла информации по агентам")

	if len(message.Messages) == 2 {
		fl := message.Messages[0]
		if fl == "true" || fl == "false" {
			if fl == "true" {
				log.Infof("агент %s включился", message.Messages[1])
				var agent Agent
				agent.Address = message.Messages[1]
				agent.Metric = UpdateAgentMetric(agent.Address)
				agents = append(agents, agent)
			} else {
				log.Infof("агент %s выключился", message.Messages[1])
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

	log.Infof("новый список агентов, кол-во: %d", len(message.Messages))
	agents = make([]Agent, len(message.Messages))
	for i := 0; i < len(message.Messages); i++ {
		var agent Agent
		agent.Address = message.Messages[i]
		agent.Metric = -1
		agents[i] = agent
	}

	if common.Flags.ChRefreshAgents == nil {
		go refreshAgents()
	} else {
		common.Flags.ChRefreshAgents <- true
	}
}

func processLocalTest(message Message, conn *net.Conn, _ context.Context) {
	log.Infof("пришел локальный тест")

	sendMessageToSocket(conn, message.TMessage, "test")
}

func processLocalInfo(message Message, conn *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на реквизиты")

	if connections.count > 0 {
		sendMessageToSocket(conn, message.TMessage, "XX:XX:XX:XX", "*****", myClient.Version,
			common.GetLocalHttpServerAddress(),
			common.GetHttpServerAddress(),
			common.Options.ProfileLogin, common.Options.ProfilePass, myClient.Token)
	} else {
		if len(message.Messages) > 0 {
			if message.Messages[0] == "random" {
				log.Infof("сгенерировали новый пароль")

				if common.DefaultNumberPassword {
					SetPass(common.RandomNumber(common.LengthPass))
				} else {
					SetPass(common.RandomString(common.LengthPass))
				}
			} else {
				SetPass(message.Messages[0])
			}
			common.SaveOptions()
		}

		sendMessageToSocket(conn, message.TMessage, myClient.Pid, GetPass(), myClient.Version,
			common.GetLocalHttpServerAddress(),
			common.GetHttpServerAddress(),
			common.Options.ProfileLogin, common.Options.ProfilePass, myClient.Token)
	}
}

func processLocalConnect(message Message, conn *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на подключение")
	printAgentsMetric()

	uiClient = conn
	if len(agents) > 0 && agents[0].Metric != -1 {
		sendMessage(TMessRequest, message.Messages[0], common.GetSHA256(message.Messages[1]+myClient.Salt), "", agents[0].Address)
	} else {
		sendMessage(TMessRequest, message.Messages[0], common.GetSHA256(message.Messages[1]+myClient.Salt))
	}
}

func processLocalInfoClient(message Message, conn *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос о vnc клиенте")

	if common.Options.ActiveVncId != -1 {
		if common.CheckForAdmin() {
			sendMessageToSocket(conn, message.TMessage,
				fmt.Sprintf("%s%s", vnc.GetActiveFolder(), strings.ReplaceAll(vnc.GetActiveVNC().CmdStartClient, "%adr", common.GetVNCAddress())),
				fmt.Sprintf("%s%s", vnc.GetActiveFolder(), vnc.GetActiveVNC().CmdManageServer),
			)
		} else {
			sendMessageToSocket(conn, message.TMessage,
				fmt.Sprintf("%s%s", vnc.GetActiveFolder(), strings.ReplaceAll(vnc.GetActiveVNC().CmdStartClient, "%adr", common.GetVNCAddress())),
				fmt.Sprintf("%s%s", vnc.GetActiveFolder(), vnc.GetActiveVNC().CmdManageServerUser),
			)
		}
	}
}

func processTerminate(message Message, conn *net.Conn, _ context.Context) {
	log.Infof("пришел запрос на завершение и удаление")

	if message.Messages[0] == "1" {
		TerminateMe(true)
	} else {
		sendMessageToSocket(conn, message.TMessage)
		TerminateMe(false)
	}

}

func processLocalReg(message Message, conn *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на регистрацию учетки")

	uiClient = conn
	sendMessage(TMessReg, message.Messages[0])
}

func processLocalLogin(message Message, conn *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на вход в учетку")

	if len(myClient.Pid) == 0 {
		log.Infof("ещё не готовы к авторизации в профиль")
		return
	}

	uiClient = conn
	if message.Messages[2] == "1" {
		log.Infof("сохраним данные для входа в профиль")
		myClient.Profile.Email = message.Messages[0]
		myClient.Profile.Pass = message.Messages[1]
		common.Options.ProfileLogin = message.Messages[0]
		common.Options.ProfilePass = message.Messages[1]
		common.SaveOptions()
	} else {
		log.Infof("удалим данные для входа в профиль")
		myClient.Profile.Email = ""
		myClient.Profile.Pass = ""
		common.Options.ProfileLogin = ""
		common.Options.ProfilePass = ""
		common.SaveOptions()
	}
	sendMessage(TMessLogin, message.Messages[0], common.GetSHA256(message.Messages[1]+myClient.Salt))
}

func processLocalContact(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос управления контактом")

	digest := ""
	if len(message.Messages[4]) > 0 {
		digest = common.GetSHA256(message.Messages[4] + myClient.Salt)
	}
	sendMessage(TMessContact, message.Messages[0], message.Messages[1], message.Messages[2], message.Messages[3], digest, message.Messages[5])
}

func processLocalContacts(_ Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на обновления списка контактов")

	sendMessage(TMessContacts)
}

func processLocalLogout(_ Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на выход")
	myClient.Profile.Contacts = nil

	common.Options.ProfileLogin = ""
	common.Options.ProfilePass = ""
	common.SaveOptions()

	sendMessage(TMessLogout)
}

func processLocalConnectContact(message Message, conn *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на подключение к контакту")

	uiClient = conn
	if len(agents) > 0 && agents[0].Metric != -1 {
		sendMessage(TMessConnectContact, message.Messages[0], agents[0].Address)
	} else {
		sendMessage(TMessConnectContact, message.Messages[0])
	}

	sendMessageToLocalCons(TMessLocalStandardAlert, fmt.Sprint(StaticMessageLocalReq))
}

func processLocalListVNC(_ Message, conn *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на список VNC")

	resp, err := getHttpClient().Get(fmt.Sprintf("%s/api?make=listvnc", common.GetHttpServerAddress()))
	if err != nil {
		log.Errorf("не получилось получить с сервера VNC: %s", err.Error())
		return
	}

	var buff []byte
	buff = make([]byte, common.Options.SizeBuff*common.Options.SizeBuff)
	n, err := resp.Body.Read(buff)
	defer func() {
		_ = resp.Body.Close()
	}()

	var listVNC []vnc.VNC
	if err = json.Unmarshal(buff[:n], &listVNC); err != nil {
		log.Errorf("не получилось получить с сервера VNC: %s", err.Error())
		return
	}

	for _, x := range listVNC {
		sendMessageToSocket(conn, TMessLocalListVNC, x.String(), x.Link)
	}
}

func processLocalInfoContact(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на информацию о контакте")

	sendMessage(TMessInfoContact, message.Messages[0])
}

func processLocalManage(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на управление")

	sendMessage(TMessManage, message.Messages...)
}

func processLocalSave(_ Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на сохранение опций")

	common.SaveOptions()
}

func processLocalOptionClear(_ Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на восстановление дефолтных опций")

	common.SetDefaultOptions()
	vnc.ProcessVNC(0)
	SendInfo()
}

func processLocalMyManage(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на своё управление")

	if message.Messages[0] == "revnc" {
		i, err := strconv.Atoi(message.Messages[1])
		if err == nil {
			go func() {
				vnc.ProcessVNC(i)
				SendInfo()
			}()
			return
		}
		log.Errorf("не получилось обновить VNC")
		return
	} else if message.Messages[0] == "update" {
		UpdateMe()
		return
	} else if message.Messages[0] == "reload" {
		ReloadMe()
		return
	} else if message.Messages[0] == "restart" {
		common.RestartSystem()
		return
	}
}

func processLocalMyInfo(_ Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на свою информацию")

	hostname, _ := os.Hostname()
	sendMessageToLocalCons(TMessLocalInfoAnswer, "", fmt.Sprint(common.Options.ActiveVncId), hostname, common.GetInfoOS(), common.RevisitVersion)
}

func processLocalContactReverse(message Message, _ *net.Conn, _ context.Context) {
	log.Infof("пришел локальный запрос на добавление в чужой профиль")

	hostname, _ := os.Hostname()
	sendMessage(TMessContactReverse, message.Messages[0], common.GetSHA256(message.Messages[1]+myClient.Salt), hostname)
}

func processLocalOptionsUI(message Message, conn *net.Conn, _ context.Context) {
	log.Infof("пришел запрос на работу с опциями UI")

	if len(message.Messages) == 0 {
		sendMessageToSocket(conn, message.TMessage, common.Options.OptionsUI)
	} else {
		common.Options.OptionsUI = message.Messages[0]
		common.SaveOptions()
	}
}

func processLocalProxy(message Message, conn *net.Conn, _ context.Context) {
	log.Infof("пришел запрос на настройку прокси")

	if len(message.Messages) == 2 {
		common.Options.Proxy = fmt.Sprintf("%s:%s", message.Messages[0], message.Messages[1])
		common.SaveOptions()
		if myClient.Conn != nil {
			_ = (*myClient.Conn).Close()
		}
	} else {
		if strings.Contains(common.Options.Proxy, ":") {
			if proxy := strings.Split(common.Options.Proxy, ":"); len(proxy) >= 2 {
				sendMessageToSocket(conn, message.TMessage, proxy[0], proxy[1])
			}
		} else {
			sendMessageToSocket(conn, message.TMessage, "", "")
		}
	}
}
