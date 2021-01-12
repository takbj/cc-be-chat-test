package netutil

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var _session_id int64

const (
// SEND_PACKET_LIMIT = 64 * 1024
)

type E_PACKET_TYPE uint8

const (
	CLIENT_PACKET_TYPE E_PACKET_TYPE = 1
	SERVER_PACKET_TYEP E_PACKET_TYPE = 2
)

//支持的包需要提供的接口
type IFNetPacket interface {
	Parse(data []byte) (ok bool)
	Write2Cache(cache []byte) (writeLen int)

	SetSession(*TSession)
	Session() *TSession
	SetId(mainId, subId uint16) IFNetPacket
	SetId2(id uint32) IFNetPacket
	Id() (mainId uint16, subId uint16)
	Id2() uint32
	SetData(data []byte) IFNetPacket
	Data() []byte
	SetWriteOkProc(func(IFNetPacket)) IFNetPacket //设置写入完成的回调函数
	GetWriteOkProc() func(IFNetPacket)
	Gen() IFNetPacketGen
	SetCheckValue(uint32) IFNetPacket
	CheckValue() uint32
}

type IFNetPacketGen interface {
	Get(sess *TSession) IFNetPacket
	Put(IFNetPacket)
}

type IFMsger interface {
	ProcessMsg(msg IFNetPacket) // CBMsgProc
}

type IFMsgerEx interface {
	Logout(key string)
	Destroy()
}

type IFServerMsger interface {
	IFMsger
	ProcessAccept(session *TSession)  // CBAcceptProc
	ProcessOffline(session *TSession) // CBOfflineProc
}

type IFServerMsgerEx interface {
	IFMsgerEx
	IFServerMsger
	IsPreferential(msg IFNetPacket) bool
	PreferentialProcessMsg(msg IFNetPacket)
}

type TSession struct {
	conn                 net.Conn         //
	packGen              IFNetPacketGen   //包生成器
	send_cache           []byte           // 发送缓冲
	sendChan             chan IFNetPacket //IFNetPacket 发送管道
	readChan             chan IFNetPacket //IFNetPacket 接收管道
	preferentialReadChan chan IFNetPacket //IFNetPacket 接收优先管道
	offChan              chan *TSession   //
	SessionId            int64            // ID
	offLine              int32            // 是否掉线, 0为掉线，非0为在线
	msger                IFMsger          //消息过程
	ServerMsger          IFServerMsgerEx  //支持优先级处理的过程

	wg                    *sync.WaitGroup
	ctx                   context.Context
	cancel                context.CancelFunc //关闭退出
	kickCtx               context.Context
	readDelay             time.Duration
	sendDalay             time.Duration
	maxRecvSize           uint32        //最大包大小
	rpmCountLimit         uint32        // 包频率包数
	rpmCheckInterval      time.Duration // 包频率检测间隔
	rpmOffLineMsg         IFNetPacket   // 超过频率控制离线通知包
	rpmKickCount          uint32        //超过频率检测次数就踢掉
	delayTimer            *time.Timer
	needCheckPreferential bool

	OnLineTime  int64
	OffLineTime int64

	EncodeKey []byte
	DecodeKey []byte

	rpmEndTime  time.Time
	rpmCount    uint32
	rpmMsgCount uint32

	packLenBuf []byte //缓存包长度头

	attachData sync.Map //附加数据
}

//用于测试的模拟接口
func MakeSess(conn net.Conn) (sess *TSession) {
	return &TSession{
		conn: conn,
	}
}

func (self *TSession) SetAttachData(key string, value interface{}) {
	if value == nil {
		self.attachData.Delete(key)
	}
	self.attachData.Store(key, value)
}

func (self *TSession) GetAttachData(key string) (value interface{}, ok bool) {
	return self.attachData.Load(key)
}

func GetIpFromConn(conn net.Conn) (ip string) {
	addr := conn.RemoteAddr().String()
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}

	return host
}

//网络连接远程ip
func (self *TSession) NetIp() string {
	return GetIpFromConn(self.conn)
}

//网络连接远程信息
func (self *TSession) NetInfo() string {
	addr := self.conn.RemoteAddr().String()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return fmt.Sprintf("%s:%s", net.ParseIP(host).String(), port)
}

func (self *TSession) Send(packet IFNetPacket) {
	if atomic.LoadInt32(&self.offLine) != 0 || packet == nil {
		return
	}

	self.sendChan <- packet
}

