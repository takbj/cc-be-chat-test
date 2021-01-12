package message

import (
	"chatroom/netutil"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
)

//消息发送方式，1：单发，2：群发
const (
	GATEWAY_POST_SINGLEMSG uint32 = 1
	GATEWAY_POST_MULTIMSG  uint32 = 2

	msgProcErrRecoverFlag = true
)

var (
	s_messageProc map[uint32]*tProc = map[uint32]*tProc{}
)

type tProc struct {
	procParaType reflect.Type
	procFunc     reflect.Value
}

//单路、同步消息处理，消息处理完成后返回
type TSingleMsger struct {
}

func NewSingleMsger() *TSingleMsger {
	self := &TSingleMsger{}

	return self
}

func SendMsg(session *netutil.TSession, mainId uint16, subId uint16, sendMsg interface{}) (bool, error) {
	if sendData, err := json.Marshal(sendMsg); err != nil {
		fmt.Println("json marshal error:", err)
		return false, err
	} else {
		if false {
			fmt.Printf("SendMsg .222, len(sendData)=%v, sendData=%#v\n", len(sendData), sendData)
		}

		pack := netutil.DefaultClientNetPackGener.Get(session).SetId(mainId, subId)
		pack = pack.SetData(sendData)

		session.Send(pack)
	}

	return true, nil
}

func (self *TSingleMsger) ProcessAccept(session *netutil.TSession) {
	processAccept(session)
}

func (self *TSingleMsger) ProcessOffline(session *netutil.TSession) {
	processOffline(session)
}

func (self *TSingleMsger) ProcessMsg(pack netutil.IFNetPacket) {
	defer netutil.PrintPanicStack()

	if false {
		fmt.Println("ProcessMsg.111\n", s_messageProc)
	}

	if false {
		// fmt.Printf("\t\t recive pack,len=%v, model=%v, SubId=%v, data=%#v\n", netutil.HEAD_SIZE+len(msg.Data)+4, msg.MainId, msg.SubId, msg.Data)
		log.Printf("\t\t recive pack,len=%v, model=%v, SubId=%v, data=%#v\n",
			netutil.HEAD_SIZE+len(pack.Data())+4, pack.Id2()>>16, uint16(pack.Id2()), pack.Data())
	}

	if msgProcData, exist := s_messageProc[pack.Id2()]; exist {
		if msgProcData != nil {
			callProc(msgProcData, pack)
		} else {
			mid, subid := pack.Id()
			log.Println("SubId not process in Server:", mid, subid)

			// if false {
			// 	pack.Session.Send(msg)
			// }
		}
	} else {
		mid, subid := pack.Id()
		log.Println("msg proc not find,", mid, subid, s_messageProc)
	}
}

func callProc(msgProcData *tProc, pack netutil.IFNetPacket) (ok bool, err error) {
	if false {
		log.Println("callProc 111 pack.CheckValue()=", pack.CheckValue())
	}
	defer func() {
		if msgProcErrRecoverFlag {
			if e := recover(); e != nil {
				outStr := fmt.Sprintf("callProc error,MainId=%v, SubId=%v, err=%v\n, \tpack.Data=%v", pack.Id2(), pack.Data(), e)
				if false {
					fmt.Println(outStr)
				}
				log.Println(outStr)
				err = fmt.Errorf("%v", e)
			}
		}

		if false {
			log.Println("callProc 222 pack.CheckValue()=", pack.CheckValue())
		}

		pack.Gen().Put(pack)
	}()

	if msgProcData.procParaType != nil && msgProcData.procParaType.Kind() != reflect.Invalid {
		receiveMsg := reflect.New(msgProcData.procParaType)
		if err := json.Unmarshal(pack.Data(), receiveMsg.Interface()); err != nil {
			log.Println("pb Unmarshal error:", err)
			return false, err
		}

		rets := msgProcData.procFunc.Call(
			[]reflect.Value{
				reflect.ValueOf(receiveMsg.Interface()),
			})
		if rets[1].Interface() != nil {
			log.Println("proc message error:", rets[1].Interface().(error).Error())
		}
	} else {
		msgProcData.procFunc.Call(
			[]reflect.Value{})
	}

	return true, nil
}

func Send2Session(sess *netutil.TSession,
	mainId uint16, subId uint16,
	sendMsg interface{}, packGener netutil.IFNetPacketGen) (bool, error) {
	if sendData, err := json.Marshal(sendMsg); err != nil {
		log.Println("pb marshal error:", err)
		return false, err
	} else {
		//TODO:改成pool
		if false {
			log.Println("Send2Session:", len(sendData), mainId, subId, sendData)
		}
		sess.Send(packGener.Get(sess).
			SetId(mainId, subId).
			SetData(sendData))
	}

	return true, nil
}

func SendPackBySession(pack netutil.IFNetPacket) (bool, error) {
	pack.Session().Send(pack)
	return true, nil
}

func RegMsgProc(mainId, subId uint16, msgProc interface{}) {
	routeCmd := netutil.GetRouteCmd(mainId, subId)
	if _, exist := s_messageProc[routeCmd]; exist {
		panic(fmt.Errorf("repleat mainId and subId: 0X%4x.0X%4x, routeCmd=%v", mainId, subId, routeCmd))
	}

	msgProcValue := reflect.TypeOf(msgProc)
	switch msgProcValue.Kind() {
	case reflect.Func:
	default:
		panic("the msgProc mast be function")
	}

	if msgProcValue.NumIn() <= 0 || msgProcValue.NumIn() > 2 {
		panic("the param num of the msgProc mast be 1 or 2.")
	}
	if msgProcValue.In(0) != reflect.TypeOf((*netutil.TSession)(nil)) {
		panic("The first in parameters of the msgProc mast be *netutil.TSession type!")
	}

	var procParaTypeTmp reflect.Type
	if msgProcValue.NumIn() == 2 {
		procParaTypeTmp = msgProcValue.In(1).Elem()
	}
	if msgProcValue.NumOut() != 2 {
		panic("The out parameters of the msgProc mast be count == 2!")
	}
	if msgProcValue.Out(0).Kind() != reflect.Bool {
		panic("The first out parameters of the msgProc mast be an bool type!")
	}
	if msgProcValue.Out(1).Name() != "error" {
		panic(fmt.Sprintf("The second out parameters of the msgProc mast be an error type!", msgProcValue.Out(1).Name()))
	}

	s_messageProc[routeCmd] = &tProc{
		procParaType: procParaTypeTmp,
		procFunc:     reflect.ValueOf(msgProc),
	}

}

func AllRouteCmds() []uint32 {
	allRouteCmds := []uint32{}
	for routeCmd, _ := range s_messageProc {
		allRouteCmds = append(allRouteCmds, routeCmd)
		if false {
			mainId, subId := netutil.GetIds(routeCmd)
			fmt.Println("AllRouteCmds:", mainId, subId)
		}
	}
	return allRouteCmds
}
