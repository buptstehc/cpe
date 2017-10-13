package plugin

import (
	"net"
	"bufio"
	"fmt"
	"errors"
	"io"
	"encoding/binary"
	"crypto/md5"
	"time"
	"github.com/pquerna/ffjson/ffjson"
)

const (
	Write_Chan_Cap = 32
	DevRND         = "0123456789ABCDEF"
	READ_TIMEOUT   = 10 * time.Second
	HB_INTERVAL    = 60 * time.Second
	RETRY_INTERVAL = 30 * time.Second
)

var (
	g_error_conn_close = errors.New("tcp conn close")
)

type TCPConn struct {
	id           uint32
	conn         net.Conn
	reader       *bufio.Reader
	writer       *bufio.Writer
	writeChan    chan []byte
	msgChan      chan Msg
	isClose      bool //isClose标记仅在ResetConn、Close中设置，其它地方只读
	isAbort      bool //网关注册失败时退出
	localAddr    string
	remoteAddr   string
	mac          string
	sn           string
	loid         string
	hbReq        []byte
	plugins      map[string]InstallPacket //安装的插件
	isPlgRunning map[string]bool
}

func NewTCPConn(msgChan chan Msg, id uint32, local, remote, mac, sn, loid string) *TCPConn {
	self := new(TCPConn)
	self.msgChan = msgChan
	self.id = id
	self.writeChan = make(chan []byte, Write_Chan_Cap)
	self.isClose = false
	self.isAbort = false
	self.localAddr = local
	self.remoteAddr = remote
	self.mac = mac
	self.sn = sn
	self.loid = loid
	self.reader = nil
	self.writer = nil
	self.plugins = make(map[string]InstallPacket, 10)
	self.isPlgRunning = make(map[string]bool, 10)

	hb := NewHBPacket()
	self.hbReq, _ = hb.Serialize()

	return self
}

func (self *TCPConn) Run() {
	for {
		if self.connect() {
			if self.init() {
				go self.readRoutine()
				self.writeRoutine() //goroutine会阻塞在这里
			} else {
				self.close()
			}
		}

		if self.isAbort {
			break
		}

		time.Sleep(RETRY_INTERVAL)
	}

	self.msgChan <- Msg{ID: self.id, Event: E_ABORT}
}

func (self *TCPConn) connect() bool {
	local := &net.TCPAddr{
		IP: net.ParseIP(self.localAddr),
	}
	d := net.Dialer{LocalAddr: local}

	conn, err := d.Dial("tcp", self.remoteAddr)
	if err != nil {
		fmt.Printf("connect to %s error :%s \n", self.remoteAddr, err.Error())
		return false
	}

	self.resetConn(conn)

	return true
}

func (self *TCPConn) resetConn(conn net.Conn) {
	self.conn = conn

	if self.reader == nil {
		self.reader = bufio.NewReader(conn)
	} else {
		//reuse bufio
		self.reader.Reset(conn)
	}

	if self.writer == nil {
		self.writer = bufio.NewWriter(conn)
	} else {
		//reuse bufio
		self.writer.Reset(conn)
	}

	self.isClose = false
}

func (self *TCPConn) close() {
	if self.isClose {
		return
	}
	self.conn.Close()
	self.writeBuf(nil) //触发writeRoutine结束
	self.isClose = true
}

func (self *TCPConn) writeBuf(buf []byte) {
	select {
	case self.writeChan <- buf:
	default:
		fmt.Println("writeBuf: channel full")
		self.conn.(*net.TCPConn).SetLinger(0)
		self.close()
	}
}

func (self *TCPConn) _writeFull(buf []byte) (err error) {
	if buf == nil {
		return g_error_conn_close
	}
	var n, nn int
	length := len(buf)
	for n < length && err == nil {
		nn, err = self.writer.Write(buf[n:]) //【Notice: WriteFull】bufio包装过，这里不会陷入系统调用；先缓存完chan的数据再Flush，更高效
		n += nn
	}
	if err != nil {
		fmt.Printf("WriteRoutine Write error: %s\n", err.Error())
	}
	return
}

func (self *TCPConn) writeRoutine() {
	defer self.close()

	for {
		select {
		case buf := <-self.writeChan:
			if self._writeFull(buf) != nil {
				return
			}
		default:
			var err error
			for i := 0; i < 100; i++ { //还写不完，等下一轮调度吧
				if err = self.writer.Flush(); err != io.ErrShortWrite {
					break
				}
			}
			if err != nil {
				fmt.Printf("WriteRoutine Flush error: %s\n", err.Error())
				return
			}
			//block
			buf := <-self.writeChan
			if self._writeFull(buf) != nil {
				return
			}
		}
	}
}

