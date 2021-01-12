package netutil

import (
	"fmt"
	"log"
	"runtime"
)

func PrintPanicStack() {
	if x := recover(); x != nil {
		for i := 0; ; i++ {
			funcName, file, line, ok := runtime.Caller(i)
			if ok {
				log.Println(fmt.Sprintf("error:frame %v:[func:%v,file:%v,line:%v]", i, runtime.FuncForPC(funcName).Name(), file, line))
			} else {
				break
			}
		}
		log.Println(fmt.Sprintf("error:%v", x))
		if true {
			panic(x)
		}
	}
}
