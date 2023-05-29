package dao

import (
	"Traceroute/config"
	"Traceroute/dataStore/measureChange"
	"Traceroute/mylog"
	"Traceroute/state"
	"Traceroute/statisticsAnalyse"
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pkg/sftp"
	"github.com/xormplus/xorm"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var logger = mylog.GetLogger()
var RetryTime = 0
var InsertTableNums = 0
var Engine *xorm.Engine
var UploadSeconds int64
var BJValue = make([]string, 22)
var sN = make([]int, 22)
var session *xorm.Session
var Detail string
var DetailLock sync.Mutex
var SshBuild int64
var SftpBuild int64
var SshBuildFailTimes int
var SftpBuildFailTimes int
var UploadBuild int64
var UploadBuildFailTimes int
var Upload int64
var UploadFailTimes int
var FileSize int
var RestTempTime int64
var ETH int
var BeginTransferBytes int64
var AfterTransferBytes int64

func GetNetDev() (bytes, packet []int64) {
	str := state.GetCmdOut("cat /proc/net/dev")
	for i := 2; i < len(str); i++ {
		str2 := strings.Fields(str[i])
		if len(str2) > 0 {
			b, err := strconv.ParseInt(str2[9], 10, 64)
			if err != nil {
				return nil, nil
			}
			bytes = append(bytes, b)
			b, err = strconv.ParseInt(str2[10], 10, 64)
			if err != nil {
				return nil, nil
			}
			packet = append(packet, b)
		}
	}
	return
}

func CountNetRate(bytes1, packet1, bytes2, packet2 []int64, second int64) (resBytes, resPacket int64) {
	if len(bytes1) == 0 {
		return 0, 0
	}
	var Cha int64
	Cha = 0
	resIndex := 0
	for i := 0; i < len(bytes1); i++ {
		if bytes2[i]-bytes1[i] > Cha {
			resIndex = i
		}
	}
	ETH = resIndex
	return (bytes2[resIndex] - bytes1[resIndex]) / second, (packet2[resIndex] - packet1[resIndex]) / second
}
func NetRate() (resBytes, resPacket int64) {
	bytes1, packet1 := GetNetDev()
	time.Sleep(time.Second * 10)
	bytes2, packet2 := GetNetDev()
	resBytes, resPacket = CountNetRate(bytes1, packet1, bytes2, packet2, 10)
	return
}

func BeginTransferNetBytes() {
	bytes1, _ := GetNetDev()
	BeginTransferBytes = bytes1[ETH]
}
func AfterTransferNetBytes() {
	bytes1, _ := GetNetDev()
	AfterTransferBytes = bytes1[ETH]
}

// 连接的配置
type ClientConfig struct {
	Host       string       //ip
	Port       int64        // 端口
	Username   string       //用户名
	Password   string       //密码
	sshClient  *ssh.Client  //ssh client
	sftpClient *sftp.Client //sftp client
	LastResult string       //最近一次运行的结果
}

func (cliConf *ClientConfig) CreateClient(host string, port int64, username, password string, rest bool) (int, error) {
	if !rest {
		SshBuild = -1
		SftpBuild = -1
	}
	var (
		sshClient  *ssh.Client
		sftpClient *sftp.Client
		err        error
	)
	cliConf.Host = host
	cliConf.Port = port
	cliConf.Username = username
	cliConf.Password = password

	config1 := ssh.ClientConfig{
		User: cliConf.Username,
		Auth: []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: 10 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", cliConf.Host, cliConf.Port)
	if !rest {
		SshBuild = time.Now().UnixNano()
	} else {
		RestTempTime = time.Now().UnixNano()
	}
	if sshClient, err = ssh.Dial("tcp", addr, &config1); err != nil {
		if !rest {
			Detail += "SshBuildFail:" + strconv.FormatInt(time.Now().UnixNano()-SshBuild, 10) + ";"
			SshBuild = -1
		} else {
			Detail += "SshBuildFail:" + strconv.FormatInt(time.Now().UnixNano()-RestTempTime, 10) + ";"
		}
		return 1, err
	}
	if !rest {
		SshBuild = time.Now().UnixNano() - SshBuild
	} else {
		RestTempTime = time.Now().UnixNano() - RestTempTime
		Detail += "SshBuild:" + strconv.FormatInt(RestTempTime, 10) + ";"
	}
	cliConf.sshClient = sshClient

	//此时获取了sshClient，下面使用sshClient构建sftpClient
	if !rest {
		SftpBuild = time.Now().UnixNano()
	} else {
		RestTempTime = time.Now().UnixNano()
	}
	if sftpClient, err = sftp.NewClient(sshClient); err != nil {
		if !rest {
			Detail += "SftpBuildFail:" + strconv.FormatInt(time.Now().UnixNano()-SftpBuild, 10) + ";"
			SftpBuild = -1
		} else {
			RestTempTime = time.Now().UnixNano() - RestTempTime
			Detail += "SftpBuildFail:" + strconv.FormatInt(RestTempTime, 10) + ";"
		}
		return 2, err
	}
	if !rest {
		SftpBuild = time.Now().UnixNano() - SftpBuild
	} else {
		RestTempTime = time.Now().UnixNano() - RestTempTime
		Detail += "SftpBuild:" + strconv.FormatInt(RestTempTime, 10) + ";"
	}
	cliConf.sftpClient = sftpClient
	return 0, nil
}

func (cliConf *ClientConfig) RunShell(shell string) string {
	var (
		session *ssh.Session
		err     error
	)

	//获取session，这个session是用来远程执行操作的
	if session, err = cliConf.sshClient.NewSession(); err != nil {
		log.Fatalln("error occurred:", err)
	}
	//执行shell
	if output, err := session.CombinedOutput(shell); err != nil {
		fmt.Println(shell)
		log.Fatalln("error occurred:", err)
	} else {
		cliConf.LastResult = string(output)
	}
	return cliConf.LastResult
}

func (cliConf *ClientConfig) Upload(srcPath, dstPath string, rest bool) (int, error) {
	srcFile, err := os.Open(srcPath) //本地
	if err != nil {
		return 3, err
	}
	if rest {
		RestTempTime = time.Now().UnixNano()
	} else {
		UploadBuild = time.Now().UnixNano()
	}
	dstFile, err := cliConf.sftpClient.Create(dstPath) //远程
	if err != nil {
		if rest {
			RestTempTime = time.Now().UnixNano() - RestTempTime
			Detail += "UploadBuildFail:" + strconv.FormatInt(RestTempTime, 10) + ";"
		} else {
			Detail += "UploadBuildFail:" + strconv.FormatInt(time.Now().UnixNano()-UploadBuild, 10) + ";"
			UploadBuild = -1
		}
		return 1, err
	}
	if rest {
		RestTempTime = time.Now().UnixNano() - RestTempTime
		Detail += "UploadBuild:" + strconv.FormatInt(RestTempTime, 10) + ";"
	} else {
		UploadBuild = time.Now().UnixNano() - UploadBuild
	}
	defer func() {
		_ = srcFile.Close()
		_ = dstFile.Close()
	}()
	buf := make([]byte, 50000)
	if rest {
		RestTempTime = time.Now().UnixNano()
	} else {
		Upload = time.Now().UnixNano()
		BeginTransferNetBytes()
	}
	for {
		n, err := srcFile.Read(buf)
		if err != nil {
			if err != io.EOF {
				return 2, err
			} else {
				break
			}
		}
		_, err = dstFile.Write(buf[:n])
		if err != nil {
			if rest {
				RestTempTime = time.Now().UnixNano() - RestTempTime
				Detail += "UploadFail:" + strconv.FormatInt(RestTempTime, 10) + ";"
			} else {
				Detail += "UploadFail:" + strconv.FormatInt(time.Now().UnixNano()-Upload, 10) + ";"
			}
			return 2, err
		}
	}
	if rest {
		RestTempTime = time.Now().UnixNano() - RestTempTime
		Detail += "Upload:" + strconv.FormatInt(RestTempTime, 10) + ";"
	} else {
		AfterTransferNetBytes()
		Upload = time.Now().UnixNano() - Upload
		Detail += "BeginTransferBytes:" + strconv.FormatInt(BeginTransferBytes, 10) + ";"
		Detail += "AfterTransferBytes:" + strconv.FormatInt(AfterTransferBytes, 10) + ";"
	}
	return 0, nil
	//fmt.Println(cliConf.RunShell(fmt.Sprintf("ls %s", dstPath)))
}

func (cliConf *ClientConfig) Download(srcPath, dstPath string) {
	srcFile, err := cliConf.sftpClient.Open(srcPath) //远程
	fmt.Println(srcFile)
	if err != nil {
		fmt.Println(err.Error())
	}

	dstFile, _ := os.Create(dstPath) //本地
	defer func() {
		_ = srcFile.Close()
		_ = dstFile.Close()
	}()

	if _, err := srcFile.WriteTo(dstFile); err != nil {
		log.Fatalln("error occurred", err)
	}
	fmt.Println("文件下载完毕")
}

// 进行zlib压缩
func DoZlibCompress(src []byte) []byte {
	var in bytes.Buffer
	w := zlib.NewWriter(&in)
	w.Write(src)
	w.Close()
	return in.Bytes()
}

// 进行zlib解压缩
func DoZlibUnCompress(compressSrc []byte) []byte {
	b := bytes.NewReader(compressSrc)
	var out bytes.Buffer
	r, _ := zlib.NewReader(b)
	io.Copy(&out, r)
	return out.Bytes()
}
func StoreJsonResult(curTime int64, hostName string, desIp string, desPort int64, desPass string) error {
	dataFile := "./data/" + strconv.FormatInt(curTime, 10) + ".json.zlib"
	//初始化SSH连接配置
	cliConf := new(ClientConfig)
	//创建连接
	status, err := cliConf.CreateClient(desIp, desPort, "root", desPass, false)
	//连接失败进行重试直到成功
	for status != 0 {
		if status == 1 {
			SshBuildFailTimes++
		} else {
			SftpBuildFailTimes++
		}
		status, err = cliConf.CreateClient(desIp, desPort, "root", desPass, false)
	}
	defer func(sshClient *ssh.Client) {
		err := sshClient.Close()
		if err != nil {
			fmt.Println("StoreJsonResult sshClient.Close()", err)
		} else {
			fmt.Println("StoreJsonResult sshClient.Close()", err)
		}
	}(cliConf.sshClient)
	defer func(sftpClient *sftp.Client) {
		err := sftpClient.Close()
		if err != nil {
			fmt.Println("StoreJsonResult sshClient.Close()", err)
		} else {
			fmt.Println("StoreJsonResult sshClient.Close()", err)
		}
	}(cliConf.sftpClient)
	//连接成功后开始文件传输
	status, err = cliConf.Upload(dataFile, "/home/600G_storage/data/"+hostName+"_"+strconv.FormatInt(curTime, 10)+".json.zlib", false)
	for status != 0 {
		//传输超时时进行失败重试
		if status == 3 {
			return err
		} else if status == 1 {
			UploadBuildFailTimes++
		} else if status == 2 {
			UploadFailTimes++
		}
		status, err = cliConf.Upload(dataFile, "/home/600G_storage/data/"+hostName+"_"+strconv.FormatInt(curTime, 10)+".json.zlib", false)
	}
	if err != nil {
		return err
	}
	//传输完成后移除本地文件
	err = os.Remove(dataFile)
	return err
}

func TransferRestFile(hostName string) int {
	baseDir := "/root/falcon/plugin/net-plugin/data"
	if runtime.GOOS == "windows" {
		baseDir = "./data"
	}
	fNames := make([]string, 0)
	filepath.Walk(baseDir, func(fname string, fi os.FileInfo, err error) error {
		if !fi.IsDir() {
			fNames = append(fNames, fname)
		}
		return nil
	})
	if len(fNames) == 0 {
		return 0
	}

	cliConf := new(ClientConfig)
	status, err := cliConf.CreateClient("106.3.133.5", 60708, "root", "1000lgf,wchql", true)
	for status != 0 {
		status, err = cliConf.CreateClient("106.3.133.5", 60708, "root", "1000lgf,wchql", true)
	}
	for _, fname := range fNames {
		dataFile := fname
		t := strings.Split(strings.Split(fname, "/")[len(strings.Split(fname, "/"))-1], ".")
		status, err = cliConf.Upload(dataFile, "/home/600G_storage/data/"+hostName+"_"+t[0]+".json.zlib", true)
		for status != 0 {
			if status == 3 {
				return -1
			}
			status, err = cliConf.Upload(dataFile, "/home/600G_storage/data/"+hostName+"_"+t[0]+".json.zlib", true)
		}
		if err != nil {
			return -1
		}
		err = os.Remove(dataFile)
		Detail += "UploadFinish:" + t[0] + ";"
	}
	return len(fNames)
}
func BirthDBWithTimeout(conf *config.Config) {
	done := make(chan struct{}, 1)
	go func() {
		args := fmt.Sprintf("%s:%s@%s(%s)/%s?charset=utf8&parseTime=true&loc=Local", conf.Data.MysqlUser, conf.Data.MysqlPassWord,
			"tcp", conf.Data.MysqlAddress, conf.Data.Database)
		Engine, _ = xorm.NewMySQL(xorm.MYSQL_DRIVER, args)
		done <- struct{}{}
	}()

	select {
	case <-done:
		logger.Info(fmt.Sprintf("Make\tmysql Engine done\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return
	case <-time.After(5 * time.Second): //设置10s超时，经过测试机器正常情况10s内就能创建Engine，10s都没完成的话肯定是哪里出了问题，重来吧
		RetryTime++
		logger.Error(fmt.Sprintf("Make\tmysql Engine fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return
	}
}
func BirthSessionWithTimeout() {
	if session != nil {
		return
	}
	done := make(chan struct{}, 1)
	go func() {
		session = Engine.NewSession()
		session.Begin()
		done <- struct{}{}
	}()

	select {
	case <-done:
		logger.Info(fmt.Sprintf("Make\tmysql Engine done\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return
	case <-time.After(10 * time.Second): //设置10s超时，经过测试机器正常情况10s内就能创建Engine，10s都没完成的话肯定是哪里出了问题，重来吧
		RetryTime++
		logger.Error(fmt.Sprintf("Make\tmysql Engine fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return
	}
}
func BirthDB(conf *config.Config) (err error) {
	//if !conf.Data.UseDB {
	//	//注意：这是使用配置文件的数据库地址
	args := fmt.Sprintf("%s:%s@%s(%s)/%s?charset=utf8&parseTime=true&loc=Local", conf.Data.MysqlUser,
		conf.Data.MysqlPassWord, "tcp", conf.Data.MysqlAddress, conf.Data.Database)
	Engine, err = xorm.NewMySQL(xorm.MYSQL_DRIVER, args)
	//} else {
	//	//注意：这是使用ETCD中/measure/database的数据库
	//	//若出现127.0.0.1:3306的报错可能是etcd上没有获取到值（key到期？被删？）
	//	DatabaseFromETCD(conf)
	//	indexDB := Index(HashKeyByCRC32(conf.Data.MyPublicIp), len(conns))
	//	Engine, err = xorm.NewMySQL(xorm.MYSQL_DRIVER, conns[indexDB])
	//}

	if err != nil {
		return err
	}
	if err != nil {
		//logger.Error("Open "+conf.Data.MysqlAddress+" mysql fail ", zap.Error(err))
		logger.Error(fmt.Sprintf("store\topen %v mysql fail\tcpu:%v,mem:%v", conf.Data.MysqlAddress, state.LogCPU, state.LogMEM))
		return
	}
	return
}
func DeadDB() (err error) {
	if Engine != nil {
		err = Engine.Close()
		if err != nil {
			//logger.Error("Close mysql fail ", zap.Error(err))
			logger.Error(fmt.Sprintf("store\tClose mysql fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		}
	}
	return
}

func InsertTableWithTimeout(data []*measureChange.Data, i int) (success int) {
	done := make(chan struct{}, 1)
	go func() {
		beginTime := time.Now().UnixNano()
		session = Engine.NewSession()
		session.Begin()
		DetailLock.Lock()
		fin := sN[i]
		DetailLock.Unlock()
		if fin == 1 {
			session.Close()
			done <- struct{}{}
			return
		}
		count, err := session.Insert(data)
		DetailLock.Lock()
		fin = sN[i]
		DetailLock.Unlock()
		if fin == 1 {
			done <- struct{}{}
			session.Close()
			return
		}
		//fmt.Println(i,time.Now(),"err = session.Commit()")
		if err == nil || count != 0 {
			success += int(count)
		}
		err = session.Commit()
		session.Close()
		DetailLock.Lock()
		sN[i] = 1
		Duration := float64((time.Now().UnixNano()-beginTime)/1e6) / 1000.0
		SDuration := fmt.Sprintf("%.3f", Duration)
		Detail += strconv.Itoa(i) + ":" + SDuration + ";"
		DetailLock.Unlock()
		done <- struct{}{}
	}()
	select {
	case <-done:
		logger.Info(fmt.Sprintf("Insert Table done\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		InsertTableNums++
		return 1
	case <-time.After(20 * time.Second): //设置20s超时，经过测试机器正常情况20s内就能插完一个表，20s都没完成的话肯定是哪里出了问题，重来吧
		RetryTime++
		logger.Error(fmt.Sprintf("Insert Table fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		DetailLock.Lock()
		Detail += strconv.Itoa(i) + ":#" + ";"
		DetailLock.Unlock()
		return 0
	}
}
func IsTableExistWithTimeout(data0 *measureChange.Data) (ok bool, err error, finish bool) {
	done := make(chan struct{}, 1)
	go func() {
		ok, err = Engine.IsTableExist(data0)
		done <- struct{}{}
	}()
	select {
	case <-done:
		logger.Info(fmt.Sprintf("Check IsTableExist done\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return ok, err, true
	case <-time.After(10 * time.Second): //设置10s超时，经过测试机器正常情况10s内就能完，10s都没完成的话肯定是哪里出了问题，重来吧
		RetryTime++
		logger.Error(fmt.Sprintf("Check IsTableExist fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return ok, err, false
	}
}

func CreateTablesWithTimeout(data0 *measureChange.Data) (err error, finish bool) {
	done := make(chan struct{}, 1)
	go func() {
		err = Engine.CreateTables(data0)
		//err = Engine.CreateUniques(data0)
		if err != nil {

		}
		done <- struct{}{}
	}()
	select {
	case <-done:
		logger.Info(fmt.Sprintf("Create Table done\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return err, true
	case <-time.After(5 * time.Second): //设置10s超时，经过测试机器正常情况10s内就能完，10s都没完成的话肯定是哪里出了问题，重来吧
		RetryTime++
		logger.Error(fmt.Sprintf("Create Table fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return err, false
	}
}
func GenerateZlib(cells []statisticsAnalyse.Cell, curTime int64) error {
	//测量结果序列化
	data, _ := json.Marshal(cells)
	//压缩测量结果文件内容，减少字节数
	data = DoZlibCompress(data)
	//测量结果文件保存目录检测
	_, err := os.Stat("./data")
	//当保存路径不存在时创建目录
	if err != nil {
		err = os.Mkdir("./data", 0777)
	}
	//定义测量结果的文件名称
	dataFile := "./data/" + strconv.FormatInt(curTime, 10) + ".json.zlib"
	//打开文件，获取句柄
	file, _ := os.OpenFile(dataFile, os.O_CREATE|os.O_WRONLY, 0666)
	//往文件内写入压缩后的内容
	FileSize, err = file.Write(data)
	//发生错误则返回
	if err != nil {
		return err
	}
	//关闭文件
	err = file.Close()
	if err != nil {
		return err
	}
	return nil
}
func readLinesFromFile(filename string) ([]string, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	return lines, nil
}

func transferOtherServer() ([]string, []int64, []string) {
	OtherServerConf, err := readLinesFromFile("/root/falcon/plugin/net-plugin/otherServerConf/transferOther.txt")
	if err == nil && len(OtherServerConf) > 0 && OtherServerConf[0] == "Yes" {
		otherServerIP := make([]string, 0)
		otherServerPort := make([]int64, 0)
		otherServerPass := make([]string, 0)
		for index, value := range OtherServerConf[1:] {
			if regexp.MustCompile(`^\s*$`).MatchString(value) {
				continue
			}
			if index%3 == 0 {
				otherServerIP = append(otherServerIP, value)
			} else if index%3 == 1 {
				port, err := strconv.ParseInt(value, 10, 64)
				if err == nil {
					otherServerPort = append(otherServerPort, port)
				} else {
					break
				}
			} else {
				otherServerPass = append(otherServerPass, value)
			}
		}
		if len(otherServerIP) == len(otherServerPort) && len(otherServerIP) == len(otherServerPass) {
			return otherServerIP, otherServerPort, otherServerPass
		}
	}
	return nil, nil, nil
}

func DoTransferOther(curTime int64, hostName string, otherServerIP []string, otherServerPort []int64, otherServerPass []string) ([]int, []int64, error) {
	dataFile := "./data/" + strconv.FormatInt(curTime, 10) + ".json.zlib"
	// 将重试的信息写进Detail里，SshBuildFailTimes和UploadBuildFailTimes和SshBuildFailTimes不再区分，
	// 都视作失败重试。
	// 用一个列表，表示重试
	// 用一个列表，表示上传开始到结束的花费的时间
	// 写detail时不同目标间用逗号隔开
	failTimes := make([]int, len(otherServerIP))    //重试次数
	spendTimes := make([]int64, len(otherServerIP)) //花费时间
	for i := 0; i < len(otherServerIP); i++ {
		failInThis := 0
		beginTime := time.Now().UnixNano()
		//初始化SSH连接配置
		cliConf := new(ClientConfig)
		status, err := cliConf.CreateClient(otherServerIP[i], otherServerPort[i], "root", otherServerPass[i], false)

		//连接失败进行重试直到成功
		for status != 0 {
			if status == 1 {
				failInThis++
			} else {
				failInThis++
			}
			status, err = cliConf.CreateClient(otherServerIP[i], otherServerPort[i], "root", otherServerPass[i], false)
		}
		defer func(cliConf *ClientConfig) {
			if cliConf == nil {
				logger.Error("Client is nil")
			}
			err := cliConf.sftpClient.Close()
			if err != nil {
				logger.Error("SFTP Client close error.", zap.Error(err))
			}
			err = cliConf.sshClient.Close()
			if err != nil {
				logger.Error("SSH Client close error.", zap.Error(err))
			}
			cliConf = nil
		}(cliConf)
		//连接成功后开始文件传输
		status, err = cliConf.Upload(dataFile, "/root/data/"+hostName+"_"+strconv.FormatInt(curTime, 10)+".json.zlib", false)
		for status != 0 {
			//传输超时时进行失败重试
			if status == 3 {
				return nil, nil, err
			} else if status == 1 {
				failInThis++
			} else if status == 2 {
				failInThis++
			}
			status, err = cliConf.Upload(dataFile, "/root/data/"+hostName+"_"+strconv.FormatInt(curTime, 10)+".json.zlib", false)
		}

		endTime := time.Now().UnixNano()
		if err != nil {
			return nil, nil, err
		}
		failTimes[i] = failInThis
		spendTimes[i] = endTime - beginTime
	}
	return failTimes, spendTimes, nil
}
func AppendFile(filePath string, content string) (err error) {
	if UploadSeconds > 600 {
		UploadSeconds = time.Now().Unix() - UploadSeconds
	}
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		logger.Error(filePath+"文件打开失败 --", zap.Error(err))
	}
	//及时关闭file句柄
	defer func(file *os.File) {
		err = file.Close()
		if err != nil {
			logger.Error(filePath+"文件关闭失败 --", zap.Error(err))
		}
	}(file)
	//写入文件时，使用带缓存的 *Writer
	write := bufio.NewWriter(file)
	content += strconv.FormatInt(SshBuild, 10) + "\t" + strconv.Itoa(SshBuildFailTimes) + "\t"
	content += strconv.FormatInt(SftpBuild, 10) + "\t" + strconv.Itoa(SftpBuildFailTimes) + "\t"
	content += strconv.FormatInt(UploadBuild, 10) + "\t" + strconv.Itoa(UploadBuildFailTimes) + "\t"
	content += strconv.FormatInt(Upload, 10) + "\t" + strconv.Itoa(UploadFailTimes) + "\t"

	for i := 0; i < 15; i++ {
		content = content + "\t" + BJValue[i]
	}
	content = content + "\t" + Detail

	//content=content+"\t"+ strconv.FormatInt(measure.Prepare, 10)
	_, err = write.WriteString(content + "\n")
	if err != nil {
		logger.Error(filePath+"文件写入缓存失败 --", zap.Error(err))
		return
	}
	//Flush将缓存的文件真正写入到文件中
	err = write.Flush()
	if err != nil {
		logger.Error(filePath+"文件真正写入失败 --", zap.Error(err))
		return
	}
	return
}

// TODO 封装各个插件的数据，然后利用GORM上传
func Store(cells []statisticsAnalyse.Cell, conf *config.Config) int {
	//debugTime:=time.Now()
	//fmt.Println("start store:",debugTime)
	//debugTime=time.Now()
	//fmt.Println("after birth Engine:",debugTime)
	//err2 := Engine.Ping()
	//debugTime=time.Now()
	//fmt.Println("after ping test:",debugTime)
	//if err2 != nil {
	//	logger.Error("Ping mysql databases fail", zap.Error(err2))
	//	return 0
	//}
	logger.Info(fmt.Sprintf("store\tstart insert measure data\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	var affectRows = 0
	multiTable, tableSlice := sep2MultiTable(cells, conf)
	curTime := time.Now().Unix()
	err := GenerateZlib(cells, curTime)
	if err != nil {
		return 0
	}
	resBytes, resPacket := NetRate()
	Detail += "BytesRate:" + strconv.FormatInt(resBytes, 10) + ";"
	Detail += "PacketRate:" + strconv.FormatInt(resPacket, 10) + ";"

	otherServerIP, otherServerPort, otherServerPass := transferOtherServer()
	failTimes, spendTimes, err := DoTransferOther(curTime, conf.Data.Hostname, otherServerIP, otherServerPort, otherServerPass)

	if err == nil {
		failString := "OtherServerFailTimes:" + strconv.Itoa(failTimes[0])
		spendString := "OtherServerSpendTimes:" + strconv.FormatInt(spendTimes[0], 10)
		for i := 1; i < len(failTimes); i++ {
			failString += "," + strconv.Itoa(failTimes[i])
			spendString += "," + strconv.FormatInt(spendTimes[i], 10)
		}
		failString += ";"
		spendString += ";"
		Detail += failString
		Detail += spendString
	} else {
		fmt.Println(err)
	}
	err = StoreJsonResult(curTime, conf.Data.Hostname, "106.3.133.5", 60708, "1000lgf,wchql")
	if err == nil {
		Detail += "Transfer Finish;"
		if TransferRestFile(conf.Data.Hostname) == 0 {
			Detail += "NoRest;"
		}
		err := AppendFile("./CrontabStatus.txt", "")
		if err != nil {
			return 0
		}
		return 0
	}
	UploadSeconds = time.Now().Unix()
	for Engine == nil {
		BirthDBWithTimeout(conf)
	}
	if err != nil {
		logger.Error("Birth DB fail", zap.Error(err))
	}
	for _, tableName := range tableSlice {
		data := multiTable[tableName]
		if len(data) == 0 {
			continue
		}
		ok, err, finish := IsTableExistWithTimeout(data[0])
		for finish == false {
			ok, err, finish = IsTableExistWithTimeout(data[0])
		}
		if !ok || err != nil {
			err, finish = CreateTablesWithTimeout(data[0])
			for finish == false {
				err, finish = CreateTablesWithTimeout(data[0])
			}
			if err != nil {
				logger.Error("Create Table fail", zap.Error(err))
			}
		}
	}
	//debugTime=time.Now()
	//fmt.Println("before new session:",debugTime)
	//session := Engine.NewSession()
	//debugTime=time.Now()
	//fmt.Println("after new session:",debugTime)
	//err = session.Begin()
	//debugTime=time.Now()
	//fmt.Println("now session begin:",debugTime)
	//session := Engine.NewSession()
	//for session==nil{
	//	session = Engine.NewSession()
	//}
	////表传输策略：先传rtt_avg,packet_loss,rtt_jitter_avg
	//tableSlice[14],tableSlice[1],tableSlice[7],tableSlice[2]=tableSlice[1],tableSlice[14],tableSlice[2],tableSlice[7]
	////表传输策略：六个四分位数放在最后
	//tableSlice[6],tableSlice[14]=tableSlice[14],tableSlice[6]
	//tableSlice[5],tableSlice[10]=tableSlice[10],tableSlice[5]
	//tableSlice[4],tableSlice[9]=tableSlice[9],tableSlice[4]
	for i, tableName := range tableSlice {
		s := InsertTableWithTimeout(multiTable[tableName], i)
		if s == 0 && sN[i] == 0 {
			s = InsertTableWithTimeout(multiTable[tableName], i)
		}
	}
	UploadSeconds = time.Now().Unix() - UploadSeconds
	//debugTime=time.Now()
	//fmt.Println("after insert all table:",debugTime)
	//err = session.Commit()
	//debugTime=time.Now()
	//fmt.Println("after session commit:",debugTime)
	if err != nil {
		logger.Error("Commit create table fail:", zap.Error(err))
	}
	//logger.Info("finish record", zap.Int("nums", affectRows))
	logger.Info(fmt.Sprintf("store\tfinish record measure data\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	//err := session.Close()
	//if err != nil {
	//	//logger.Error("Close mysql fail ", zap.Error(err))
	//	logger.Error(fmt.Sprintf("store\tClose mysql fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	//	return 0
	//}

	//每次测量完查一次，影响效率
	//for _, tableName := range tableSlice {
	//	SendTimeStamp:=multiTable[tableName][0].TimeStamp
	//	sql_2_1 := "select count(*) from "+tableName+" where timestamp='"+SendTimeStamp+"' and source_name='"+conf.Data.Hostname+"'"
	//	results, err := Engine.QueryString(sql_2_1)
	//	if err != nil {
	//		logger.Error("Engine.QueryString fail:", zap.Error(err))
	//	}
	//	//fmt.Println(results[0]["count(*)"])
	//	s_insert,_:=strconv.Atoi(results[0]["count(*)"])
	//	SendTimeStampInt64, err := strconv.ParseInt(SendTimeStamp, 10, 64)
	//	//fmt.Println(tableName[:len(tableName)-9])
	//	n9e.Collect(conf.Data.Hostname,float64(s_insert),SendTimeStampInt64,conf.Data.Step,"proc.InsertNums."+tableName[:len(tableName)-9])
	//}

	if Engine != nil {
		err = DeadDB()
	}
	if err != nil {
		logger.Error("Dead DB fail", zap.Error(err))
	}
	return affectRows
}

func sep2MultiTable(cells []statisticsAnalyse.Cell, conf *config.Config) (dataMap map[string][]*measureChange.Data, tableNameSlice []string) {
	// 直接make，永远不要返回nil
	tableNameSlice = make([]string, 0)
	dataMap = make(map[string][]*measureChange.Data)
	for _, cell := range cells {
		metric := &measureChange.MetaValue{
			Endpoint: cell.Endpoint,
			Metric:   cell.Metric,
			Value:    cell.Value,
			Step:     strconv.FormatInt(cell.Step, 10),
			Type:     cell.CounterType,
			//
			Tags:       dictedTagstring(cell.Tags),
			Timestamp:  strconv.FormatInt(cell.Timestamp, 10),
			SourceIp:   cell.SourceIp,
			SourceName: cell.SourceName,
			DstIp:      cell.DestIp,
		}
		data := measureChange.Convert(metric)
		data.TimeStamp = strconv.FormatInt(time.Now().Unix(), 10)
		if data.DesIp == strings.Split(conf.Data.MysqlAddress, ":")[0] {
			if data.DesName == "proc.icmp.BJ.DB.rtt.avg" {
				BJValue[0] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.var" {
				BJValue[1] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.min" {
				BJValue[2] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.max" {
				BJValue[3] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.quantile25" {
				BJValue[4] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.quantile50" {
				BJValue[5] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.quantile75" {
				BJValue[6] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.jitter.avg" {
				BJValue[7] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.jitter.var" {
				BJValue[8] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.jitter.min" {
				BJValue[9] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.jitter.max" {
				BJValue[10] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.jitter.quantile25" {
				BJValue[11] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.jitter.quantile50" {
				BJValue[12] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.rtt.jitter.quantile75" {
				BJValue[13] = data.Value
			} else if data.DesName == "proc.icmp.BJ.DB.packet.loss" {
				BJValue[14] = data.Value
			}
		}
		tableName := data.TableName()
		if _, ok := dataMap[tableName]; !ok {
			dataMap[tableName] = make([]*measureChange.Data, 0)
			tableNameSlice = append(tableNameSlice, tableName)
		}
		dataMap[tableName] = append(dataMap[tableName], data)
	}
	return
}