func (self *TCPConn) _readFull(to time.Duration) ([]byte, error) {
	var err error
	var msgLen int
	var msgHeader = make([]byte, 4) //前4字节存msgLen

	if to > 0 {
		self.conn.SetReadDeadline(time.Now().Add(to))
	} else {
		//clear timeout
		self.conn.SetReadDeadline(time.Time{})
	}

	_, err = io.ReadFull(self.reader, msgHeader)
	if err != nil {
		fmt.Printf("read msg header error: %s\n", err.Error())
		return nil, err
	}

	msgLen = int(binary.BigEndian.Uint32(msgHeader))
	if msgLen <= 0 {
		fmt.Printf("invalid msg length: %d\n", msgLen)
		return nil, err
	}

	msg := make([]byte, msgLen)
	_, err = io.ReadFull(self.reader, msg)
	if err != nil {
		fmt.Printf("read msg body error: %s\n", err.Error())
		return nil, err
	}

	return msg, nil
}

func (self *TCPConn) readRoutine() {
	defer self.close()

	for {
		if self.isClose {
			break
		}

		msg, err := self._readFull(0)
		if err != nil {
			break
		}

		self.dispatch(msg)
	}
}

func (self *TCPConn) dispatch(msg []byte) {
	pkt := Packet{}
	ffjson.Unmarshal(msg, &pkt)
	m := pkt.RPCMethod

	switch m {
	case "":
		//recognize as PONG, just ignore
		self.msgChan <- Msg{ID: self.id, Event: E_PONG}
	case "Install":
		p := InstallPacket{}
		ffjson.Unmarshal(msg, &p)
		self.doInstall(p)
	case "Install_query":
		p := InstallQueryPacket{}
		ffjson.Unmarshal(msg, &p)
		self.doInstallQuery(p)
	case "Install_cancel":
		p := InstallCancelPacket{}
		ffjson.Unmarshal(msg, &p)
		self.doInstallCancel(p)
	case "UnInstall":
		p := UnInstallPacket{}
		ffjson.Unmarshal(msg, &p)
		self.doUnInstall(p)
	case "Stop":
		p := StopPacket{}
		ffjson.Unmarshal(msg, &p)
		self.doStop(p)
	case "Run":
		p := RunPacket{}
		ffjson.Unmarshal(msg, &p)
		self.doRun(p)
	case "FactoryPlugin":
		p := FactoryPluginPacket{}
		ffjson.Unmarshal(msg, &p)
		self.doFactoryPlugin(p)
	case "ListPlugin":
		p := ListPluginPacket{}
		ffjson.Unmarshal(msg, &p)
		self.doListPlugin(p)
	}

}

func (self *TCPConn) init() bool {
	boot := NewBootInitiationPacket(self.id, self.mac)
	b, ok := boot.Serialize()
	if !ok {
		return false
	}

	_, err := self.conn.Write(b)
	if err != nil {
		fmt.Printf("write doBootInitiation fails: %s\n", err.Error())
		return false
	}
	self.msgChan <- Msg{ID: self.id, Event: E_BOOT_SENT}

	msg, err := self._readFull(READ_TIMEOUT)
	if err != nil {
		fmt.Printf("read doBootInitiation respone fails: %s\n", err.Error())
		return false
	}
	self.msgChan <- Msg{ID: self.id, Event: E_BOOT_REPLY}

	pkt := BootInitiationRespPacket{}
	ffjson.Unmarshal(msg, &pkt)
	return self.doBootInitiationResp(pkt)
}

func (self *TCPConn) doBootInitiationResp(pkt BootInitiationRespPacket) bool {
	if pkt.Result != 0 {
		self.close()
		self.isAbort = true
		return false
	}

	return self.doRegister(pkt.ChallengeCode)
}

func (self *TCPConn) doRegister(challengeCode string) bool {
	//param := challengeCode + self.sn + self.loid
	param := challengeCode + self.sn + "123"
	m := fmt.Sprintf("%x", md5.Sum([]byte(param)))

	rp := NewRegisterPacket(self.id, self.mac, m, DevRND)
	b, ok := rp.Serialize()
	if !ok {
		return false
	}

	_, err := self.conn.Write(b)
	if err != nil {
		fmt.Printf("write Register fails: %s\n", err.Error())
		return false
	}
	self.msgChan <- Msg{ID: self.id, Event: E_REG_SENT}

	msg, err := self._readFull(READ_TIMEOUT)
	if err != nil {
		fmt.Printf("read Register respone fails: %s\n", err.Error())
		return false
	}
	self.msgChan <- Msg{ID: self.id, Event: E_REG_REPLY}

	pkt := RegisterRespPacket{}
	ffjson.Unmarshal(msg, &pkt)
	return self.doRegisterResp(pkt)
}

