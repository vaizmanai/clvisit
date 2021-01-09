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
		logAdd(MessError, "queue is full")
		time.Sleep(time.Millisecond * 100)
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
		logAdd(MessError, "Поток mainClient поймал критическую ошибку")
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
		logAdd(MessInfo, "mainClient пробует подключиться к "+options.MainServerAdr)
		sendMessageToLocalCons(TMessLocalInfoHide, "1")

		if len(options.Proxy) > 0 {

			proxy.RegisterDialerType("http", newHTTPProxy)
			httpProxyURI, err := url.Parse("http://" + options.Proxy)
			if err != nil {
				logAdd(MessError, "mainClient не смог использовать proxy-строку: "+fmt.Sprint(err))
			}

			dialer, err := proxy.FromURL(httpProxyURI, &net.Dialer{
				Timeout:   ProxyTimeout * time.Second,
				KeepAlive: ProxyTimeout * time.Second,
			})
			if err != nil {
				logAdd(MessError, "mainClient не смог подключиться к proxy: "+fmt.Sprint(err))
				time.Sleep(time.Second * 1)
				continue
			}

			conn, err := dialer.Dial("tcp", options.MainServerAdr+":"+options.MainServerPort)
			if err != nil {
				logAdd(MessError, "mainClient не смог подключиться: "+fmt.Sprint(err))
				time.Sleep(time.Second * 1)
				continue
			}
			myClient.Conn = &conn
		} else {
			conn, err := net.Dial("tcp", options.MainServerAdr+":"+options.MainServerPort)
			if err != nil {
				logAdd(MessError, "mainClient не смог подключиться: "+fmt.Sprint(err))
				time.Sleep(time.Second * 1)
				continue
			}
			myClient.Conn = &conn
		}

		//отправим свою версию
		myClient.Version = RevisitVersion
		sendMessage(TMessVersion, myClient.Version)

		//отправим свой идентификатор
		myClient.Serial = getMac()
		sendMessage(TMessAuth, myClient.Serial)

		sendMessage(TMessServers)
		sendMessageToLocalCons(TMessLocalInfoHide, "0")

		reader := bufio.NewReader(*myClient.Conn)

		for {
			buff, err := reader.ReadBytes('}')

			if err != nil {
				logAdd(MessError, "mainClient ошибка чтения буфера: "+fmt.Sprint(err))
				break
			}

			logAdd(MessDetail, fmt.Sprint("buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))

			//удаляем мусор
			if buff[0] != '{' {
				logAdd(MessInfo, "mainServer удаляем мусор")
				if bytes.Index(buff, []byte("{")) >= 0 {
					logAdd(MessDetail, fmt.Sprint("buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))
					buff = buff[bytes.Index(buff, []byte("{")):]
				} else {
					continue
				}
			}

			var message Message
			err = json.Unmarshal(buff, &message)
			if err != nil {
				logAdd(MessError, "mainClient ошибка разбора json: "+fmt.Sprint(err))
				time.Sleep(time.Millisecond * WaitIdle)
				continue
			}

			logAdd(MessDetail, fmt.Sprint(message))

			//обрабатываем полученное сообщение
			if len(processing) > message.TMessage {
				if processing[message.TMessage].Processing != nil {
					if takeQ() {
						go func() {
							//todo давай-ка мы тут таймаут какой-то добавим
							processing[message.TMessage].Processing(message, myClient.Conn)
							backQ()
						}()
					}
				} else {
					logAdd(MessInfo, "mainClient нет обработчика для сообщения")
					time.Sleep(time.Millisecond * WaitIdle)
				}
			} else {
				logAdd(MessInfo, "mainClient неизвестное сообщение")
				time.Sleep(time.Millisecond * WaitIdle)
			}

		}

		sendMessageToLocalCons(TMessLocalLogout)

		logAdd(MessInfo, "mainClient остановился")
		_ = (*myClient.Conn).Close()
		myClient.Conn = nil

		time.Sleep(time.Second * 1)
	}
}

func localServer() {
	count := 0
	for count < WaitCountRestartSrv && !flagReload {
		logAdd(MessInfo, "localServer запустился")

		ln, err := net.Listen("tcp", options.LocalServerAdr+":"+options.LocalServerPort)
		if err != nil {
			logAdd(MessError, "localServer не смог занять порт: "+fmt.Sprint(err))
			panic(err.Error())
		}

		myClient.LocalServ = &ln

		for {
			conn, err := ln.Accept()
			if err != nil {
				logAdd(MessError, "localServer не смог занять сокет: "+fmt.Sprint(err))
				break
			}
			go localHandler(&conn)
		}

		_ = ln.Close()
		logAdd(MessInfo, "localServer остановился")
		time.Sleep(time.Millisecond * WaitIdle)
		count++
	}

	if !flagReload {
		logAdd(MessError, "localServer так и не смог запуститься")
		os.Exit(1)
	}
}

func localHandler(conn *net.Conn) {
	id := randomString(6)
	logAdd(MessInfo, id+" localServer получил соединение")

	item := localConnections.PushBack(conn)

	processLocalInfo(createMessage(TMessLocalInfo), conn)
	processLocalInfoClient(createMessage(TMessLocalInfoClient), conn)
	if len(options.OptionsUI) > 0 {
		processLocalOptionsUI(createMessage(TMessLocalOptionsUi), conn)
	}

	reader := bufio.NewReader(*conn)

	for {
		buff, err := reader.ReadBytes('}')

		if err != nil {
			logAdd(MessError, id+" localServer ошибка чтения буфера: "+fmt.Sprint(err))
			break
		}

		logAdd(MessDetail, id+fmt.Sprint(" buff ("+strconv.Itoa(len(buff))+"): "+string(buff)))

		//удаляем мусор
		if buff[0] != '{' {
			logAdd(MessInfo, id+" localServer удаляем мусор")
			if bytes.Index(buff, []byte("{")) >= 0 {
				buff = buff[bytes.Index(buff, []byte("{")):]
			} else {
				continue
			}
		}

		var message Message
		err = json.Unmarshal(buff, &message)
		if err != nil {
			logAdd(MessError, id+" localServer ошибка разбора json: "+fmt.Sprint(err))
			time.Sleep(time.Millisecond * WaitIdle)
			continue
		}

		logAdd(MessDetail, id+" "+fmt.Sprint(message))

		//обрабатываем полученное сообщение
		if len(localProcessing) > message.TMessage {
			if localProcessing[message.TMessage].Processing != nil {
				if takeQ() {
					go func() {
						//todo давай-ка мы тут таймаут какой-то добавим
						localProcessing[message.TMessage].Processing(message, conn)
						backQ()
					}()
				}
			} else {
				logAdd(MessInfo, fmt.Sprintf("%s localServer нет обработчика для сообщения", id))
				time.Sleep(time.Millisecond * WaitIdle)
			}
		} else {
			logAdd(MessInfo, fmt.Sprintf("%s localServer неизвестное сообщение", id))
			time.Sleep(time.Millisecond * WaitIdle)
		}

	}

	localConnections.Remove(item)
	_ = (*conn).Close()
	logAdd(MessInfo, id+" localServer потерял соединение")
}

func localDataServer() {
	count := 0
	for count < WaitCountRestartSrv && !flagReload {
		logAdd(MessInfo, "localDataServer запустился")

		ln, err := net.Listen("tcp", options.LocalAdrVNC+":"+options.PortClientVNC)
		if err != nil {
			logAdd(MessError, "localDataServer не смог занять порт: "+fmt.Sprint(err))
			panic(err.Error())
		}

		myClient.DataServ = &ln

		for {
			conn, err := ln.Accept()
			if err != nil {
				logAdd(MessError, "localDataServer не смог занять сокет: "+fmt.Sprint(err))
				break
			}
			go localDataHandler(&conn)
		}

		_ = ln.Close()
		logAdd(MessInfo, "localDataServer остановился")
		time.Sleep(time.Millisecond * WaitIdle)
		count++
	}

	if !flagReload {
		logAdd(MessError, "localDataServer так и не смог запуститься")
		os.Exit(1)
	}
}

func localDataHandler(conn *net.Conn) {
	id := randomString(6)
	logAdd(MessInfo, id+" localDataHandler получил соединение")

	var cWait = 0
	for (peerBuff1 == nil || peerBuff2 == nil) && cWait < WaitCount {
		logAdd(MessInfo, id+" ожидание peer для локального сервера...")
		time.Sleep(time.Millisecond * WaitIdle)
		cWait++
	}

	if peerBuff1 == nil || peerBuff2 == nil {
		_ = (*conn).Close()
		logAdd(MessInfo, id+" не дождались peer")
		return
	}

	peer1 := peerBuff1
	peer2 := peerBuff2

	peerBuff1 = nil
	peerBuff2 = nil

	peer1.Pointer = conn

	cWait = 0
	for peer2.Pointer == nil && cWait < WaitCount && !flagTerminated {
		logAdd(MessInfo, id+" ожидание peer для локального сервера...")
		time.Sleep(time.Millisecond * WaitIdle)
		cWait++
	}

	if peer2.Pointer == nil {
		_ = (*conn).Close()
		logAdd(MessInfo, id+" не дождались peer")
		return
	}

	logAdd(MessInfo, id+" peer существует для локального сервера")
	time.Sleep(time.Millisecond * WaitAfterConnect)

	var z []byte
	z = make([]byte, options.SizeBuff)

	for {
		n1, err1 := (*conn).Read(z)
		//fmt.Println(id, "server:", z[:n1], err1)
		n2, err2 := (*peer2.Pointer).Write(z[:n1])
		//fmt.Println(id, "server:", z[:n2], err2)
		if (err1 != nil && err1 != io.EOF) && err2 != nil || n1 == 0 || n2 == 0 {
			logAdd(MessInfo, id+" подключение к локальному серверу отвалилось: "+fmt.Sprint(n1, n2))
			_ = (*peer2.Pointer).Close()
			break
		}
	}

	_ = (*conn).Close()
	logAdd(MessInfo, id+" подключение к локальному серверу завершило работу")
}

func hideInfo() {
	connections.mutex.Lock()
	connections.count = connections.count + 1
	connections.mutex.Unlock()

	if connections.count > 0 {
		sendMessageToLocalCons(TMessLocalInfoHide, "1")
	}
}

func showInfo() {
	connections.mutex.Lock()
	connections.count = connections.count - 1
	connections.mutex.Unlock()

	if connections.count == 0 {
		sendMessageToLocalCons(TMessLocalInfoHide, "0")
	}
}

func startVNC() {
	if len(arrayVnc) == 0 || options.ActiveVncId == -1 {
		logAdd(MessInfo, "VNC серверы отсутствуют")
		return
	}

	sendMessageToLocalCons(TMessLocalInfoClient, parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[options.ActiveVncId].Name+"_"+arrayVnc[options.ActiveVncId].Version+string(os.PathSeparator)+strings.Replace(arrayVnc[options.ActiveVncId].CmdStartClient, "%adr", options.LocalAdrVNC+":"+options.PortClientVNC, 1))

	logAdd(MessInfo, "Готовим VNC сервер для запуска")
	if !checkForAdmin() {
		logAdd(MessInfo, "У нас нет прав Администратора, запускаем обычную версию VNC сервера")

		if configVNCServerUser() {
			if installVNCServerUser() {
				go runVNCServerUser()
			}
		}
	} else {
		logAdd(MessInfo, "У нас есть права Администратора, запускаем службу для VNC сервера")

		if configVNCServer() {
			if installVNCServer() {
				go runVNCServer()
			}
		}
	}

}

func closeVNC() {
	if len(arrayVnc) == 0 || options.ActiveVncId == -1 {
		logAdd(MessInfo, "VNC серверы отсутствуют")
		return
	}

	if !checkForAdmin() {
		logAdd(MessInfo, "У нас нет прав Администратора")

		if stopVNCServerUser() {
			uninstallVNCServerUser()
		}
	} else {
		logAdd(MessInfo, "У нас есть права Администратора")

		if stopVNCServer() {
			uninstallVNCServer()
		}
	}

	//контрольный вариант завершения процессов vnc сервера
	emergencyExitVNC(options.ActiveVncId)
}

func configVNCServer() bool {
	logAdd(MessInfo, "Импортируем настройки сервера")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdConfigServer) {
		logAdd(MessError, "Не получилось импортировать настройки")
		return true //todo change to false
	}

	logAdd(MessInfo, "Импортировали настройки сервера")
	return true
}

func installVNCServer() bool {
	logAdd(MessInfo, "Устанавливаем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdInstallServer) {
		logAdd(MessError, "Не получилось установить VNC сервер")
		return true //todo change to false
	}
	logAdd(MessInfo, "Установили VNC сервер")
	return true
}

func runVNCServer() bool {
	logAdd(MessInfo, "Запускаем VNC сервер")

	_, pid := checkExistsProcess(arrayVnc[options.ActiveVncId].FileServer)
	if pid != 0 {
		logAdd(MessInfo, "VNC сервер уже запущен")
		return true
	}

	_ = os.Chdir(parentPath + VNCFolder + string(os.PathSeparator) + arrayVnc[options.ActiveVncId].Name + "_" + arrayVnc[options.ActiveVncId].Version + string(os.PathSeparator))
	if !actVNC(arrayVnc[options.ActiveVncId].CmdStartServer) {
		logAdd(MessError, "Не получилось запустить VNC сервер")
		return false
	}
	logAdd(MessInfo, "Запустился VNC сервер")
	return true
}

func stopVNCServer() bool {
	logAdd(MessInfo, "Останавливаем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdStopServer) {
		logAdd(MessError, "Не получилось установить VNC сервер")
		return true //todo change to false
	}
	logAdd(MessInfo, "Остановили VNC сервер")
	return true
}

func uninstallVNCServer() bool {
	logAdd(MessInfo, "Удаляем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdRemoveServer) {
		logAdd(MessError, "Не получилось запустить VNC сервер")
		return false
	}
	logAdd(MessInfo, "Удалили VNC сервер")
	return true
}

func configVNCServerUser() bool {
	logAdd(MessInfo, "Импортируем настройки сервера")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdConfigServerUser) {
		logAdd(MessError, "Не получилось импортировать настройки")
		return true //todo change to false
	}

	logAdd(MessInfo, "Импортировали настройки сервера")
	return true
}

