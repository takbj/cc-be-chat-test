package netutil

import (
// "errors"
// "math"
)

const (
	PACKET_LIMIT = 1 << 20 << 2 //4M数据
	PACKET_POOL  = 10000
)

func GetRouteCmd(mainId, subId uint16) (routeCmd uint32) {
	return (uint32(mainId) << 16) | uint32(subId)
}

func GetIds(routeCmd uint32) (mainId, subId uint16) {
	mainId = uint16(routeCmd >> 16)
	subId = uint16(routeCmd)
	return
}
