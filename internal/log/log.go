package log

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
)

var log *os.File

func Log(msg interface{}) {
	spew.Fdump(log, msg)
}

func LogRaw(msg string) {
	fmt.Fprintln(log, msg)
}

func init() {
	var err error
	log, err = os.OpenFile("gh-my.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		panic(err)
	}
}