func installVNCServerUser() bool {
	logAdd(MessInfo, "Устанавливаем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdInstallServerUser) {
		logAdd(MessError, "Не получилось установить VNC сервер")
		return true //todo change to false
	}
	logAdd(MessInfo, "Установили VNC сервер")
	return true
}

func runVNCServerUser() bool {
	logAdd(MessInfo, "Запускаем VNC сервер")

	_, pid := checkExistsProcess(arrayVnc[options.ActiveVncId].FileServer)
	if pid != 0 {
		logAdd(MessInfo, "VNC сервер уже запущен")
		return true
	}

	if !actVNC(arrayVnc[options.ActiveVncId].CmdStartServerUser) {
		logAdd(MessError, "Не получилось запустить VNC сервер")
		return false
	}
	logAdd(MessInfo, "Завершился VNC сервер")
	return true
}

func stopVNCServerUser() bool {
	logAdd(MessInfo, "Останавливаем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdStopServerUser) {
		logAdd(MessError, "Не получилось установить VNC сервер")
		return true //todo change to false
	}
	logAdd(MessInfo, "Остановили VNC сервер")
	return true
}

func uninstallVNCServerUser() bool {
	logAdd(MessInfo, "Удаляем VNC сервер")

	if !actVNC(arrayVnc[options.ActiveVncId].CmdRemoveServerUser) {
		logAdd(MessError, "Не получилось запустить VNC сервер")
		return false
	}
	logAdd(MessInfo, "Удалили VNC сервер")
	return true
}

