package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/go-ps"
	"github.com/tatsushid/go-fastping"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func getMac() string {
	int, err := net.Interfaces()
	if err == nil {
		for _, i := range int {
			if (i.Flags&net.FlagLoopback == 0) && (i.Flags&net.FlagPointToPoint == 0) && (i.Flags&net.FlagUp == 1) {

				return i.HardwareAddr.String()
			}
		}
	}

	return "00:00:00:00:00:00"
}

func logAdd(TMessage int, Message string) {
	if options.FDebug && (MessFull-options.TypeLog) <= TMessage {

		if logFile == nil {
			logFile, _ = os.Create(LogName)
		}

		//todo наверное стоит убрать, но пока меашет пинг в логах
		if strings.Contains(Message, "buff (31): {\"TMessage\":18,\"Messages\":null}") || strings.Contains(Message, "{18 []}") {
			return
		}

		//todo наверное стоит убрать, нужно на время отладки
		if TMessage == MessInfo || TMessage == MessError {
			sendMessageToLocalCons(TMessLocalLog, Message)
		}

		_, _ = logFile.Write([]byte(fmt.Sprint(time.Now().Format("02 Jan 2006 15:04:05.000000")) + "\t" + messLogText[TMessage] + ":\t" + Message + "\n"))

		fmt.Println(fmt.Sprint(time.Now().Format("02 Jan 2006 15:04:05.000000")) + "\t" + messLogText[TMessage] + ":\t" + Message)
	}
}

func createMessage(TMessage int, Messages ...string) Message {
	var mes Message
	mes.TMessage = TMessage
	mes.Messages = Messages
	return mes
}

func sendMessageToSocket(conn *net.Conn, TMessage int, Messages ...string) bool {
	time.Sleep(time.Millisecond * WaitSendMess) //чисто на всякий случай, чтобы не заспамить

	if conn == nil {
		logAdd(MessDetail, "Нет подключения к сокету")
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
		_, err = (*conn).Write(out)
		if err == nil {
			return true
		}
	}
	return false
}

func sendMessageToLocalCons(TMessage int, Messages ...string) {
	//logAdd(MessDetail, "Попытка отправить сообщение в UI панель: " + fmt.Sprint(TMessage) + " " + fmt.Sprint(Messages))
	if localConnections.Front() == nil {
		//logAdd(MessDetail, "Нет запущенных UI панелей")
	}
	for e := localConnections.Front(); e != nil; e = e.Next() {
		conn := e.Value.(*net.Conn)
		sendMessageToSocket(conn, TMessage, Messages...)
	}
}

func sendMessage(TMessage int, Messages ...string) bool {
	return sendMessageToSocket(myClient.Conn, TMessage, Messages...)
}

func randomNumber(l int) string {
	var result bytes.Buffer
	var temp string
	for i := 0; i < l; {
		if fmt.Sprint(randInt(0, 9)) != temp {
			temp = fmt.Sprint(randInt(0, 9))
			result.WriteString(temp)
			i++
		}
	}
	return result.String()
}

func randomString(l int) string {
	var result bytes.Buffer
	var temp string
	for i := 0; i < l; {
		if string(randInt(65, 90)) != temp {
			temp = string(randInt(65, 90))
			result.WriteString(temp)
			i++
		}
	}
	return result.String()
}

func randInt(min int, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return min + rand.Intn(max-min)
}

func pageReplace(e []byte, a string, b string) []byte {
	return bytes.Replace(e, []byte(a), []byte(b), -1)
}

func getSHA256(str string) string {

	s := sha256.Sum256([]byte(str))
	var r string

	for _, x := range s {
		r = r + fmt.Sprintf("%02x", x)
	}

	return r
}

func saveOptions() bool {
	logAdd(MessInfo, "Пробуем сохранить настройки")

	f, err := os.Create(parentPath + OptionsFile)
	if err != nil {
		logAdd(MessError, fmt.Sprintf("Не получилось сохранить настройки: %s", err.Error()))
		return false
	}
	defer f.Close()

	buff, err := json.MarshalIndent(options, "", "\t")
	if err != nil {
		logAdd(MessError, fmt.Sprintf("Не получилось сохранить настройки: %s", err.Error()))
		return false
	}

	_, err = f.Write(buff)
	if err != nil {
		logAdd(MessError, fmt.Sprintf("Не получилось сохранить настройки: %s", err.Error()))
		return false
	}
	return true
}

