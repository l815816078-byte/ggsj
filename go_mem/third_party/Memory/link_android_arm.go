//go:build android && arm && cgo

package Memory

/*
#cgo LDFLAGS: -L${SRCDIR}/../../resources/libs/armeabi-v7a -l:libmem.so -llog -ldl -lm -latomic
*/
import "C"
