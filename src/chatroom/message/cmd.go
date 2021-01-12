package message

import (
	"chatroom/app"
	"chatroom/app/global"
	"chatroom/netutil"
	"fmt"
	"log"
	"strings"
	"time"
)

var (
	cmders = map[string]func(cmd string, session *netutil.TSession, user *app.TUser, params []string){}
)

func cmdProc(session *netutil.TSession, user *app.TUser, content string) {
	paramsTmp := strings.Split(content, " ")
	if len(paramsTmp) < 1 {
		log.Println("param error")
		return
	}

	params := make([]string, 0, len(paramsTmp))
	for _, v := range paramsTmp {
		v = strings.TrimSpace(v)
		if len(v) > 0 {
			params = append(params, v)
		}
	}

	cmd := params[0]
	cmdProc := cmders[cmd]
	if cmdProc == nil {
		log.Println("unknow commond:", cmd)
		return
	}
	cmders[cmd](cmd, session, user, params[1:])
}

func OnStatsCmd(cmd string, session *netutil.TSession, user *app.TUser, params []string) {
	if len(params) < 1 {
		log.Println("param error")
		return
	}
	userName := params[0]
	if len(userName) <= 0 {
		log.Println("param error")
		return
	}
	for _, userTmp := range global.Users {
		if userTmp.Name == userName {
			loginDuration := time.Now().Sub(userTmp.LoginTime)
			content := fmt.Sprintf("%02dd %02dh %02dm %02ds\n",
				loginDuration/time.Hour/24, loginDuration/time.Hour%24, loginDuration/time.Minute%60, loginDuration/time.Second%60)
			sendMsg := &TSC_ChatMsg{
				UserName: user.Name,
				Content:  content,
			}
			SendMsg(session, MAIN_ID_MAIN, SC_SUBID_CHAT_MSG, sendMsg)
			log.Println(content)
			return
		}
	}

	log.Println("Can't get user:", userName)
}

func OnPopularCmd(cmd string, session *netutil.TSession, user *app.TUser, params []string) {
	popularWord := global.RecordManger.StatPopular()
	if len(popularWord) > 0 {
		log.Println("Popular is ", popularWord)
	} else {
		log.Println("empty content.")
	}
}

func init() {
	cmders["/stats"] = OnStatsCmd
	cmders["/popular"] = OnPopularCmd
}