func (self *TCPConn) doRegisterResp(pkt RegisterRespPacket) bool {
	if pkt.Result != 0 {
		self.close()
		self.isAbort = true
		return false
	}

	//param := self.sn + DevRND
	//m := fmt.Sprintf("%x", md5.Sum([]byte(param)))
	//if m != pkt.CheckPlatform {
	//	self.close()
	//	self.isAbort = true
	//	return
	//}

	//启动心跳
	go self.doHB()

	return true
}

func (self *TCPConn) doHB() {
	for {
		if self.isClose {
			break
		}

		time.Sleep(HB_INTERVAL)
		self.writeBuf(self.hbReq)
		self.msgChan <- Msg{ID: self.id, Event: E_PING}
	}
}

func (self *TCPConn) doInstall(pkt InstallPacket) {
	self.plugins[pkt.Plugin_Name] = pkt
	self.isPlgRunning[pkt.Plugin_Name] = true

	resp := RespPacket{ID:pkt.ID, Result:0}
	bytes, _ := resp.Serialize()
	self.writeBuf(bytes)
}

func (self *TCPConn) doInstallQuery(pkt InstallQueryPacket) {
	resp := InstallQueryRespPacket{ID:pkt.ID, Result:0, Percent:"100"}
	bytes, _ := resp.Serialize()
	self.writeBuf(bytes)
}

func (self *TCPConn) doInstallCancel(pkt InstallCancelPacket) {
	resp := InstallCancelRespPacket{ID:pkt.ID}
	_, exists := self.plugins[pkt.Plugin_Name]
	if exists {
		resp.Result = -111
	} else {
		resp.Result = -101
	}

	bytes, _ := resp.Serialize()
	self.writeBuf(bytes)
}

func (self *TCPConn) doUnInstall(pkt UnInstallPacket) {
	resp := UnInstallRespPacket{ID:pkt.ID}
	_, exists := self.plugins[pkt.Plugin_Name]
	if exists {
		resp.Result = 0
		delete(self.plugins, pkt.Plugin_Name)
		delete(self.isPlgRunning, pkt.Plugin_Name)
	} else {
		resp.Result = -112
	}

	bytes, _ := resp.Serialize()
	self.writeBuf(bytes)
}

func (self *TCPConn) doStop(pkt StopPacket) {
	resp := StopRespPacket{ID:pkt.ID}
	_, exists := self.plugins[pkt.Plugin_Name]
	if exists {
		resp.Result = 0
		self.isPlgRunning[pkt.Plugin_Name] = false
	} else {
		resp.Result = -112
	}

	bytes, _ := resp.Serialize()
	self.writeBuf(bytes)
}

func (self *TCPConn) doRun(pkt RunPacket) {
	resp := RunRespPacket{ID:pkt.ID}
	_, exists := self.plugins[pkt.Plugin_Name]
	if exists {
		resp.Result = 0
		self.isPlgRunning[pkt.Plugin_Name] = true
	} else {
		resp.Result = -112
	}

	bytes, _ := resp.Serialize()
	self.writeBuf(bytes)
}

func (self *TCPConn) doFactoryPlugin(pkt FactoryPluginPacket) {
	resp := FactoryPluginRespPacket{ID:pkt.ID}
	_, exists := self.plugins[pkt.Plugin_Name]
	if exists {
		resp.Result = 0
	} else {
		resp.Result = -112
	}

	bytes, _ := resp.Serialize()
	self.writeBuf(bytes)
}

func (self *TCPConn) doListPlugin(pkt ListPluginPacket) {
	resp := ListPluginRespPacket{ID:pkt.ID}
	for _, p := range self.plugins {
		p2 := InstalledPacket{}
		if self.isPlgRunning[p.Plugin_Name] {
			p2.Run = 1
		} else {
			p2.Run = 0
		}
		p2.Plugin_Name = p.Plugin_Name
		p2.Version = p.Version

		resp.Plugin = append(resp.Plugin, p2)
	}

	bytes, _ := resp.Serialize()
	self.writeBuf(bytes)
}
