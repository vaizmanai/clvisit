package common

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/go-ps"
	log "github.com/sirupsen/logrus"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	httpClient   *http.Client
	alertMessage = make(chan string)
)

func init() {
	httpClient = &http.Client{
		Timeout: HttpTimeout * time.Second,
	}

	parentPath, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	parentPath = fmt.Sprintf("%s%s", parentPath, string(os.PathSeparator))
	_ = os.Chdir(parentPath)
	log.Debugf("текущая папка: %s", parentPath)

	log.SetFormatter(&log.TextFormatter{
		DisableQuote: true,
	})
	log.SetLevel(log.InfoLevel)
	logFile, _ = os.OpenFile(logName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
	log.SetOutput(io.MultiWriter(logFile, os.Stdout))
}

func controlNum(str string) int {
	i := 0

	for _, b := range str {
		i = i + int(b)
	}

	return i % 100
}

func ReOpenLogFile() {
	logFile, _ = os.OpenFile(logName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	log.SetOutput(io.MultiWriter(logFile, os.Stdout))
	log.Infof("truncate log file")
}

func CloseLogFile() {
	log.SetOutput(os.Stdout)

	if logFile != nil {
		if err := logFile.Close(); err != nil {
			log.Warnf("closing logs: %s", err.Error())
		}
	}

	logFile = nil
}

func RotateLogFiles() {
	fs, err := os.Stat(logName)
	if err != nil {
		return
	}

	if fs.Size() > maxLogFileMb*1024*1024 {
		CloseLogFile()

		if err = os.Rename(logName, logName+".old"); err != nil {
			return
		}

		ReOpenLogFile()
	}
}

func GetHttpClient() *http.Client {
	return httpClient
}

func GetMac() string {
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, i := range interfaces {
			if (i.Flags&net.FlagLoopback == 0) && (i.Flags&net.FlagPointToPoint == 0) && (i.Flags&net.FlagUp == 1) {
				return i.HardwareAddr.String()
			}
		}
	}
	return "00:00:00:00:00:00"
}

func GetMainServerAddress() string {
	return fmt.Sprintf("%s:%s", Options.ServerAddress, Options.MainServerPort)
}

func GetHttpServerAddress() string {
	return fmt.Sprintf("%s://%s:%s", Options.HttpServerType, Options.ServerAddress, Options.HttpServerPort)
}

func GetLocalHttpServerAddress() string {
	return fmt.Sprintf("%s://%s:%s", Options.HttpServerClientType, Options.HttpServerClientAdr, Options.HttpServerClientPort)
}

func GetLocalServerAddress() string {
	return fmt.Sprintf("%s:%s", Options.LocalServerAdr, Options.LocalServerPort)
}

func GetVNCAddress() string {
	return fmt.Sprintf("%s:%s", Options.LocalAdrVNC, Options.PortClientVNC)
}

func SetDefaultOptions() {
	Options = options{
		ServerAddress:  DefaultMainServerName,
		MainServerPort: "65471",
		DataServerPort: "65475",
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
		LogLevel:    log.DebugLevel,
		ActiveVncId: -1,
	}
	_ = os.Remove(optionsFile)
}

func RandomNumber(l int) string {
	var result bytes.Buffer
	var temp string
	for i := 0; i < l; {
		if fmt.Sprint(RandInt(0, 9)) != temp {
			temp = fmt.Sprint(RandInt(0, 9))
			result.WriteString(temp)
			i++
		}
	}
	return result.String()
}

func RandomString(l int) string {
	var result bytes.Buffer
	var temp string
	for i := 0; i < l; {
		if string(rune(RandInt(65, 90))) != temp {
			temp = string(rune(RandInt(65, 90)))
			result.WriteString(temp)
			i++
		}
	}
	return result.String()
}

func RandInt(min int, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return min + rand.Intn(max-min)
}

func GetSHA256(str string) string {
	s := sha256.Sum256([]byte(str))
	var r string

	for _, x := range s {
		r = r + fmt.Sprintf("%02x", x)
	}

	return r
}

func SaveOptions() bool {
	log.Infof("пробуем сохранить настройки")

	f, err := os.Create(fmt.Sprintf("%s%s", parentPath, optionsFile))
	if err != nil {
		log.Errorf("не получилось сохранить настройки: %s", err.Error())
		return false
	}
	defer f.Close()

	buff, err := json.MarshalIndent(Options, "", "\t")
	if err != nil {
		log.Errorf("не получилось сохранить настройки: %s", err.Error())
		return false
	}

	n, err := f.Write(buff)
	if err != nil || n != len(buff) {
		log.Errorf("не получилось сохранить настройки")
		return false
	}
	return true
}

func LoadOptions() error {
	if buff, err := os.ReadFile(fmt.Sprintf("%s%s", parentPath, optionsFile)); err != nil {
		return err
	} else if err = json.Unmarshal(buff, &Options); err != nil {
		return err
	}
	log.SetLevel(Options.LogLevel)
	return nil
}

func ExtractZip(arch string, out string) bool {
	reader, err := zip.OpenReader(arch)
	if err != nil {
		log.Errorf("не получилось открыть архив: %s", err.Error())
		return false
	}
	defer reader.Close()

	for _, f := range reader.Reader.File {
		zipped, err := f.Open()
		if err != nil {
			log.Errorf("не получилось открыть файл: %s", err.Error())
			continue
		}
		path := filepath.Join(out, f.Name)
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(path, os.ModePerm)
		} else {
			writer, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, f.Mode())
			if err != nil {
				log.Errorf("не получается распаковать: %s", err.Error())
				continue
			}
			if _, err = io.Copy(writer, zipped); err != nil {
				log.Errorf("не получается распаковать: %s", err.Error())
			}
			_ = writer.Close()
		}
		_ = zipped.Close()
	}

	log.Infof("распаковка закончена")
	return true
}