func defaultOptions() bool {
	_ = os.Remove(OptionsFile)

	options = Options{
		MainServerAdr:  DefaultMainServerName,
		MainServerPort: "65471",
		DataServerAdr:  DefaultDataServerName,
		DataServerPort: "65475",
		HttpServerAdr:  DefaultHttpServerName,
		HttpServerPort: "8090",
		HttpServerType: "http",

		LocalServerAdr:  "127.0.0.1",
		LocalServerPort: "32781",

		HttpServerClientAdr:  "127.0.0.1",
		HttpServerClientPort: "8082",
		HttpServerClientType: "http",

		LocalAdrVNC:   "127.0.0.1",
		PortClientVNC: "32783",

		ProfileLogin: "",
		ProfilePass:  "",

		SizeBuff:    16000,
		FDebug:      true,
		TypeLog:     MessFull,
		ActiveVncId: -1}

	return true
}

func loadOptions() bool {
	logAdd(MessInfo, "Пробуем загрузить настройки")

	f, err := os.OpenFile(parentPath+OptionsFile, os.O_RDONLY, 0)
	if err != nil {
		logAdd(MessError, "Не получилось открыть настройки "+fmt.Sprint(err))
		return false
	}
	defer f.Close()

	buff, err := ioutil.ReadAll(f)
	if err != nil {
		logAdd(MessError, "Не получилось прочитать настройки "+fmt.Sprint(err))
		return false
	}

	err = json.Unmarshal(buff, &options)
	if err != nil {
		logAdd(MessError, "Не получилось разобрать настройки "+fmt.Sprint(err))
		return false
	}

	//if options.ActiveVncId > len(arrayVnc) - 1 {
	//	options.ActiveVncId = -1;
	//}
	return true
}

func saveListVNC() bool {
	logAdd(MessInfo, "Пробуем сохранить список VNC")

	f, err := os.Create(parentPath + VNCListFile)
	if err != nil {
		logAdd(MessError, fmt.Sprintf("Не получилось сохранить список VNC: %s", err.Error()))
		return false
	}
	defer f.Close()

	buff, err := json.MarshalIndent(arrayVnc, "", "\t")
	if err != nil {
		logAdd(MessError, fmt.Sprintf("Не получилось сохранить список VNC: %s", err.Error()))
		return false
	}

	_, err = f.Write(buff)
	if err != nil {
		logAdd(MessError, fmt.Sprintf("Не получилось сохранить список VNC: %s", err.Error()))
		return false
	}

	return true
}

func loadListVNC() bool {
	logAdd(MessInfo, "Пробуем загрузить список VNC")

	f, err := os.OpenFile(parentPath+VNCListFile, os.O_RDONLY, 0)
	if err != nil {
		logAdd(MessError, "Не получилось загрузить список VNC: "+fmt.Sprint(err))
		return false
	}
	defer f.Close()

	buff, err := ioutil.ReadAll(f)
	if err != nil {
		options.ActiveVncId = -1
		return false
	}

	err = json.Unmarshal(buff, &arrayVnc)
	if err != nil {
		logAdd(MessError, "Не получилось загрузить список VNC: "+fmt.Sprint(err))
		fmt.Println(err)
		return false
	}

	if len(arrayVnc) > 0 && options.ActiveVncId < 0 {
		options.ActiveVncId = 0
	}
	logAdd(MessInfo, "Список VNC загружен")
	return true
}

func extractZip(arch string, out string) bool {
	reader, err := zip.OpenReader(arch)
	if err != nil {
		logAdd(MessError, "Не получилось открыть архив: "+fmt.Sprint(err))
		return false
	}
	defer reader.Close()

	for _, f := range reader.Reader.File {
		zipped, err := f.Open()
		if err != nil {
			logAdd(MessError, "Не получилось открыть файл: "+fmt.Sprint(err))
			continue
		}
		defer zipped.Close()
		path := filepath.Join(out, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			writer, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, f.Mode())
			if err != nil {
				logAdd(MessError, "Не получается распаковать: "+fmt.Sprint(err))
				continue
			}
			defer writer.Close()
			if _, err = io.Copy(writer, zipped); err != nil {
				logAdd(MessError, "Не получается распаковать: "+fmt.Sprint(err))
			}
		}
	}

	logAdd(MessInfo, "Распаковка закончена")
	return true
}

