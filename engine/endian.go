package engine

import (
	"encoding/binary"
	"unsafe"
)

// 确定了的当前机器大小端编码器
var endian binary.ByteOrder

func init() {
	const INT_SIZE = int(unsafe.Sizeof(0))
	var i int = 0x1
	bs := (*[INT_SIZE]byte)(unsafe.Pointer(&i))
	if bs[0] == 0 {
		endian = binary.BigEndian
	} else {
		endian = binary.LittleEndian
	}
}
