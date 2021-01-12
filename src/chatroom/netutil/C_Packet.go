//与客户端通信的包

package netutil

import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"
)

const (
	HEAD_SIZE = 8
)

var (
	DefaultClientNetPackGener = &TClientNetPacketGen{
		packPool: sync.Pool{
			New: func() interface{} {
				return &tClientNetPacket{}
			},
		},
	}
)

type TClientNetPacketGen struct {
	packPool sync.Pool
}

func (packGen *TClientNetPacketGen) Get(session *TSession) IFNetPacket {
	newPack := packGen.packPool.Get().(*tClientNetPacket)
	newPack.session = session
	newPack.gen = packGen

	return newPack
}

func (packGen *TClientNetPacketGen) Put(pack IFNetPacket) {
	packGen.packPool.Put(pack)
}

type tClientNetPacket struct {
	mainId      uint16
	subId       uint16
	dataLen     uint32 //TODO:暂时为了配合客户端在此占位
	data        []byte
	writeOkProc func(IFNetPacket) //消息写进net时的回调
	session     *TSession
	gen         IFNetPacketGen
}

func (pack *tClientNetPacket) Type() E_PACKET_TYPE {
	return CLIENT_PACKET_TYPE
}

func (pack *tClientNetPacket) Gen() IFNetPacketGen {
	return pack.gen
}

func (pack *tClientNetPacket) Parse(data []byte) (ok bool) {
	if len(data) < HEAD_SIZE {
		log.Println("error receiving packLen:", len(data), " sessionId:", pack.session.SessionId)
		return false
	}
	if false {
		fmt.Printf("tClientNetPacket.Parse 111")
	}

	pack.parseHead(data)
	pack.data = data[HEAD_SIZE:]
	return true
}

func (pack *tClientNetPacket) SetSession(sess *TSession) {
	pack.session = sess
}

func (pack *tClientNetPacket) Session() *TSession {
	return pack.session
}

func (pack *tClientNetPacket) Id() (mainId uint16, subId uint16) {
	return pack.mainId, pack.subId
}

func (pack *tClientNetPacket) Id2() uint32 {
	return uint32(pack.mainId)<<16 | uint32(pack.subId)
}

func (pack *tClientNetPacket) SetId(mainId, subId uint16) IFNetPacket {
	pack.mainId = mainId
	pack.subId = subId

	return pack
}

func (pack *tClientNetPacket) SetId2(id uint32) IFNetPacket {
	pack.mainId = uint16(id >> 16)
	pack.subId = uint16(id)

	return pack
}

func (pack *tClientNetPacket) Data() []byte {
	return pack.data
}

func (pack *tClientNetPacket) SetData(data []byte) IFNetPacket {
	pack.data = data
	return pack
}

func (pack *tClientNetPacket) SetWriteOkProc(proc func(IFNetPacket)) IFNetPacket {
	pack.writeOkProc = proc
	return pack
}

func (pack *tClientNetPacket) GetWriteOkProc() func(IFNetPacket) {
	return pack.writeOkProc
}

func (pack *tClientNetPacket) SetCheckValue(v uint32) IFNetPacket {
	// pack.checkValue = v
	return pack
}

func (pack *tClientNetPacket) CheckValue() uint32 {
	// return pack.checkValue
	return 0
}

func (pack *tClientNetPacket) parseHead(data []byte) (ok bool) {

	headerPos := 0

	pack.mainId = binary.LittleEndian.Uint16(data[headerPos:])
	headerPos += 2

	pack.subId = binary.LittleEndian.Uint16(data[headerPos:])
	headerPos += 2

	// pack.Time = binary.LittleEndian.Uint64(data[headerPos:])
	// headerPos += 8

	pack.dataLen = binary.LittleEndian.Uint32(data[headerPos:])
	headerPos += 4

	if false {
		fmt.Printf("tClientNetPacket.parseHead   pack.mainId=%v\n", pack.mainId)
	}
	pack.data = data[headerPos:]
	return true
}

func (pack *tClientNetPacket) Write2Cache(cache []byte) (writeLen int) {
	pos := 0

	// 2字节模块id
	binary.LittleEndian.PutUint16(cache[pos:], pack.mainId)
	pos += 2

	// 2字节消息id
	binary.LittleEndian.PutUint16(cache[pos:], pack.subId)
	pos += 2

	// 8字节
	// binary.LittleEndian.PutUint64(cache[pos:], pack.Time)
	// pos += 8
	pack.dataLen = uint32(len(pack.data))
	binary.LittleEndian.PutUint32(cache[pos:], pack.dataLen)
	pos += 4

	copy(cache[pos:], pack.data)
	pos += len(pack.data)
	if false {
		tmp := fmt.Sprintf("Write2Cache.000, pos=%v, len(data)=%v, data=%#v", pos, len(pack.data), pack.data)
		fmt.Println(tmp)
	}

	return pos
}
