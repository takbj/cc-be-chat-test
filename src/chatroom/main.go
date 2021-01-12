package main

import (
	"chatroom/app"
	"chatroom/app/global"
	"chatroom/message"
	"chatroom/netutil"
	"chatroom/socketServer"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
)

const (
	cstListenAddr  = "0.0.0.0:9999"
	filterListFile = "list.txt"
)

func main() {
	loadFilterListFromFile(filterListFile)
	global.Server = socketServer.NewServer(cstListenAddr)
	if err := global.Server.Run(message.NewSingleMsger(), netutil.DefaultClientNetPackGener); err != nil {
		log.Println("run server error:", err.Error())
	}
}

func loadFilterListFromFile(fileName string) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		panic(fmt.Sprintf("\t\"%s\" load failed:%v", fileName, err.Error()))
		return
	}

	textList := strings.Split(string(data), "\r\n")
	if len(textList) <= 1 {
		textList = strings.Split(string(data), "\n")
	}
	global.DfaFilter = app.NewFilter(textList)
	log.Printf("Load filter file %s success.\n", fileName)
}