func ping() {
	logAdd(MessDetail, "Запустили поток пинга")
	for true {
		time.Sleep(time.Second * WaitPing)
		sendMessage(TMessPing)
	}
	logAdd(MessDetail, "Остановили поток пинга")
}

//пир1 в сторону сервера/клиента(если напрямую)
//пир2 в сторону vnc(server/viewer)
func connectVisit(peerAdr1 string, peerAdr2 string, code string, upnp bool, mode int) {

	logAdd(MessInfo, "Запустили поток подключения трансляции")

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
		go connectClient(peerAdr1, code, &peer1, &peer2, randomString(6))
		go connectClient(peerAdr2, "", &peer2, &peer1, randomString(6))
	} else if mode == 2 {
		go connectClient(peerAdr1, code, &peer1, &peer2, randomString(6))
		go connectServer(peerAdr2, "", &peer2, &peer1, randomString(6))
	} else if mode == 3 {
		go connectServer(peerAdr1, code, &peer1, &peer2, randomString(6))
		go connectClient(peerAdr2, "", &peer2, &peer1, randomString(6))
	} else if mode == 4 {
		go connectServer(peerAdr1, code, &peer1, &peer2, randomString(6))
		go connectServer(peerAdr2, "", &peer2, &peer1, randomString(6))
	}

}

