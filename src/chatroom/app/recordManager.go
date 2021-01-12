package app

import (
	"strings"
	"sync"
	"time"
)

type TRecordNode struct {
	Content string
	User    *TUser
	Time    time.Time
	pre     *TRecordNode
	next    *TRecordNode
}

type TRecordManger struct {
	frontNode  *TRecordNode
	limitIndex *TRecordNode //前50最起始索引
	backNode   *TRecordNode
	recordSize int           //保留记录最多的条数
	recoreTime time.Duration //保留记录最多的时长
	nodeCnt    int
	mu         sync.RWMutex
	recordCh   chan *TRecordNode
}

func NewRecordManger(recordMaxSize int, recordMaxSecond time.Duration) *TRecordManger {
	rm := &TRecordManger{
		backNode:   nil,
		frontNode:  nil,
		recordSize: recordMaxSize,
		recoreTime: recordMaxSecond,
		recordCh:   make(chan *TRecordNode, 256),
	}
	go rm.Run()

	return rm
}

func (rm *TRecordManger) Run() {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			//TODO:清理过期数据
		case record := <-rm.recordCh:
			rm.realAddRecord(record)
		}
	}
}

func (rm *TRecordManger) AddRecord(user *TUser, t time.Time, chat string) {
	rm.recordCh <- &TRecordNode{
		Content: chat,
		User:    user, // *TUser
		Time:    t,
		pre:     rm.frontNode, // *TRecordNode
		next:    nil,          // *TRecordNode
	}
}

func (rm *TRecordManger) realAddRecord(record *TRecordNode) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.nodeCnt == 0 {
		rm.frontNode = record
		rm.limitIndex = record
		rm.frontNode = record
		return
	}

	rm.nodeCnt++
	rm.frontNode.next = rm.frontNode
	rm.frontNode = record
	if rm.nodeCnt > rm.recordSize {
		rm.limitIndex = rm.limitIndex.next
	}
}

type FEachRecordProc func(User *TUser, Content string) bool

func (rm *TRecordManger) RangeRecord(fp FEachRecordProc) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	index := rm.limitIndex
	for i := 0; i < rm.nodeCnt && i < rm.recordSize && index != nil; i++ {
		if !fp(index.User, index.Content) {
			return
		}
		index = index.next
	}
}

func (rm *TRecordManger) StatPopular() string {
	startTimes := time.Now().Add(-rm.recoreTime)
	statResult := map[string]int{}
	var maxWord string
	var maxWordCnt int
	for index := rm.frontNode; index != nil && index.Time.After(startTimes); index = index.pre {
		wordList := strings.Split(index.Content, " ")
		for _, word := range wordList {
			statResult[word]++
			if statResult[word] > maxWordCnt {
				maxWord = word
				maxWordCnt = statResult[word]
			}
		}
	}

	return maxWord
}