func getAndExtractVNC(i int) bool {
	if i > len(arrayVnc) {
		logAdd(MessError, "Нет у нас такого VNC в списке ("+fmt.Sprint(i)+"/"+fmt.Sprint(len(arrayVnc))+")")
		return false
	}

	if i < 0 {
		i = 0
	}

	logAdd(MessError, "Собираемся получить и включить "+arrayVnc[i].Name+" "+arrayVnc[i].Version)

	resp, err := http.Get(options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort + arrayVnc[i].Link)
	if err != nil {
		logAdd(MessError, "Не получилось получить с сервера VNC: "+fmt.Sprint(err))
		return false
	}

	os.Mkdir(parentPath+VNCFolder, 0)
	os.Mkdir(parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[i].Name+"_"+arrayVnc[i].Version, 0)
	f, err := os.OpenFile(parentPath+VNCFolder+string(os.PathSeparator)+arrayVnc[i].Name+"_"+arrayVnc[i].Version+string(os.PathSeparator)+"tmp.zip", os.O_CREATE, 0)
	if err != nil {
		logAdd(MessError, "Не получилось получить с сервера VNC: "+fmt.Sprint(err))
		return false
	}
	defer f.Close()

	buff, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logAdd(MessError, "Не получилось прочитать ответ с сервера VNC: "+fmt.Sprint(err))
		return false
	}
	resp.Body.Close()

	_, err = f.Write(buff)
	if err != nil {
		logAdd(MessError, "Не получилось записать ответ с сервера VNC: "+fmt.Sprint(err))
		return false
	}

	logAdd(MessInfo, "Получили архив с "+arrayVnc[i].Name+" "+arrayVnc[i].Version)

	zip := parentPath + VNCFolder + string(os.PathSeparator) + arrayVnc[i].Name + "_" + arrayVnc[i].Version + string(os.PathSeparator) + "tmp.zip"
	out := parentPath + VNCFolder + string(os.PathSeparator) + arrayVnc[i].Name + "_" + arrayVnc[i].Version
	if extractZip(zip, out) {
		options.ActiveVncId = i
		return true
	}

	return false
}

func getListVNC() bool {
	logAdd(MessInfo, "Получим список VNC")

	resp, err := http.Get(options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort + "/api?make=listvnc")
	if err != nil {
		logAdd(MessError, "Не получилось получить с сервера VNC: "+fmt.Sprint(err))
		return false
	}

	buff, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logAdd(MessError, "Не получилось прочитать ответ с сервера VNC: "+fmt.Sprint(err))
		return false
	}
	resp.Body.Close()

	err = json.Unmarshal(buff, &arrayVnc)
	if err != nil {
		logAdd(MessError, "Не получилось получить с сервера VNC: "+fmt.Sprint(err))
		return false
	}

	if len(arrayVnc) > 0 && options.ActiveVncId < 0 {
		options.ActiveVncId = 0
	}

	logAdd(MessInfo, "Получили список VNC с сервера")
	return true
}

func actVNC(cmd string) bool {
	if len(cmd) == 0 {
		logAdd(MessDetail, "Нет команды для этого")
		return true
	}

	os.Chdir(parentPath + VNCFolder + string(os.PathSeparator) + arrayVnc[options.ActiveVncId].Name + "_" + arrayVnc[options.ActiveVncId].Version + string(os.PathSeparator))

	logAdd(MessDetail, "Выполняем "+cmd)
	str := strings.Split(cmd, " ")
	out, err := exec.Command(str[0], str[1:]...).Output()
	logAdd(MessInfo, fmt.Sprint(cmd, " result: ", out))
	if err != nil {
		logAdd(MessError, fmt.Sprint(cmd, " error: ", err))
		os.Chdir(parentPath)
		return false
	}

	os.Chdir(parentPath)
	return true
}

func checkForAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		return false
	}
	return true
}

func checkExistsProcess(name string) (bool, int) {
	p, err := ps.Processes()

	if err != nil {
		return false, 0
	}

	if len(p) <= 0 {
		return false, 0
	}

	for _, p1 := range p {
		if p1.Executable() == name {
			return true, p1.Pid()
		}
	}

	return false, 0
}

func terminateMe(term bool) {
	if localConnections.Len() > 1 && !term {
		logAdd(MessInfo, "Отказываемся выходить так как несколько ui панелей")
		return
	}

	flagTerminated = true

	sendMessageToLocalCons(TMessLocalTerminate)

	logAdd(MessInfo, "Выходим из коммуникатора")

	closeVNC()

	if logFile != nil {
		logFile.Close()
	}
	os.Exit(0)
}

func updateMe() bool {
	logAdd(MessError, "Собираемся получить актуальную версию")

	err := os.Remove(parentPath + "revisit_old.exe")
	if err != nil {
		logAdd(MessError, "Не получилось удалить старый временный файл: "+fmt.Sprint(err))
	}
	err = os.Remove(parentPath + "communicator_old.exe")
	if err != nil {
		logAdd(MessError, "Не получилось удалить старый временный файл: "+fmt.Sprint(err))
	}

	resp, err := http.Get(options.HttpServerType + "://" + options.HttpServerAdr + ":" + options.HttpServerPort + "/resource/revisit.exe")
	if err != nil || resp.StatusCode != 200 {
		logAdd(MessError, "Не получилось получить с сервера VNC: "+fmt.Sprint(err))
		return false
	}

	f, err := os.OpenFile(parentPath+"revisit_new.exe", os.O_CREATE, 0)
	if err != nil {
		logAdd(MessError, "Не получилось создать временный файл: "+fmt.Sprint(err))
		return false
	}

	buff, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logAdd(MessError, "Не получилось прочитать ответ с сервера: "+fmt.Sprint(err))
		return false
	}
	resp.Body.Close()

	_, err = f.Write(buff)
	if err != nil {
		logAdd(MessError, "Не получилось получить записать новую версию: "+fmt.Sprint(err))
		return false
	}
	f.Close()

	_, myName := filepath.Split(os.Args[0])
	err = os.Rename(parentPath+myName, parentPath+"communicator_old.exe")
	if err != nil {
		logAdd(MessError, "Не получилось получить переименновать файл: "+fmt.Sprint(err))
		return false
	}

	err = os.Rename(parentPath+"revisit.exe", parentPath+"revisit_old.exe")
	if err != nil {
		logAdd(MessError, "Не получилось получить переименновать файл: "+fmt.Sprint(err))
		err = os.Rename(parentPath+"communicator_old.exe", parentPath+myName)
		if err != nil {
			logAdd(MessError, "Не получилось получить откатить файл: "+fmt.Sprint(err))
			return false
		}
		logAdd(MessInfo, "Откатились назад")
		return false
	}

	_, err = exec.Command(parentPath+"revisit_new.exe", "-extract").Output()
	if err != nil {
		logAdd(MessError, "Не получилось распаковать коммуниктор: "+fmt.Sprint(err))
		err = os.Rename(parentPath+"communicator_old.exe", parentPath+myName)
		if err != nil {
			logAdd(MessError, "Не получилось получить откатить файл: "+fmt.Sprint(err))
			return false
		}
		err = os.Rename(parentPath+"revisit_old.exe", parentPath+"revisit.exe")
		if err != nil {
			logAdd(MessError, "Не получилось получить откатить файл: "+fmt.Sprint(err))
			return false
		}
		logAdd(MessInfo, "Откатились назад")
		return false
	}

	err = os.Rename(parentPath+"revisit_new.exe", parentPath+"revisit.exe")
	if err != nil {
		logAdd(MessError, "Не получилось переименновать новый клиент, оставим старый: "+fmt.Sprint(err))
		err = os.Rename(parentPath+"revisit_old.exe", parentPath+"revisit.exe")
		if err != nil {
			logAdd(MessError, "Не получилось получить откатить файл: "+fmt.Sprint(err))
			return false
		}
		logAdd(MessInfo, "Попробуем запуститься с новым коммуникатором")
	}

	reloadMe()

	return true
}

