package fs

import (
	"fmt"
	"log"
	"runtime"

	"github.com/billziss-gh/cgofuse/fuse"
)

func (fs *FileSystem) log() bool {
	return fs.shouldLog
}

func traceJoin(deref bool, vals []interface{}) string {
	rslt := ""
	for _, v := range vals {
		if deref {
			switch i := v.(type) {
			case *bool:
				rslt += fmt.Sprintf(", %#v", *i)
			case *int:
				rslt += fmt.Sprintf(", %#v", *i)
			case *int8:
				rslt += fmt.Sprintf(", %#v", *i)
			case *int16:
				rslt += fmt.Sprintf(", %#v", *i)
			case *int32:
				rslt += fmt.Sprintf(", %#v", *i)
			case *int64:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uint:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uint8:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uint16:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uint32:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uint64:
				rslt += fmt.Sprintf(", %#v", *i)
			case *uintptr:
				rslt += fmt.Sprintf(", %#v", *i)
			case *float32:
				rslt += fmt.Sprintf(", %#v", *i)
			case *float64:
				rslt += fmt.Sprintf(", %#v", *i)
			case *complex64:
				rslt += fmt.Sprintf(", %#v", *i)
			case *complex128:
				rslt += fmt.Sprintf(", %#v", *i)
			case *string:
				rslt += fmt.Sprintf(", %#v", *i)
			default:
				rslt += fmt.Sprintf(", %#v", v)
			}
		} else {
			rslt += fmt.Sprintf(", %#v", v)
		}
	}
	if len(rslt) > 0 {
		rslt = rslt[2:]
	}
	return rslt
}

func (fs *FileSystem) trace(vals ...interface{}) func(vals ...interface{}) {
	uid, gid, _ := fuse.Getcontext()

	pc, _, _, ok := runtime.Caller(2)
	name := "< ---- >"
	if ok {
		fn := runtime.FuncForPC(pc)
		name = fn.Name()
	}
	prfx := fmt.Sprintf("[uid=%v,gid=%v]: ", uid, gid)
	args := traceJoin(false, vals)
	return func(vals ...interface{}) {
		form := "%v%v(%v) = %v"
		rslt := ""
		rcvr := recover()
		if nil != rcvr {
			rslt = fmt.Sprintf("!PANIC:%v", rcvr)
		} else {
			if len(vals) != 1 {
				form = "%v%v(%v) = (%v)"
			}
			rslt = traceJoin(true, vals)
		}
		log.Printf(form, prfx, name, args, rslt)
		if nil != rcvr {
			panic(rcvr)
		}
	}

}
