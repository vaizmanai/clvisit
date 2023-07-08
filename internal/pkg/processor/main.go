package processor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/vaizmanai/clvisit/internal/pkg/common"
	"github.com/vaizmanai/clvisit/internal/pkg/proxy"
	"io"
	"net"
	"net/url"
	"os"
	"runtime/debug"
	"time"
)

const (
	qLimit = 6
)

var (
	qChannel = make(chan bool, qLimit)
)

func takeQ() bool {
	select {
	case <-time.After(time.Second):
		return false
	case qChannel <- true:
		return true
	}
}

func backQ() {
	<-qChannel
}

func recoverMainClient(conn *net.Conn) {
	if recover() != nil {
		log.Errorf("поток mainClient поймал критическую ошибку")
		debug.PrintStack()

		if conn != nil {
			_ = (*conn).Close()
		}
	}
}

func MainClient() {

	defer recoverMainClient(myClient.Conn)

	go ping()

	for !common.Flags.Reload {
		log.Infof("mainClient пробует подключиться к %s", common.Options.ServerAddress)
		sendMessageToLocalCons(TMessLocalInfoHide, "1")

		if len(common.Options.Proxy) > 0 {
			proxy.RegisterDialerType()
			httpProxyURI, err := url.Parse(common.Options.Proxy)
			if err != nil {
				log.Errorf("mainClient не смог использовать proxy-строку: %s", err.Error())
			}

			dialer, err := proxy.FromURL(httpProxyURI, &net.Dialer{
				Timeout:   common.HttpTimeout * time.Second,
				KeepAlive: common.HttpTimeout * time.Second,
			})
			if err != nil {
				log.Errorf("mainClient не смог подключиться к proxy: %s", err.Error())
				time.Sleep(time.Second)
				continue
			}

			conn, err := dialer.Dial("tcp", common.GetMainServerAddress())
			if err != nil {
				log.Errorf("mainClient не смог подключиться: %s", err.Error())
				time.Sleep(time.Second)
				continue
			}
			myClient.Conn = &conn
		} else {
			conn, err := net.Dial("tcp", common.GetMainServerAddress())
			if err != nil {
				log.Errorf("mainClient не смог подключиться: %s", err.Error())
				time.Sleep(time.Second)
				continue
			}
			myClient.Conn = &conn
		}

		//отправим свою версию
		myClient.Version = common.RevisitVersion
		myClient.Name = common.WhiteLabelName
		sendMessage(TMessVersion, myClient.Version)

		//отправим свой идентификатор
		myClient.Serial = common.GetMac()
		sendMessage(TMessAuth, myClient.Serial)

		sendMessage(TMessServers)
		sendMessageToLocalCons(TMessLocalInfoHide, "0")

		reader := bufio.NewReader(*myClient.Conn)

		for {
			buff, err := reader.ReadBytes('}')

			if err != nil {
				log.Errorf("mainClient ошибка чтения буфера: %s", err.Error())
				break
			}

			log.Debugf("buff (%d): %s", len(buff), string(buff))

			//удаляем мусор
			if buff[0] != '{' {
				log.Infof("mainServer удаляем мусор")
				if bytes.Index(buff, []byte("{")) >= 0 {
					log.Debugf("buff (%d): %s", len(buff), string(buff))
					buff = buff[bytes.Index(buff, []byte("{")):]
				} else {
					continue
				}
			}

			var message Message
			err = json.Unmarshal(buff, &message)
			if err != nil {
				log.Errorf("mainClient ошибка разбора json: %s", err.Error())
				time.Sleep(time.Millisecond * common.WaitIdle)
				continue
			}

			//log.Debugf("%+v", message)

			//обрабатываем полученное сообщение
			if len(processing) > message.TMessage {
				if processing[message.TMessage].Processing != nil {
					if takeQ() {
						go func() {
							processing[message.TMessage].Processing(message, myClient.Conn, context.Background())
							backQ()
						}()
					}
				} else {
					log.Infof("mainClient нет обработчика для сообщения")
					time.Sleep(time.Millisecond * common.WaitIdle)
				}
			} else {
				log.Infof("mainClient неизвестное сообщение")
				time.Sleep(time.Millisecond * common.WaitIdle)
			}

		}

		sendMessageToLocalCons(TMessLocalLogout)

		log.Infof("mainClient остановился")
		_ = (*myClient.Conn).Close()
		myClient.Conn = nil

		time.Sleep(time.Second * 1)
	}
}

