package socketServer

import (
	"chatroom/netutil"
	"context"
	"fmt"
	"log"
	"net"
	"sync"
)

const (
	DEBUG_FLAG bool = true
)

type TSocketServer struct {
	listernAddr string

	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	listener *net.TCPListener

	msger netutil.IFServerMsger

	sessions sync.Map //map[int64]*netutil.TSession

	active     bool                   //是否激活
	OffChan    chan *netutil.TSession // 下线、掉线的玩家
	acceptChan chan *netutil.TSession
	packgener  netutil.IFNetPacketGen //包生成器
}

func NewServer(addr string) *TSocketServer {
	return &TSocketServer{
		OffChan:    make(chan *netutil.TSession, 1000),
		acceptChan: make(chan *netutil.TSession, 1000),
		// sessions: map[int64]*netutil.TSession{},
		active: false,

		listernAddr: addr,
	}
}

// start server loop
func (self *TSocketServer) Run(msger netutil.IFServerMsger, packgener netutil.IFNetPacketGen) (err error) {
	defer netutil.PrintPanicStack()

	tcpAddr, err := net.ResolveTCPAddr("tcp", self.listernAddr)
	if nil != err {
		return err
	}

	self.msger = msger
	self.active = true
	self.packgener = packgener

	self.listener, err = net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return err
	}

	outStr := fmt.Sprintf("------------ server (%v) Start Success ------------", tcpAddr.String())
	log.Println("\n", outStr)

	self.ctx, self.cancel = context.WithCancel(context.Background())

	self.wg.Add(1)
	go self.loopAccept()

	self.wg.Add(1)
	go self.loopLoginOut()

	self.wg.Wait()

	outStr = fmt.Sprintf("============== server  (%v) Close  ==============", tcpAddr.String())
	log.Println("\n", outStr)

	return nil
}

// accetp
func (self *TSocketServer) loopAccept() {
	defer self.wg.Done()

	for {
		select {
		case <-self.ctx.Done():
			return
		default:
			conn, err := self.listener.AcceptTCP()
			if err != nil {
				if self.active {
					log.Println("Error：accept failed:", err)
				}
				continue
			}

			if nil == conn {
				continue
			}

			log.Println("accept client:", conn.RemoteAddr().String())
			if DEBUG_FLAG {
				log.Println("accept client:", conn.RemoteAddr().String())
			}

			conn.SetNoDelay(true)
			// 创建会话
			self.acceptChan <- netutil.NewSession(conn, self.OffChan, self.packgener, self.msger, &self.wg, self.ctx)
		}
	}
}

func (self *TSocketServer) loopLoginOut() {
	defer self.wg.Done()

	for {
		select {
		case <-self.ctx.Done():
			return
		case offlineSession, ok := <-self.OffChan:
			if !ok {
				return
			}
			self.msger.ProcessOffline(offlineSession)
			self.sessions.Delete(offlineSession.SessionId)
		case acceptSession, ok := <-self.OffChan:
			if !ok {
				return
			}
			self.sessions.Store(acceptSession.SessionId, acceptSession)
			self.msger.ProcessAccept(acceptSession)
			acceptSession.HandleConn()
		}
	}
}

func (self *TSocketServer) RangeSession(f func(session *netutil.TSession) bool) {
	self.sessions.Range(func(key, value interface{}) bool {
		return f(value.(*netutil.TSession))
	})
}

func (self *TSocketServer) Close() {
	self.active = false
	self.listener.Close()
	self.cancel()
}