func reloadMe() bool {
	logAdd(MessInfo, "Перезапускаемся")

	flagReload = true
	sendMessageToLocalCons(TMessLocalReload)

	if myClient.Conn != nil {
		(*myClient.Conn).Close()
	}
	if myClient.LocalServ != nil {
		(*myClient.LocalServ).Close()
	}
	if myClient.DataServ != nil {
		(*myClient.DataServ).Close()
	}
	if myClient.WebServ != nil {
		(*myClient.WebServ).Close()
	}

	closeVNC()
	if logFile != nil {
		logFile.Close()
	}

	logAdd(MessInfo, "Запускаем новый экземпляр коммуникатора")
	os.Chdir(parentPath)
	_, myName := filepath.Split(os.Args[0])
	var sI syscall.StartupInfo
	sI.ShowWindow = 1
	var pI syscall.ProcessInformation
	argv, _ := syscall.UTF16PtrFromString(parentPath + myName)
	err := syscall.CreateProcess(
		nil,
		argv,
		nil,
		nil,
		false,
		0,
		nil,
		nil,
		&sI,
		&pI)

	if err != nil {
		flagReload = false
		logAdd(MessError, "Не получилось перезапустить коммуниктор: "+fmt.Sprint(err))
		return false
	}

	logAdd(MessInfo, "Вышли...")
	os.Exit(0)
	return true
}

func restartSystem() bool {
	out, err := exec.Command("shutdown", "-r", "-t", "0").Output()
	if err != nil {
		logAdd(MessError, "Не получилось перезапустить компьютер: "+fmt.Sprint(err))
		return false
	}
	logAdd(MessInfo, string(out))
	return true
}

func processVNC(i int) {

	//todo надо бы ещё проверить наличие сессий по vnc
	if flagReinstallVnc {
		logAdd(MessError, "Уже кто-то запустил процесс переустановки VNC")
		return
	}

	flagReinstallVnc = true //надеемся, что не будет у нас одновременных обращений

	//закроем текущую версию
	closeVNC()

	for {
		//пробуем запустить vnc когда у нас уже есть коннект до сервера, если что можем загрузить новый vnc с сервера
		if !loadListVNC() || options.ActiveVncId != i || options.ActiveVncId > len(arrayVnc)-1 {
			if getListVNC() {
				if options.ActiveVncId > len(arrayVnc)-1 {
					logAdd(MessError, "Нет такого VNC в списке")
					i = 0
				}

				if getAndExtractVNC(i) {
					logAdd(MessInfo, "Обновили VNC")
					saveOptions()
					saveListVNC()
					break
				}
				continue
			}
			continue
		}

		break
		time.Sleep(time.Second) //todo проверь почему оно у нас тут так
	}

	startVNC() //надо бы добавить проверку установлен уже или нет сервер

	flagReinstallVnc = false
}

func GetInfoOS() string {
	cmd := exec.Command("cmd", "ver")
	cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		//panic(err)
	}
	osStr := strings.Replace(out.String(), "\n", "", -1)
	osStr = strings.Replace(osStr, "\r\n", "", -1)
	tmp1 := strings.Index(osStr, "[Version")
	tmp2 := strings.Index(osStr, "]")
	var ver string
	if tmp1 == -1 || tmp2 == -1 {
		ver = "unknown"
	} else {
		ver = osStr[tmp1+9 : tmp2]
	}
	return fmt.Sprint(runtime.GOOS, " ", ver)
}

func closeProcess(name string) {
	logAdd(MessInfo, "Пробуем закрыть процесс "+name)

	p, err := ps.Processes()

	if err != nil {
		return
	}

	if len(p) <= 0 {
		return
	}

	for _, p1 := range p {

		if p1.Executable() == name && p1.Pid() != os.Getpid() {
			p, err := os.FindProcess(p1.Pid())
			//fmt.Println(p1.Executable(), p, err)
			if err == nil {
				err = p.Kill()
				if err != nil {
					logAdd(MessError, "Результат закрытия процесса "+fmt.Sprint(err))
				} else {
					logAdd(MessInfo, "Результат закрытия процесса успешный")
				}
			}
		}
	}
}

