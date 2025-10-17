package routedata

import (
	"fmt"

	"github.com/dave/gpt/globals"
)

func logln(a ...interface{}) {
	if globals.LOG {
		fmt.Println(a...)
	}
}

func logf(format string, a ...interface{}) {
	if globals.LOG {
		fmt.Printf(format, a...)
	}
}

func debugln(a ...interface{}) {
	if globals.DEBUG {
		fmt.Println(a...)
	}
}

func debugf(format string, a ...interface{}) {
	if globals.DEBUG {
		fmt.Printf(format, a...)
	}
}

func debugfln(format string, a ...interface{}) {
	if globals.DEBUG {
		fmt.Printf(format+"\n", a...)
	}
}
