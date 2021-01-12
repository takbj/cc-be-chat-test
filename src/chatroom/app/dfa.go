package app

import ()

// DFA算法实现

type IFDirtyFilter interface {
	Replace(text string, delim rune) (string, error)
}

type tDfaNode struct {
	end   bool
	child map[rune]*tDfaNode
}

func newNode() *tDfaNode {
	return &tDfaNode{
		child: make(map[rune]*tDfaNode),
	}
}

type TDfaFilter struct {
	root *tDfaNode
}

func NewFilter(textList []string) *TDfaFilter {
	filter := &TDfaFilter{
		root: newNode(),
	}
	for i, l := 0, len(textList); i < l; i++ {
		filter.addDirtyWords(textList[i])
	}
	return filter
}

func (filter *TDfaFilter) addDirtyWords(text string) {
	n := filter.root
	uchars := []rune(text)
	for i, l := 0, len(uchars); i < l; i++ {
		if _, ok := n.child[uchars[i]]; !ok {
			n.child[uchars[i]] = newNode()
		}
		n = n.child[uchars[i]]
	}
	n.end = true
}

func (nf *TDfaFilter) Replace(text string, delim rune) (string, error) {
	uchars := []rune(text)
	idexs := nf.doIndexes(uchars)
	if len(idexs) == 0 {
		return "", nil
	}
	for i := 0; i < len(idexs); i++ {
		uchars[idexs[i]] = rune(delim)
	}
	return string(uchars), nil
}

func (filter *TDfaFilter) doIndexes(uchars []rune) (idexs []int) {
	var (
		tIdexs []int
		ul     = len(uchars)
		n      = filter.root
	)
	for i := 0; i < ul; i++ {
		if _, ok := n.child[uchars[i]]; !ok {
			continue
		}
		n = n.child[uchars[i]]
		tIdexs = append(tIdexs, i)
		if n.end {
			idexs = filter.appendTo(idexs, tIdexs)
			tIdexs = nil
		}
		for j := i + 1; j < ul; j++ {
			if _, ok := n.child[uchars[j]]; !ok {
				break
			}
			n = n.child[uchars[j]]
			tIdexs = append(tIdexs, j)
			if n.end {
				idexs = filter.appendTo(idexs, tIdexs)
			}
		}
		if tIdexs != nil {
			tIdexs = nil
		}
		n = filter.root
	}
	return
}

func (filter *TDfaFilter) appendTo(dst, src []int) []int {
	var t []int
	for i, srcSize := 0, len(src); i < srcSize; i++ {
		var exist bool
		for j, dstSize := 0, len(dst); j < dstSize; j++ {
			if src[i] == dst[j] {
				exist = true
				break
			}
		}
		if !exist {
			t = append(t, src[i])
		}
	}
	return append(dst, t...)
}
