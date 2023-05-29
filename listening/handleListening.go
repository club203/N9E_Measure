package listening

import (
	"Traceroute/state"
	"fmt"
	"io"
	"net"
	"os"
)

const (
	shellToUse = "bash"
	proTypeTCP = "tcp4"
	proTypeUDP = "udp4"
)

func tcpListen(addr string) {
	tcpAddr, err := net.ResolveTCPAddr(proTypeTCP, addr)
	ln, err := net.ListenTCP(proTypeTCP, tcpAddr) //tcp 端口监听回显
	if err != nil {
		//logger.Error("listening server port start fail: ", zap.Error(err))
		logger.Error(fmt.Sprintf("listen\tlistening server port start fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return
	}
	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			//logger.Error("accept fail", zap.String("proType", proTypeTCP), zap.Error(err))
			logger.Error(fmt.Sprintf("listen\taccept tcp fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		}
		go handleTCPMessage(conn)
	}
}
func handleTCPMessage(conn *net.TCPConn) {
	//Close()方法用于关闭conn连接。实现代码中会优先调用 ok()方法
	defer conn.Close()
forLabel:
	for {
		receive := make([]byte, 1000)
		//Read 方法用于从conn对象中读取字节流并写入[]byte类型的b对象中
		n, err := conn.Read(receive) // 阻塞等待
		//if n < 0 || err != nil {
		//	logger.Errorf("%v server read fail: %v", proType, err)
		//}
		switch err {
		//当在Read时，收到一个IO.EOF，代表的就是对端已经关闭了发送的通道，通常来说是发起了FIN
		// 当客户端发起FIN之后，服务器会进如CLOSE_WAIT的状态，这个状态的主要意义在于，
		//如果服务器有没有发送完的数据需要发送给客户端时，那么需要继续发送，当服务器端发送完毕之后，再给客户端回复FIN表示关闭
		//所以，可以间接的认为，当我们收到一个IO.EOF的时候，就是客户端在发起FIN了
		case io.EOF:
			break forLabel //使用了goto语法
		case nil:

		default:
			//logger.Error("server read fail: ", zap.String("proType", proTypeTCP), zap.Error(err))
			logger.Error(fmt.Sprintf("listen\tserver read fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		}
		//Write()方法用于把[]byte类型的切片中的数据写入到conn对象中
		n, err = conn.Write(receive[0:n])
		if err != nil || n < 0 {
			//logger.Error("server write fail: ", zap.String("proType", proTypeTCP), zap.Error(err))
			logger.Error(fmt.Sprintf("listen\ttcp server write fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		}
		receive = nil
	}
}

//socat命令
func udpListen(addr string) {

	udpAddr, err := net.ResolveUDPAddr(proTypeUDP, addr)
	if err != nil {
		//logger.Error("resolve udp addr fail:", zap.Error(err))
		logger.Error(fmt.Sprintf("listen\tresolve udp addr fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return
	}
	udpConn, err := net.ListenUDP(proTypeUDP, udpAddr) //UDP 回显监听
	//待修改zm
	defer func() {
		if udpConn != nil {
			err2 := udpConn.Close()
			if err2 != nil {
				//logger.Error("udpConn close fail")
				logger.Error(fmt.Sprintf("listen\tudpConn close fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
			}
		}
	}()
	if err != nil {
		//logger.Error("listening server port start fail: ", zap.Error(err))
		logger.Error(fmt.Sprintf("listen\tlistening udp server port start fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		os.Exit(1)
	}
	for {
		//循环处理客户端
		receive := make([]byte, 1024)
		//读取一条udp消息，拿到客户端remoteAddr
		n, recvAddr, err := udpConn.ReadFromUDP(receive)
		if n < 0 || err != nil {
			//logger.Error("server read fail: ", zap.String("proType", proTypeUDP), zap.Error(err))
			logger.Error(fmt.Sprintf("listen\tudp server read fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		}
		go handleUDPMessage(udpConn, *recvAddr, receive, n)
	}
}
func handleUDPMessage(udpConn *net.UDPConn, addr net.UDPAddr, receive []byte, n int) {
	//给客户端回复一条消息
	count, err := udpConn.WriteToUDP(receive[0:n], &addr)
	if err != nil || count < 0 {
		//logger.Error("send fail", zap.Any("addr", addr), zap.Error(err))
		logger.Error(fmt.Sprintf("listen\tsend message to client fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
		return
	}
}

//运行，进行发包的回显
func socatUDP(port string) {
	//服务端在 TCP-LISTEN 地址后面加了 fork 的参数后，就能同时应答多个链接过来的客户端，
	//每个客户端会 fork 一个进程出来进行通信
	err, _ := ShellCmd(fmt.Sprintf("socat -T 3 UDP-LISTEN:%v,FORK PIPE&", port))
	if err != nil {
		//logger.Error("install yum fail  ", zap.String("output", out), zap.Error(err))
		logger.Error(fmt.Sprintf("listen\tinstall yum fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	}
}
func socatTCP(port string) {
	err, _ := ShellCmd(fmt.Sprintf("socat -T 3 TCP-LISTEN:%v,FORK PIPE&", port))
	if err != nil {
		//logger.Error("install yum fail", zap.String("output", out), zap.Error(err))
		logger.Error(fmt.Sprintf("listen\tinstall yum fail\tcpu:%v,mem:%v", state.LogCPU, state.LogMEM))
	}
}