func Thread() {
	count := 0
	for count < common.WaitCountRestartSrv && !common.Flags.Reload {
		log.Infof("localServer запустился")

		ln, err := net.Listen("tcp", common.GetLocalServerAddress())
		if err != nil {
			log.Errorf("localServer не смог занять порт: %s", err.Error())
			panic(err.Error())
		}

		myClient.LocalServ = &ln

		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Errorf("localServer не смог занять сокет: %s", err.Error())
				break
			}
			go localHandler(&conn)
		}

		_ = ln.Close()
		log.Infof("localServer остановился")
		time.Sleep(time.Millisecond * common.WaitIdle)
		count++
	}

	if !common.Flags.Reload {
		log.Errorf("localServer так и не смог запуститься")
		os.Exit(1)
	}
}

func localHandler(conn *net.Conn) {
	id := common.RandomString(6)
	log.Infof("%s localServer получил соединение", id)

	item := localConnections.PushBack(conn)

	processLocalInfo(createMessage(TMessLocalInfo), conn, context.Background())
	processLocalInfoClient(createMessage(TMessLocalInfoClient), conn, context.Background())
	if len(common.Options.OptionsUI) > 0 {
		processLocalOptionsUI(createMessage(TMessLocalOptionsUi), conn, context.Background())
	}

	reader := bufio.NewReader(*conn)

	for {
		buff, err := reader.ReadBytes('}')

		if err != nil {
			log.Errorf("%s localServer ошибка чтения буфера: %s", id, err.Error())
			break
		}

		log.Debugf("%s buff (%d): %s", id, len(buff), string(buff))

		//удаляем мусор
		if buff[0] != '{' {
			log.Infof("%s localServer удаляем мусор", id)
			if bytes.Index(buff, []byte("{")) >= 0 {
				buff = buff[bytes.Index(buff, []byte("{")):]
			} else {
				continue
			}
		}

		var message Message
		err = json.Unmarshal(buff, &message)
		if err != nil {
			log.Errorf("%s localServer ошибка разбора json: %s", id, err.Error())
			time.Sleep(time.Millisecond * common.WaitIdle)
			continue
		}

		log.Debugf("%s %+v", id, message)

		//обрабатываем полученное сообщение
		if len(localProcessing) > message.TMessage {
			if localProcessing[message.TMessage].Processing != nil {
				if takeQ() {
					go func() {
						localProcessing[message.TMessage].Processing(message, conn, context.Background())
						backQ()
					}()
				}
			} else {
				log.Infof("%s localServer нет обработчика для сообщения", id)
				time.Sleep(time.Millisecond * common.WaitIdle)
			}
		} else {
			log.Infof("%s localServer неизвестное сообщение", id)
			time.Sleep(time.Millisecond * common.WaitIdle)
		}

	}

	localConnections.Remove(item)
	_ = (*conn).Close()
	log.Infof("%s localServer потерял соединение", id)
}

func DataThread() {
	count := 0
	for count < common.WaitCountRestartSrv && !common.Flags.Reload {
		log.Infof("localDataServer запустился")

		ln, err := net.Listen("tcp", common.Options.LocalAdrVNC+":"+common.Options.PortClientVNC)
		if err != nil {
			log.Errorf("localDataServer не смог занять порт: %s", err.Error())
			panic(err.Error())
		}

		myClient.DataServ = &ln

		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Errorf("localDataServer не смог занять сокет: %s", err.Error())
				break
			}
			go localDataHandler(&conn)
		}

		_ = ln.Close()
		log.Infof("localDataServer остановился")
		time.Sleep(time.Millisecond * common.WaitIdle)
		count++
	}

	if !common.Flags.Reload {
		log.Errorf("localDataServer так и не смог запуститься")
		os.Exit(1)
	}
}

