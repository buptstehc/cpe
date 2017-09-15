package plugin

import (
	"net"
	"bufio"
	"fmt"
	"errors"
	"io"
	"encoding/binary"
	"encoding/json"
	"crypto/md5"
	"time"
)

const (
	Write_Chan_Cap = 32
	DevRND         = "0123456789ABCDEF"
	READ_TIMEOUT   = 10 * time.Second
)

var (
	g_error_conn_close = errors.New("tcp conn close")
)

type TCPConn struct {
	id         uint32
	conn       net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
	writeChan  chan []byte
	msgChan    chan Msg
	isClose    bool //isClose标记仅在ResetConn、Close中设置，其它地方只读
	isAbort    bool //网关注册失败时退出
	localAddr  string
	remoteAddr string
	mac        string
	sn         string
	loid       string
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

		time.Sleep(10 * time.Second)
	}

	self.msgChan <- Msg{ID:self.id, Event:E_ABORT}
}

func (self *TCPConn) connect() bool {
	local := &net.TCPAddr{
		IP: net.ParseIP(self.localAddr),
	}
	d := net.Dialer{LocalAddr: local}

	conn, err := d.Dial("tcp", self.remoteAddr)
	if err != nil {
		fmt.Errorf("connect to %s error :%s \n", self.remoteAddr, err.Error())
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
		fmt.Errorf("writeBuf: channel full")
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
		fmt.Errorf("WriteRoutine Write error: %s", err.Error())
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
				fmt.Errorf("WriteRoutine Flush error: %s", err.Error())
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
		fmt.Errorf("read msg header error: %s", err.Error())
		return nil, err
	}

	msgLen = int(binary.BigEndian.Uint32(msgHeader))
	if msgLen <= 0 {
		fmt.Errorf("invalid msg length: %d", msgLen)
		return nil, err
	}

	msg := make([]byte, msgLen)
	_, err = io.ReadFull(self.reader, msg)
	if err != nil {
		fmt.Errorf("read msg body error: %s", err.Error())
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
	json.Unmarshal(msg, &pkt)

	self.doRPC(pkt)
}

func (self *TCPConn) init() bool {
	boot := NewBootInitiationPacket(self.id, self.mac)
	b, ok := boot.Serialize()
	if !ok {
		return false
	}

	_, err := self.conn.Write(b)
	if err != nil {
		fmt.Errorf("write doBootInitiation fails: %s", err.Error())
		return false
	}
	self.msgChan <- Msg{ID:self.id, Event: E_BOOT_SENT}

	msg, err := self._readFull(READ_TIMEOUT)
	if err != nil {
		fmt.Errorf("read doBootInitiation respone fails: %s", err.Error())
		return false
	}
	self.msgChan <- Msg{ID:self.id, Event:E_BOOT_REPLY}


	pkt := BootInitiationRespPacket{}
	json.Unmarshal(msg, &pkt)
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
	param := challengeCode + self.sn
	m := fmt.Sprintf("%x", md5.Sum([]byte(param)))

	rp := NewRegisterPacket(self.id, self.mac, m, DevRND)
	b, ok := rp.Serialize()
	if !ok {
		return false
	}

	_, err := self.conn.Write(b)
	if err != nil {
		fmt.Errorf("write Register fails: %s", err.Error())
		return false
	}
	self.msgChan <- Msg{ID:self.id, Event: E_REG_SENT}

	msg, err := self._readFull(READ_TIMEOUT)
	if err != nil {
		fmt.Errorf("read Register respone fails: %s", err.Error())
		return false
	}
	self.msgChan <- Msg{ID:self.id, Event: E_REG_REPLY}

	pkt := RegisterRespPacket{}
	json.Unmarshal(msg, &pkt)
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

		time.Sleep(60 * time.Second)
		hb := NewHBPacket()
		b, ok := hb.Serialize()
		if !ok {
			break
		}

		self.writeBuf(b)
		self.msgChan <- Msg{ID:self.id, Event:E_PING}
	}
}

func (self *TCPConn) doRPC(pkt Packet) {
	//可能收到心跳响应和RPC请求
	if pkt.PONG == "PONG" {
		//ignore
		self.msgChan <- Msg{ID:self.id, Event: E_PONG}
		return
	}
}