func CheckForAdmin() bool {
	if _, err := os.Open("\\\\.\\PHYSICALDRIVE0"); err != nil {
		return false
	}
	return true
}

func CheckExistsProcess(name string) (bool, int) {
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

func RestartSystem() bool {
	out, err := exec.Command("shutdown", "-r", "-t", "0").Output()
	if err != nil {
		log.Errorf("не получилось перезапустить компьютер: %s", err.Error())
		return false
	}
	log.Infof(string(out))
	return true
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

func CloseProcess(name string) {
	log.Infof("пробуем закрыть процесс %s", name)

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
					log.Errorf("результат закрытия процесса: %s", err.Error())
				} else {
					log.Infof("результат закрытия процесса успешный")
				}
			}
		}
	}
}

func EncXOR(str1, str2 string) string {
	var result string

	cn := controlNum(str1)
	lenStr := string(len(str1))
	str1 = lenStr + str1
	Flags.Password = false

	for i, b := range str1 {
		a := str2[i%len(str2)]
		c := uint8(b) ^ a
		result = result + fmt.Sprintf("%.2x", c)
	}

	result = result + fmt.Sprintf("%.2x", cn)

	if len(result) < MaxEncPass {
		salt := RandomString(MaxEncPass - len(result))
		add := hex.EncodeToString([]byte(salt))
		result = result + add
	}

	return result
}

func DecXOR(str1, str2 string) (string, bool) {
	var result string
	decoded, err := hex.DecodeString(str1)

	if err == nil {
		for i, b := range decoded {
			a := str2[i%len(str2)]
			c := b ^ a
			result = result + string(c)
		}

		n := result[0] + 1
		if int(n) <= len(decoded) {
			cn := decoded[n]
			result = result[1 : result[0]+1]

			if controlNum(result) == int(cn) {
				return result, true
			}
		}
	}

	return str1, false
}

func GetParentFolder() string {
	return parentPath
}

func Close() {
	_, myName := filepath.Split(os.Args[0])
	CloseProcess(myName)
	CloseProcess(WhiteLabelFileName)
}

func Reload() {
	_, myName := filepath.Split(os.Args[0])
	CloseProcess(myName)
	Options.ActiveVncId = -1
}

func Clean() {
	_, myName := filepath.Split(os.Args[0])
	CloseProcess(myName)
	CloseProcess(WhiteLabelFileName)
	SetDefaultOptions()
}

func SetAlert(message string) {
	for {
		select {
		case alertMessage <- message:
		default:
			return
		}
	}
}

func GetAlert() string {
	select {
	case m := <-alertMessage:
		return m
	case <-time.After(time.Second):
		return ""
	}
}
