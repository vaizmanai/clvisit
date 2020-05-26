package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/net/proxy"
	"io"
	"net"
	"net/url"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	qLimit = 6
)

var (
	qCount = 0
	mutex  sync.Mutex
)

func takeQ() bool {
	for qCount > qLimit {
		logAdd(MESS_ERROR, "queue is full")
		time.Sleep(time.Millisecond * 100)
		//return false
	}
	mutex.Lock()
	defer mutex.Unlock()
	qCount++
	return true
}

func backQ() {
	mutex.Lock()
	defer mutex.Unlock()
	qCount--
}

func recoverMainClient(conn *net.Conn) {
	if recover() != nil {
		logAdd(MESS_ERROR, "Поток mainClient поймал критическую ошибку")
		debug.PrintStack()

		if conn != nil {
			(*conn).Close()
		}
	}
}

func mainClient() {

	defer recoverMainClient(myClient.Conn)

	go ping()

	for !flagReload {
		logAdd(MESS_INFO, "mainClient пробует подключиться к "+options.MainServerAdr)
		sendMessageToLocalCons(TMESS_LOCAL_INFO_HIDE, "1")

		if len(options.Proxy) > 0 {

			proxy.RegisterDialerType("http", newHTTPProxy)
			httpProxyURI, err := url.Parse("http://" + options.Proxy)
			if err != nil {
				logAdd(MESS_ERROR, "mainClient не смог использовать proxy-строку: "+fmt.Sprint(err))
			}

			dialer, err := proxy.FromURL(httpProxyURI, &net.Dialer{
				Timeout:   PROXY_TIMEOUT * time.Second,
				KeepAlive: PROXY_TIMEOUT * time.Second,
			})
			if err != nil {
				logAdd(MESS_ERROR, "mainClient не смог подключиться к proxy: "+fmt.Sprint(err))
				time.Sleep(time.Second * 1)
				continue
			}

			conn, err := dialer.Dial("tcp", options.MainServerAdr+":"+options.MainServerPort)
			if err != nil {
				logAdd(MESS_ERROR, "mainClient не смог подключиться: "+fmt.Sprint(err))
				time.Sleep(time.Second * 1)
				continue
			}
			myClient.Conn = &conn
		} else {
			conn, err := net.Dial("tcp", options.MainServerAdr+":"+options.MainServerPort)
			if err != nil {
				logAdd(MESS_ERROR, "mainClient не смог подключиться: "+fmt.Sprint(err))
				time.Sleep(time.Second * 1)
				continue
			}
			myClient.Conn = &conn
		}

		//отправим свою версию
		myClient.Version = REVISIT_VERSION
		sendMessage(TMESS_VERSION, myClient.Version)

		//отправим свой идентификатор
		myClient.Serial = getMac()
		sendMessage(TMESS_AUTH, myClient.Serial)

		sendMessage(TMESS_SERVERS)
		sendMessageToLocalCons(TMESS_LOCAL_INFO_HIDE, "0")

		reader := bufio.NewReader(*myClient.Conn)

		for {
			buff, err := reader.ReadBytes('}')

			if err != nil {
				logAdd(MESS_ERROR, "mainClient ошибка чтения буфера: "+fmt.Sprint(err))
				break
			}

			logAdd(MESS_DETAIL, fmt.Sprint("buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))

			//удаляем мусор
			if buff[0] != '{' {
				logAdd(MESS_INFO, "mainServer удаляем мусор")
				if bytes.Index(buff, []byte("{")) >= 0 {
					logAdd(MESS_DETAIL, fmt.Sprint("buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))
					buff = buff[bytes.Index(buff, []byte("{")):]
				} else {
					continue
				}
			}

			var message Message
			err = json.Unmarshal(buff, &message)
			if err != nil {
				logAdd(MESS_ERROR, "mainClient ошибка разбора json: "+fmt.Sprint(err))
				time.Sleep(time.Millisecond * WAIT_IDLE)
				continue
			}

			logAdd(MESS_DETAIL, fmt.Sprint(message))

			//обрабатываем полученное сообщение
			if len(processing) > message.TMessage {
				if processing[message.TMessage].Processing != nil {
					if takeQ() {
						go func() {
							processing[message.TMessage].Processing(message, myClient.Conn)
							backQ()
						}()
					}
				} else {
					logAdd(MESS_INFO, "mainClient нет обработчика для сообщения")
					time.Sleep(time.Millisecond * WAIT_IDLE)
				}
			} else {
				logAdd(MESS_INFO, "mainClient неизвестное сообщение")
				time.Sleep(time.Millisecond * WAIT_IDLE)
			}

		}

		sendMessageToLocalCons(TMESS_LOCAL_LOGOUT)

		logAdd(MESS_INFO, "mainClient остановился")
		(*myClient.Conn).Close()
		myClient.Conn = nil

		time.Sleep(time.Second * 1)
	}
}

func localServer() {
	count := 0
	for count < WAIT_COUNT_RESTART_SRV && !flagReload {
		logAdd(MESS_INFO, "localServer запустился")

		ln, err := net.Listen("tcp", options.LocalServerAdr+":"+options.LocalServerPort)
		if err != nil {
			logAdd(MESS_ERROR, "localServer не смог занять порт: "+fmt.Sprint(err))
			os.Exit(1) //todo наверное оставим так
			time.Sleep(time.Millisecond * WAIT_IDLE)
			count++
			continue
		}

		myClient.LocalServ = &ln

		for {
			conn, err := ln.Accept()
			if err != nil {
				logAdd(MESS_ERROR, "localServer не смог занять сокет: "+fmt.Sprint(err))
				break
			}
			go localHandler(&conn)
		}

		ln.Close()
		logAdd(MESS_INFO, "localServer остановился")
		time.Sleep(time.Millisecond * WAIT_IDLE)
		count++
	}

	if !flagReload {
		logAdd(MESS_ERROR, "localServer так и не смог запуститься")
		os.Exit(1)
	}
}

func localHandler(conn *net.Conn) {
	id := randomString(6)
	logAdd(MESS_INFO, id+" localServer получил соединение")

	item := localConnections.PushBack(conn)

	processLocalInfo(createMessage(TMESS_LOCAL_INFO), conn)
	processLocalInfoClient(createMessage(TMESS_LOCAL_INFO_CLIENT), conn)
	if len(options.OptionsUI) > 0 {
		processLocalOptionsUI(createMessage(TMESS_LOCAL_OPTIONS_UI), conn)
	}

	reader := bufio.NewReader(*conn)

	for {
		buff, err := reader.ReadBytes('}')

		if err != nil {
			logAdd(MESS_ERROR, id+" localServer ошибка чтения буфера: "+fmt.Sprint(err))
			break
		}

		logAdd(MESS_DETAIL, id+fmt.Sprint(" buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))

		//удаляем мусор
		if buff[0] != '{' {
			logAdd(MESS_INFO, id+" localServer удаляем мусор")
			if bytes.Index(buff, []byte("{")) >= 0 {
				buff = buff[bytes.Index(buff, []byte("{")):]
			} else {
				continue
			}
		}

		var message Message
		err = json.Unmarshal(buff, &message)
		if err != nil {
			logAdd(MESS_ERROR, id+" localServer ошибка разбора json: "+fmt.Sprint(err))
			time.Sleep(time.Millisecond * WAIT_IDLE)
			continue
		}

		logAdd(MESS_DETAIL, id+" "+fmt.Sprint(message))

		//обрабатываем полученное сообщение
		if len(localProcessing) > message.TMessage {
			if localProcessing[message.TMessage].Processing != nil {
				if takeQ() {
					go func() {
						localProcessing[message.TMessage].Processing(message, conn)
						backQ()
					}()
				}
			} else {
				logAdd(MESS_INFO, fmt.Sprintf("%s localServer нет обработчика для сообщения", id))
				time.Sleep(time.Millisecond * WAIT_IDLE)
			}
		} else {
			logAdd(MESS_INFO, fmt.Sprintf("%s localServer неизвестное сообщение", id))
			time.Sleep(time.Millisecond * WAIT_IDLE)
		}

	}

	localConnections.Remove(item)
	_ = (*conn).Close()
	logAdd(MESS_INFO, id+" localServer потерял соединение")
}

func localDataServer() {
	count := 0
	for count < WAIT_COUNT_RESTART_SRV && !flagReload {
		logAdd(MESS_INFO, "localDataServer запустился")

		ln, err := net.Listen("tcp", options.LocalAdrVNC+":"+options.PortClientVNC)
		if err != nil {
			logAdd(MESS_ERROR, "localDataServer не смог занять порт: "+fmt.Sprint(err))
			os.Exit(1) //todo наверное оставим так
			time.Sleep(time.Millisecond * WAIT_IDLE)
			count++
			continue
		}

		myClient.DataServ = &ln

		for {
			conn, err := ln.Accept()
			if err != nil {
				logAdd(MESS_ERROR, "localDataServer не смог занять сокет: "+fmt.Sprint(err))
				break
			}
			go localDataHandler(&conn)
		}

		ln.Close()
		logAdd(MESS_INFO, "localDataServer остановился")
		time.Sleep(time.Millisecond * WAIT_IDLE)
		count++
	}

	if !flagReload {
		logAdd(MESS_ERROR, "localDataServer так и не смог запуститься")
		os.Exit(1)
	}
}

func localDataHandler(conn *net.Conn) {
	id := randomString(6)
	logAdd(MESS_INFO, id+" localDataHandler получил соединение")

	var cWait = 0
	for (peerBuff1 == nil || peerBuff2 == nil) && cWait < WAIT_COUNT {
		logAdd(MESS_INFO, id+" ожидание peer для локального сервера...")
		time.Sleep(time.Millisecond * WAIT_IDLE)
		cWait++
	}

	if peerBuff1 == nil || peerBuff2 == nil {
		(*conn).Close()
		logAdd(MESS_INFO, id+" не дождались peer")
		return
	}

	peer1 := peerBuff1
	peer2 := peerBuff2

	peerBuff1 = nil
	peerBuff2 = nil

	peer1.Pointer = conn

	cWait = 0
	for peer2.Pointer == nil && cWait < WAIT_COUNT && !flagTerminated {
		logAdd(MESS_INFO, id+" ожидание peer для локального сервера...")
		time.Sleep(time.Millisecond * WAIT_IDLE)
		cWait++
	}

	if peer2.Pointer == nil {
		(*conn).Close()
		logAdd(MESS_INFO, id+" не дождались peer")
		return
	}

	logAdd(MESS_INFO, id+" peer существует для локального сервера")
	time.Sleep(time.Millisecond * WAIT_AFTER_CONNECT)

	var z []byte
	z = make([]byte, options.SizeBuff)

	for {
		n1, err1 := (*conn).Read(z)
		//fmt.Println(id, "server:", z[:n1], err1)
		n2, err2 := (*peer2.Pointer).Write(z[:n1])
		//fmt.Println(id, "server:", z[:n2], err2)
		if (err1 != nil && err1 != io.EOF) && err2 != nil || n1 == 0 || n2 == 0 {
			logAdd(MESS_INFO, id+" подключение к локальному серверу отвалилось: "+fmt.Sprint(n1, n2))
			(*peer2.Pointer).Close()
			break
		}
	}

	(*conn).Close()
	logAdd(MESS_INFO, id+" подключение к локальному серверу завершило работу")
}

func hideInfo() {
	connections.mutex.Lock()
	connections.count = connections.count + 1
	connections.mutex.Unlock()

	if connections.count > 0 {
		sendMessageToLocalCons(TMESS_LOCAL_INFO_HIDE, "1")
	}
}

func showInfo() {
	connections.mutex.Lock()
	connections.count = connections.count - 1
	connections.mutex.Unlock()

	if connections.count == 0 {
		sendMessageToLocalCons(TMESS_LOCAL_INFO_HIDE, "0")
	}
}

func startVNC() {
	if len(arrayVnc) == 0 || options.ActiveVncId == -1 {
		logAdd(MESS_INFO, "VNC серверы отсутствуют")
		return
	}

	sendMessageToLocalCons(TMESS_LOCAL_INFO_CLIENT, parentPath+VNC_FOLDER+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].Name+"_"+arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator)+strings.Replace(arrayVnc[options.ActiveVncId].CmdStartClient, "%adr", options.LocalAdrVNC+":"+options.PortClientVNC, 1))

	logAdd(MESS_INFO, "Готовим VNC сервер для запуска")
	if !checkForAdmin() {
		logAdd(MESS_INFO, "У нас нет прав Администратора, запускаем обычную версию VNC сервера")

		if configVNCserverUser() {
			if installVNCserverUser() {
				go runVNCserverUser()
			}
		}
	} else {
		logAdd(MESS_INFO, "У нас есть права Администратора, запускаем службу для VNC сервера")

		if configVNCserver() {
			if installVNCserver() {
				go runVNCserver()
			}
		}
	}

}

func closeVNC() {
	if len(arrayVnc) == 0 || options.ActiveVncId == -1 {
		logAdd(MESS_INFO, "VNC серверы отсутствуют")
		return
	}

	if !checkForAdmin() {
		logAdd(MESS_INFO, "У нас нет прав Администратора")

		if stopVNCserverUser() {
			uninstallVNCserverUser()
		}
	} else {
		logAdd(MESS_INFO, "У нас есть права Администратора")

		if stopVNCserver() {
			uninstallVNCserver()
		}
	}

	//контрольный вариант завершения процессов vnc сервера
	emergencyExitVNC(options.ActiveVncId)
}

func configVNCserver() bool {
	logAdd(MESS_INFO, "Импортируем настройки сервера")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdConfigServer) {
		logAdd(MESS_ERROR, "Не получилось импортировать настройки")
		return true //todo change to false
	}

	logAdd(MESS_INFO, "Импортировали настройки сервера")
	return true
}

func installVNCserver() bool {
	logAdd(MESS_INFO, "Устанавливаем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdInstallServer) {
		logAdd(MESS_ERROR, "Не получилось установить VNC сервер")
		return true //todo change to false
	}
	logAdd(MESS_INFO, "Установили VNC сервер")
	return true
}

func runVNCserver() bool {
	logAdd(MESS_INFO, "Запускаем VNC сервер")

	_, pid := checkExistsProcess(arrayVnc[options.ActiveVncId].FileServer)
	if pid != 0 {
		logAdd(MESS_INFO, "VNC сервер уже запущен")
		return true
	}

	os.Chdir(parentPath + VNC_FOLDER + string(os.PathSeparator) + arrayVnc[options.ActiveVncId].Name + "_" + arrayVnc[options.ActiveVncId].Version + string(os.PathSeparator))
	if !actVNC(arrayVnc[options.ActiveVncId].CmdStartServer) {
		logAdd(MESS_ERROR, "Не получилось запустить VNC сервер")
		return false
	}
	logAdd(MESS_INFO, "Запустился VNC сервер")
	return true
}

func stopVNCserver() bool {
	logAdd(MESS_INFO, "Останавливаем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdStopServer) {
		logAdd(MESS_ERROR, "Не получилось установить VNC сервер")
		return true //todo change to false
	}
	logAdd(MESS_INFO, "Остановили VNC сервер")
	return true
}

func uninstallVNCserver() bool {
	logAdd(MESS_INFO, "Удаляем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdRemoveServer) {
		logAdd(MESS_ERROR, "Не получилось запустить VNC сервер")
		return false
	}
	logAdd(MESS_INFO, "Удалили VNC сервер")
	return true
}

func configVNCserverUser() bool {
	logAdd(MESS_INFO, "Импортируем настройки сервера")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdConfigServerUser) {
		logAdd(MESS_ERROR, "Не получилось импортировать настройки")
		return true //todo change to false
	}

	logAdd(MESS_INFO, "Импортировали настройки сервера")
	return true
}

func installVNCserverUser() bool {
	logAdd(MESS_INFO, "Устанавливаем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdInstallServerUser) {
		logAdd(MESS_ERROR, "Не получилось установить VNC сервер")
		return true //todo change to false
	}
	logAdd(MESS_INFO, "Установили VNC сервер")
	return true
}

func runVNCserverUser() bool {
	logAdd(MESS_INFO, "Запускаем VNC сервер")

	_, pid := checkExistsProcess(arrayVnc[options.ActiveVncId].FileServer)
	if pid != 0 {
		logAdd(MESS_INFO, "VNC сервер уже запущен")
		return true
	}

	if !actVNC(arrayVnc[options.ActiveVncId].CmdStartServerUser) {
		logAdd(MESS_ERROR, "Не получилось запустить VNC сервер")
		return false
	}
	logAdd(MESS_INFO, "Завершился VNC сервер")
	return true
}

func stopVNCserverUser() bool {
	logAdd(MESS_INFO, "Останавливаем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdStopServerUser) {
		logAdd(MESS_ERROR, "Не получилось установить VNC сервер")
		return true //todo change to false
	}
	logAdd(MESS_INFO, "Остановили VNC сервер")
	return true
}

func uninstallVNCserverUser() bool {
	logAdd(MESS_INFO, "Удаляем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdRemoveServerUser) {
		logAdd(MESS_ERROR, "Не получилось запустить VNC сервер")
		return false
	}
	logAdd(MESS_INFO, "Удалили VNC сервер")
	return true
}

func ping() {
	logAdd(MESS_DETAIL, "Запустили поток пинга")
	for true {
		time.Sleep(time.Second * WAIT_PING)
		sendMessage(TMESS_PING)
	}
	logAdd(MESS_DETAIL, "Остановили поток пинга")
}

//пир1 в сторону сервера/клиента(если напрямую)
//пир2 в сторону vnc(server/viewer)
func convisit(peerAdr1 string, peerAdr2 string, code string, upnp bool, mode int) {

	logAdd(MESS_INFO, "Запустили поток подключения трансляции")

	//режимы работы коннектора
	//пир1 всегда в сторону сервера/клиента(если напрямую)
	//пир2 всегда в сторону vnc(server/viewer)

	//1 подключаем пир1 и шлем код, подключаем пир2
	//2 подключаем пир1 и шлем код, ждем подключение пир2
	//3 ждем подключение пир1 и ждем код, подключаем пир2
	//4 ждем подключение пир1 и ждем код, ждем подключение пир2

	var peer1, peer2 pConn
	peer1.Pointer = nil
	peer2.Pointer = nil

	if mode == 1 {
		go conclient(peerAdr1, code, &peer1, &peer2, randomString(6))
		go conclient(peerAdr2, "", &peer2, &peer1, randomString(6))
	} else if mode == 2 {
		go conclient(peerAdr1, code, &peer1, &peer2, randomString(6))
		go conserver(peerAdr2, "", &peer2, &peer1, randomString(6))
	} else if mode == 3 {
		go conserver(peerAdr1, code, &peer1, &peer2, randomString(6))
		go conclient(peerAdr2, "", &peer2, &peer1, randomString(6))
	} else if mode == 4 {
		go conserver(peerAdr1, code, &peer1, &peer2, randomString(6))
		go conserver(peerAdr2, "", &peer2, &peer1, randomString(6))
	}

}

func conserver(adr string, code string, peer1 *pConn, peer2 *pConn, id string) {
	peerBuff1 = peer1
	peerBuff2 = peer2
}

func conclient(adr string, code string, peer1 *pConn, peer2 *pConn, id string) {

	logAdd(MESS_INFO, id+" запустили клиента "+code+" к "+adr)
	if len(options.Proxy) > 0 && len(code) > 0 { //если прокси указан и это не локальное подключение

		proxy.RegisterDialerType("http", newHTTPProxy)
		httpProxyURI, err := url.Parse("http://" + options.Proxy)
		if err != nil {
			sendMessage(TMESS_DISCONNECT, code, fmt.Sprint(STATIC_MESSAGE_PROXY_ERROR))
			logAdd(MESS_ERROR, "mainClient не смог использовать proxy-строку: "+fmt.Sprint(err))
			return
		}

		dialer, err := proxy.FromURL(httpProxyURI, &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		})
		if err != nil {
			logAdd(MESS_ERROR, id+" не смог подключиться к proxy "+fmt.Sprint(err))
			sendMessage(TMESS_DISCONNECT, code, fmt.Sprint(STATIC_MESSAGE_PROXY_ERROR))
			return
		}

		conn, err := dialer.Dial("tcp", adr)
		if err != nil {
			logAdd(MESS_ERROR, id+" не смог подключиться: "+fmt.Sprint(err))
			sendMessage(TMESS_DISCONNECT, code, fmt.Sprint(STATIC_MESSAGE_PROXY_ERROR))
			return
		}

		defer conn.Close()
		peer1.Pointer = &conn
	} else {
		conn, err := net.Dial("tcp", adr)
		if err != nil {
			logAdd(MESS_ERROR, id+" не удалось клиенту подключиться: "+fmt.Sprint(err))
			if len(code) > 0 {
				sendMessage(TMESS_DISCONNECT, code, fmt.Sprint(STATIC_MESSAGE_NETWORK_ERROR))
			}
			return
		}

		defer conn.Close()
		peer1.Pointer = &conn
	}

	if len(code) > 0 {
		(*peer1.Pointer).Write([]byte(code + "\n"))
		hideInfo()
		defer showInfo()
	}

	var cWait = 0
	for peer2.Pointer == nil && cWait < WAIT_COUNT {
		logAdd(MESS_INFO, id+" ожидаем peer для клиента...")
		time.Sleep(time.Millisecond * WAIT_IDLE)
		cWait++
	}
	if peer2.Pointer == nil {
		logAdd(MESS_ERROR, id+" превышено время ожидания")
		if len(code) > 0 {
			sendMessage(TMESS_DISCONNECT, code, fmt.Sprint(STATIC_MESSAGE_TIMEOUT_ERROR))
		}
		sendMessageToLocalCons(TMESS_LOCAL_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_LOCAL_ERROR))
		return
	}

	sendMessageToLocalCons(TMESS_LOCAL_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_LOCAL_CONN))
	logAdd(MESS_INFO, id+" peer существует для клиента")
	time.Sleep(time.Millisecond * WAIT_AFTER_CONNECT)

	var z []byte
	z = make([]byte, options.SizeBuff)

	for {
		n1, err1 := (*peer1.Pointer).Read(z)
		n2, err2 := (*peer2.Pointer).Write(z[:n1])
		if (err1 != nil && err1 != io.EOF) && err2 != nil || n1 == 0 || n2 == 0 {
			logAdd(MESS_INFO, id+" соединение клиента отвалилось")
			(*peer2.Pointer).Close()
			break
		}
	}

	logAdd(MESS_INFO, id+" клиент завершил работу")
	sendMessageToLocalCons(TMESS_LOCAL_STANDART_ALERT, fmt.Sprint(STATIC_MESSAGE_LOCAL_DISCONN))
}