func localDataHandler(conn *net.Conn) {
	id := common.RandomString(6)
	log.Infof("%s localDataHandler получил соединение", id)

	var cWait = 0
	for (peerBuff1 == nil || peerBuff2 == nil) && cWait < common.WaitCount {
		log.Infof("%s ожидание peer для локального сервера...", id)
		time.Sleep(time.Millisecond * common.WaitIdle)
		cWait++
	}

	if peerBuff1 == nil || peerBuff2 == nil {
		_ = (*conn).Close()
		log.Infof("%s не дождались peer", id)
		return
	}

	peer1 := peerBuff1
	peer2 := peerBuff2

	peerBuff1 = nil
	peerBuff2 = nil

	peer1.Pointer = conn

	cWait = 0
	for peer2.Pointer == nil && cWait < common.WaitCount && !common.Flags.Terminated {
		log.Infof("%s ожидание peer для локального сервера...", id)
		time.Sleep(time.Millisecond * common.WaitIdle)
		cWait++
	}

	if peer2.Pointer == nil {
		_ = (*conn).Close()
		log.Infof("%s не дождались peer", id)
		return
	}

	log.Infof("%s peer существует для локального сервера", id)
	time.Sleep(time.Millisecond * common.WaitAfterConnect)

	var z []byte
	z = make([]byte, common.Options.SizeBuff)

	for {
		n1, err1 := (*conn).Read(z)
		//fmt.Println(id, "server:", z[:n1], err1)
		n2, err2 := (*peer2.Pointer).Write(z[:n1])
		//fmt.Println(id, "server:", z[:n2], err2)
		if (err1 != nil && err1 != io.EOF) && err2 != nil || n1 == 0 || n2 == 0 {
			log.Infof("%s подключение к локальному серверу отвалилось: %d, %d", id, n1, n2)
			_ = (*peer2.Pointer).Close()
			break
		}
	}

	_ = (*conn).Close()
	log.Infof("%s подключение к локальному серверу завершило работу", id)
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

func ping() {
	log.Debugf("запустили поток пинга")
	for true {
		time.Sleep(time.Second * common.WaitPing)
		sendMessage(TMessPing)
	}
	log.Debugf("остановили поток пинга")
}

// пир1 в сторону сервера/клиента(если напрямую)
// пир2 в сторону vnc(server/viewer)
func connectVisit(peerAdr1 string, peerAdr2 string, code string, upnp bool, mode int) {
	log.Infof("запустили поток подключения трансляции")

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
		go connectClient(peerAdr1, code, &peer1, &peer2, common.RandomString(6))
		go connectClient(peerAdr2, "", &peer2, &peer1, common.RandomString(6))
	} else if mode == 2 {
		go connectClient(peerAdr1, code, &peer1, &peer2, common.RandomString(6))
		go connectServer(peerAdr2, "", &peer2, &peer1, common.RandomString(6))
	} else if mode == 3 {
		go connectServer(peerAdr1, code, &peer1, &peer2, common.RandomString(6))
		go connectClient(peerAdr2, "", &peer2, &peer1, common.RandomString(6))
	} else if mode == 4 {
		go connectServer(peerAdr1, code, &peer1, &peer2, common.RandomString(6))
		go connectServer(peerAdr2, "", &peer2, &peer1, common.RandomString(6))
	}
}

func connectServer(adr string, code string, peer1 *pConn, peer2 *pConn, id string) {
	peerBuff1 = peer1
	peerBuff2 = peer2
}

func connectClient(adr string, code string, peer1 *pConn, peer2 *pConn, id string) {
	log.Infof("%s запустили клиента %s к %s", id, code, adr)

	if len(common.Options.Proxy) > 0 && len(code) > 0 { //если прокси указан и это не локальное подключение
		proxy.RegisterDialerType()
		httpProxyURI, err := url.Parse(common.Options.Proxy)
		if err != nil {
			sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageProxyError))
			log.Errorf("mainClient не смог использовать proxy-строку: %s", err.Error())
			return
		}

		dialer, err := proxy.FromURL(httpProxyURI, &net.Dialer{
			Timeout:   common.HttpTimeout * time.Second,
			KeepAlive: common.HttpTimeout * time.Second,
		})
		if err != nil {
			log.Errorf("%s не смог подключиться к proxy: %s", id, err.Error())
			sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageProxyError))
			return
		}

		conn, err := dialer.Dial("tcp", adr)
		if err != nil {
			log.Errorf("%s не смог подключиться: %s", id, err.Error())
			sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageProxyError))
			return
		}

		defer conn.Close()
		peer1.Pointer = &conn
	} else {
		conn, err := net.Dial("tcp", adr)
		if err != nil {
			log.Errorf("%s не удалось клиенту подключиться: %s", id, err.Error())
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
	for peer2.Pointer == nil && cWait < common.WaitCount {
		log.Infof("%s ожидаем peer для клиента...", id)
		time.Sleep(time.Millisecond * common.WaitIdle)
		cWait++
	}
	if peer2.Pointer == nil {
		log.Errorf("%s превышено время ожидания", id)
		if len(code) > 0 {
			sendMessage(TMessDisconnect, code, fmt.Sprint(StaticMessageTimeoutError))
		}
		sendMessageToLocalCons(TMessLocalStandardAlert, fmt.Sprint(StaticMessageLocalError))
		return
	}

	sendMessageToLocalCons(TMessLocalStandardAlert, fmt.Sprint(StaticMessageLocalConn))
	log.Infof("%s peer существует для клиента", id)
	time.Sleep(time.Millisecond * common.WaitAfterConnect)

	z := make([]byte, common.Options.SizeBuff)

	for {
		n1, err1 := (*peer1.Pointer).Read(z)
		n2, err2 := (*peer2.Pointer).Write(z[:n1])
		if (err1 != nil && err1 != io.EOF) && err2 != nil || n1 == 0 || n2 == 0 {
			log.Infof("%s соединение клиента отвалилось", id)
			_ = (*peer2.Pointer).Close()
			break
		}
	}

	log.Infof("%s клиент завершил работу", id)
	sendMessageToLocalCons(TMessLocalStandardAlert, fmt.Sprint(StaticMessageLocalDisconn))
}