//从conn中读取一个包
func (self *TSession) readPack() (pack IFNetPacket, ok bool) {
	// 4字节包长度
	if _, err := io.ReadFull(self.conn, self.packLenBuf); err != nil {
		// if err != io.EOF && err != io.ErrClosedPipe && 0 == atomic.LoadInt32(&self.offLine) {
		// 	log.Println(fmt.Sprintf(
		// 		"error receiving header, bytes: %v, self.readDelay=%v, sessionId:=%v, err:%v",
		// 		n, self.readDelay, self.SessionId, err.Error()))
		// }
		return
	}

	packlen := binary.LittleEndian.Uint32(self.packLenBuf)
	if packlen > self.maxRecvSize {
		log.Println("error receiving packLen:", packlen, " sessionId:", self.SessionId)
		return
	}

	data := make([]byte, packlen)
	if n, err := io.ReadFull(self.conn, data); err != nil {
		log.Println("error receiving msg, bytes:", n, "packLen:", packlen, " sessionId:", self.SessionId, "reason:", err)
		return
	}

	if !self.rpmCheck() {
		return
	}

	pack = self.packGen.Get(self)
	ok = pack.Parse(data)
	if !ok {
		tmp := fmt.Sprintf("receive pack packlen=%v, len(pack.Data())=%v, pack.Data()=%#v", packlen, len(pack.Data()), pack.Data())
		log.Println(tmp)
	}

	return
}

//向conn写入一个包
func (self *TSession) writePack(pack IFNetPacket) (ok bool) {
	packLen := pack.Write2Cache(self.send_cache[4:])

	// 写超时
	if self.sendDalay > 0 {
		self.conn.SetWriteDeadline(time.Now().Add(self.sendDalay))
	}

	if false {
		mainId, subId := pack.Id()
		tmp := fmt.Sprintf("send pack, sessId=%v,, MSGID=(%v:%v), packLen=%v, len(pack.Data())=%v", self.SessionId, mainId, subId, packLen, len(pack.Data()))
		log.Println(tmp)
		log.Println(tmp)
	}

	binary.LittleEndian.PutUint32(self.send_cache, uint32(packLen))
	n, err := self.conn.Write(self.send_cache[:uint32(packLen)+4])
	if err != nil {
		// if n != 0 {
		log.Println("error writing msg, bytes:", n, " sessionId:", self.SessionId, "reason:", err)
		// }
		return
	}

	if nil != pack.GetWriteOkProc() {
		pack.GetWriteOkProc()(pack)
	}

	pack.Gen().Put(pack)

	return true
}

func (self *TSession) rpmCheck() (ok bool) {
	// 收包频率控制
	if self.rpmCountLimit > 0 {
		self.rpmCount++

		// 达到限制包数
		if self.rpmCount > self.rpmCountLimit {
			now := time.Now()
			// 检测时间间隔
			if self.rpmEndTime.After(now) {
				// 发送频率过高的消息包
				if self.rpmOffLineMsg != nil {
					self.Send(self.rpmOffLineMsg)
				}

				// 提示操作太频繁三次后踢下线
				self.rpmMsgCount++
				if self.rpmKickCount > 0 && self.rpmMsgCount > self.rpmKickCount {
					log.Println("session RPM too high ", self.rpmCount, " in ", self.rpmCheckInterval, "s sessionId:", self.SessionId)
					return false
				}
			}

			// 未超过限制
			self.rpmCount = 0
			self.rpmEndTime = now.Add(self.rpmCheckInterval)
		}
	}

	return true
}

func (self *TSession) debugPakcInfo(pack IFNetPacket) {
	mainId, subId := pack.Id()
	tmp := fmt.Sprintf("read pack, sessId=%v, ID=(%v:%v), Datalen=%v", self.SessionId, mainId, subId, len(pack.Data()))
	log.Println(tmp)
}

//投递到管道
func (self *TSession) toChan(pack IFNetPacket) {
	readChan := self.readChan
	if self.needCheckPreferential {
		if self.ServerMsger.IsPreferential(pack) {
			readChan = self.preferentialReadChan
		}
	}

	if self.readDelay > 0 {
		self.delayTimer.Reset(self.readDelay)
		select {
		case readChan <- pack:
			if false {
				self.debugPakcInfo(pack)
			}
		case <-self.delayTimer.C:
			log.Println("warn: room busy or room closed, sessionId:", self.SessionId, " len:", len(readChan))
			return
		}
	} else {
		readChan <- pack
		if false {
			self.debugPakcInfo(pack)
		}
	}
}

func (self *TSession) readLoop(wg *sync.WaitGroup) {
	defer wg.Done()
	defer self.Close()

	if self.readDelay > 0 {
		self.delayTimer = time.NewTimer(self.readDelay)
	}

	self.rpmEndTime = time.Now().Add(self.rpmCheckInterval)

	for {
		select {
		case <-self.ctx.Done():
			return
		case <-self.kickCtx.Done():
			return
		default:
			// 读取超时
			if self.readDelay > 0 {
				self.conn.SetReadDeadline(time.Now().Add(self.readDelay))
			}

			pack, ok := self.readPack()
			if !ok {
				return
			}
			self.toChan(pack)
		}
	}
}