func emergencyExitVNC(i int) {
	if i < 0 || i >= len(arrayVnc) {
		logAdd(MessError, "Нет такого VNC в списке")
		return
	}

	closeProcess(arrayVnc[i].FileServer)

	closeProcess(arrayVnc[i].FileClient)
}

func closeAllVNC() {
	for i, _ := range arrayVnc {
		logAdd(MessInfo, "Пробуем закрыть "+fmt.Sprint(arrayVnc[i].Name, arrayVnc[i].Version))
		emergencyExitVNC(i)
	}
}

func controlNam(str string) int {
	i := 0

	for _, b := range str {
		i = i + int(b)
	}

	return i % 100
}

func encXOR(str1, str2 string) string {
	var result string

	cn := controlNam(str1)
	lenStr := string(len(str1))
	str1 = lenStr + str1
	flagPassword = false

	for i, b := range str1 {
		a := str2[i%len(str2)]
		c := uint8(b) ^ a
		result = result + fmt.Sprintf("%.2x", c)
	}

	result = result + fmt.Sprintf("%.2x", cn)

	if len(result) < MaxEncPass {
		salt := randomString(MaxEncPass - len(result))
		add := hex.EncodeToString([]byte(salt))
		result = result + add
	}

	return result
}

func decXOR(str1, str2 string) (string, bool) {
	var result string
	decoded, err := hex.DecodeString(str1)

	if err == nil {
		for i, b := range decoded {
			a := str2[i%len(str2)]
			c := uint8(b) ^ a
			result = result + string(c)
		}

		n := result[0] + 1
		if int(n) <= len(decoded) {
			cn := decoded[n]
			result = result[1 : result[0]+1]

			if controlNam(result) == int(cn) {
				return result, true
			}
		}
	}

	return str1, false
}

func getPass() string {

	if len(myClient.Pid) == 0 {
		//это не даст удаленной системе подключиться к нам
		return "***" + randomString(2)
	}

	if flagPassword {
		return options.Pass
	}

	if len(options.Pass) > 0 {
		pass, success := decXOR(options.Pass, myClient.Pid)
		if success == true {
			return pass
		}
	}

	logAdd(MessError, "Не получилось расшифровать пароль")

	if DefaultNumberPassword {
		options.Pass = encXOR(randomNumber(LengthPass), myClient.Pid)
	} else {
		options.Pass = encXOR(randomString(LengthPass), myClient.Pid)
	}

	logAdd(MessInfo, "Сгенерировали новый пароль")
	saveOptions()

	return getPass()
}

func sortAgents() {
	for i := 0; i < len(agents); i++ {
		for j := i; j < len(agents); j++ {
			if agents[i].Metric > agents[j].Metric && agents[j].Metric != -1 {
				tmp := agents[i]
				agents[i] = agents[j]
				agents[j] = tmp
			}
		}
	}
	printAgentsMetric()
}

func updateAgentsMetric() {
	for i := 0; i < len(agents); i++ {
		agents[i].Metric = updateAgentMetric(agents[i].Address)
	}
	logAdd(MessInfo, "Обновили метрики агентов")
}

func updateAgentMetric(address string) int {
	metric := -1
	p := fastping.NewPinger()

	ra, err := net.ResolveIPAddr("ip4:icmp", address)
	if err != nil {
		return metric
	}

	p.AddIPAddr(ra)
	p.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
		metric = int(rtt.Nanoseconds() / 1000)
	}
	p.Run()
	return metric
}

func printAgentsMetric() {
	for i := 0; i < len(agents); i++ {
		logAdd(MessDetail, "Метрика для "+fmt.Sprint(agents[i].Address, " - ", agents[i].Metric))
	}
}

func refreshAgents() {

	if chRefreshAgents == nil {
		chRefreshAgents = make(chan bool)
	}

	for {

		updateAgentsMetric()
		sortAgents()

		select {
		case <-time.After(time.Duration(WaitRefreshAgents) * time.Second):
		case <-chRefreshAgents:
		}
	}

}
