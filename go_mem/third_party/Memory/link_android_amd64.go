//go:build android && amd64 && cgo

package Memory

/*
#cgo LDFLAGS:  -L${SRCDIR}/../../resources/libs/x86_64 -l:libmem.so -llog -ldl -lm -latomic
*/
import "C"
