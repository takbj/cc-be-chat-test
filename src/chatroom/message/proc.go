package message

import (
	"chatroom/app"
	"chatroom/app/global"
	"chatroom/netutil"
	"fmt"
	"log"
	"time"
)

const (
	MAIN_ID_MAIN = 0x01
)
const (
	SC_SUBID_ADD_USER        = 0x1  //新进来用户
	CS_SUBID_CHAT_MSG        = 0x11 //上行聊天消息
	SC_SUBID_CHAT_MSG        = 0x12 //下行聊天消息
	SC_SUBID_RECORD_CHAT_MSG = 0x14 //下行历史聊天消息
)

type TSC_ChatMsg struct {
	UserName string
	Content  string
}

type TSC_AddUser struct {
	UserName string
}

func processAccept(session *netutil.TSession) {
	//TODO:发送50条历史消息
	recordMsgs := []*TSC_ChatMsg{}
	global.RecordManger.RangeRecord(func(user *app.TUser, content string) bool {
		recordMsgs = append(recordMsgs, &TSC_ChatMsg{
			UserName: user.Name,
			Content:  content,
		})
		return true
	})
	SendMsg(session, MAIN_ID_MAIN, SC_SUBID_RECORD_CHAT_MSG, recordMsgs)

	userName := fmt.Sprintf("user_%v", session.SessionId)
	user := app.NewUser(userName, session.SessionId)
	global.Users[session.SessionId] = user
	addUserMsg := &TSC_AddUser{
		UserName: user.Name,
	}
	global.Server.RangeSession(func(session *netutil.TSession) bool {
		SendMsg(session, MAIN_ID_MAIN, SC_SUBID_ADD_USER, addUserMsg)
		return true
	})
}

func processOffline(session *netutil.TSession) {
	delete(global.Users, session.SessionId)
}

type TChatMsg struct {
	Content string
}

func OnChatProc(session *netutil.TSession, msg *TChatMsg) (bool, error) {
	user, exist := global.Users[session.SessionId]
	if !exist || user == nil {
		err := fmt.Errorf("unknow user(sid=%v)", session.SessionId)
		log.Println(err.Error())
		return false, err
	}

	if []rune(msg.Content)[0] == rune('/') {
		cmdProc(session, user, msg.Content)
		return true, nil
	}

	content, err := global.DfaFilter.Replace(msg.Content, rune('*'))
	if err != nil {
		err := fmt.Errorf("call global.DfaFilter.Replace error:%s, \nrawString=%#v", err.Error(), msg.Content)
		log.Println(err.Error())
		return false, err
	}
	global.RecordManger.AddRecord(user, time.Now(), content)

	sendMsg := &TSC_ChatMsg{
		UserName: user.Name,
		Content:  content,
	}
	global.Server.RangeSession(func(session *netutil.TSession) bool {
		SendMsg(session, MAIN_ID_MAIN, SC_SUBID_CHAT_MSG, sendMsg)
		return true
	})
	return true, nil
}

func init() {
	RegMsgProc(0, CS_SUBID_CHAT_MSG, OnChatProc)
}