func (self *TSession) sendloop(wg *sync.WaitGroup) {
	defer wg.Done()
	defer self.Close()

	for {
		select {
		case pack := <-self.sendChan:
			if !self.writePack(pack) {
				return
			}
		case <-self.ctx.Done():
			return
		case <-self.kickCtx.Done():
			return
		}
	}
}

// 消息处理
func (self *TSession) msgLoop(wg *sync.WaitGroup) {
	defer wg.Done()
	defer self.Close()

	var pack IFNetPacket

	for {
		select {
		case <-self.kickCtx.Done():
			return
		case <-self.ctx.Done():
			return
		case msg := <-self.readChan:
			pack = msg.(IFNetPacket)
			self.msger.ProcessMsg(pack)
			if len(self.readChan) > 1000 {
				if len(self.readChan)%1000 == 0 {
					log.Println("Server readChan len:", len(self.readChan))
				} else if len(self.readChan) > 5000 {
					msg.(IFNetPacket).Session().Close()
				}
			}
			// cstDataPool.Put(pack.Data())
		}
	}
}

// 消息处理
func (self *TSession) msgPreferentialLoop(wg *sync.WaitGroup) {
	defer wg.Done()
	defer self.Close()

	var pack IFNetPacket

	for {
		select {
		case <-self.kickCtx.Done():
			return
		case <-self.ctx.Done():
			return
		case msg := <-self.preferentialReadChan:
			pack = msg.(IFNetPacket)
			self.ServerMsger.PreferentialProcessMsg(pack)
			if len(self.preferentialReadChan) > 100 {
				if len(self.preferentialReadChan)%100 == 0 {
					log.Println("Server preferentialReadChan len:", len(self.preferentialReadChan))
				} else if len(self.preferentialReadChan) > 500 {
					msg.(IFNetPacket).Session().Close()
				}
			}
			// cstDataPool.Put(pack.Data())
		}
	}
}

func (self *TSession) Close() {
	if atomic.CompareAndSwapInt32(&self.offLine, 0, 1) {
		self.OffLineTime = time.Now().Unix()
		select {
		case self.offChan <- self: // 通知服务器该玩家掉线,或者下线
		case <-time.NewTimer(60 * time.Second).C:
			log.Println("off chan time out, sessionId:", self.SessionId)
		}
		log.Println("close socket:[", self.SessionId, "]", self.conn.RemoteAddr().String())

		self.cancel()
		self.conn.Close()
	}
}

// 设置链接参数
func (self *TSession) SetParameter(readDelay, sendDalay time.Duration, maxRecvSize uint32) {
	self.maxRecvSize = maxRecvSize
	self.readDelay = readDelay
	self.sendDalay = sendDalay
}

// 包频率控制参数
func (self *TSession) SetRpmParameter(checkInterval time.Duration, countLimit uint32, offLineMsg IFNetPacket, kickCount uint32) {
	self.rpmCountLimit = countLimit
	self.rpmCheckInterval = checkInterval
	self.rpmOffLineMsg = offLineMsg
	self.rpmKickCount = kickCount
}

func (self *TSession) HandleConn() {
	// self.conn.SetDeadline(time.Time{})
	if serverMsger, ok := self.msger.(IFServerMsgerEx); ok {
		self.needCheckPreferential = true
		self.ServerMsger = serverMsger
		self.wg.Add(1)
		go self.msgPreferentialLoop(self.wg)
	} else {
		self.needCheckPreferential = false
	}
	self.wg.Add(3)
	go self.readLoop(self.wg)
	go self.sendloop(self.wg)
	go self.msgLoop(self.wg)
}

func NewSession(conn net.Conn, offChan chan *TSession, packGen IFNetPacketGen, msger IFMsger, wg *sync.WaitGroup, kickCtx context.Context) *TSession {
	sess := TSession{}
	sess.conn = conn
	sess.sendChan = make(chan IFNetPacket, 20000)
	sess.readChan = make(chan IFNetPacket, 20000)
	sess.preferentialReadChan = make(chan IFNetPacket, 200)
	sess.offChan = offChan
	sess.offLine = 0
	sess.msger = msger
	sess.SessionId = atomic.AddInt64(&_session_id, -1)
	sess.send_cache = make([]byte, PACKET_LIMIT+HEAD_SIZE)
	sess.packGen = packGen
	sess.ctx, sess.cancel = context.WithCancel(context.Background())
	sess.wg = wg
	sess.kickCtx = kickCtx

	// 默认设置
	// sess.SetParameter(300*time.Second, 15*time.Second, 1<<20)
	sess.maxRecvSize = 1 << 20
	sess.rpmCountLimit = 0 // 默认无限制
	sess.rpmCheckInterval = 0
	sess.OnLineTime = time.Now().Unix()

	sess.packLenBuf = make([]byte, 4)

	return &sess
}