func connectServer(adr string, code string, peer1 *pConn, peer2 *pConn, id string) {
	peerBuff1 = peer1
	peerBuff2 = peer2
}

func connectClient(adr string, code string, peer1 *pConn, peer2 *pConn, id string) {

	logAdd(MessInfo, id+" запустили клиента "+code+" к "+adr)
	if len(options.Proxy) > 0 && len(code) > 0 { //если прокси указан и это не локальное подключение

		proxy.RegisterDialerType("http", newHTTPProxy)
		httpProxyURI, err := url.Parse("http://" + options.Proxy)
		if err != nil {
			sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageProxyError))
			logAdd(MessError, "mainClient не смог использовать proxy-строку: "+fmt.Sprint(err))
			return
		}

		dialer, err := proxy.FromURL(httpProxyURI, &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		})
		if err != nil {
			logAdd(MessError, id+" не смог подключиться к proxy "+fmt.Sprint(err))
			sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageProxyError))
			return
		}

		conn, err := dialer.Dial("tcp", adr)
		if err != nil {
			logAdd(MessError, id+" не смог подключиться: "+fmt.Sprint(err))
			sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageProxyError))
			return
		}

		defer conn.Close()
		peer1.Pointer = &conn
	} else {
		conn, err := net.Dial("tcp", adr)
		if err != nil {
			logAdd(MessError, id+" не удалось клиенту подключиться: "+fmt.Sprint(err))
			if len(code) > 0 {
				sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageNetworkError))
			}
			return
		}

		defer conn.Close()
		peer1.Pointer = &conn
	}

	if len(code) > 0 {
		_, _ = (*peer1.Pointer).Write([]byte(code + "\n"))
		hideInfo()
		defer showInfo()
	}

	var cWait = 0
	for peer2.Pointer == nil && cWait < WaitCount {
		logAdd(MessInfo, id+" ожидаем peer для клиента...")
		time.Sleep(time.Millisecond * WaitIdle)
		cWait++
	}
	if peer2.Pointer == nil {
		logAdd(MessError, id+" превышено время ожидания")
		if len(code) > 0 {
			sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageTimeoutError))
		}
		sendMessageToLocalCons(TMessLocalStandartAlert, fmt.Sprint(StaticMessageLocalError))
		return
	}

	sendMessageToLocalCons(TMessLocalStandartAlert, fmt.Sprint(StaticMessageLocalConn))
	logAdd(MessInfo, id+" peer существует для клиента")
	time.Sleep(time.Millisecond * WaitAfterConnect)

	var z []byte
	z = make([]byte, options.SizeBuff)

	for {
		n1, err1 := (*peer1.Pointer).Read(z)
		n2, err2 := (*peer2.Pointer).Write(z[:n1])
		if (err1 != nil && err1 != io.EOF) && err2 != nil || n1 == 0 || n2 == 0 {
			logAdd(MessInfo, id+" соединение клиента отвалилось")
			_ = (*peer2.Pointer).Close()
			break
		}
	}

	logAdd(MessInfo, id+" клиент завершил работу")
	sendMessageToLocalCons(TMessLocalStandartAlert, fmt.Sprint(StaticMessageLocalDisconn))
}
