//go:build android && 386 && cgo

package Memory

/*
#cgo LDFLAGS:  -L${SRCDIR}/../../resources/libs/x86 -l:libmem.so -llog -ldl -lm -latomic
*/
import "C"
