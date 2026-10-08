//go:build android && arm64 && cgo

package Memory

/*
#cgo LDFLAGS:  -L${SRCDIR}/../../resources/libs/arm64-v8a -l:libmem.so -llog -ldl -lm -latomic
*/
import "C"
