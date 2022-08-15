package main

import (
	"log"
	"os"

	"github.com/alipourhabibi/restream/rtmp"
	"github.com/alipourhabibi/restream/settings"
)

func main() {
	settings.SetUp()

	// Setting Up logger
	logfile, err := os.OpenFile("general.log", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}
	l := log.Logger{}
	l.SetOutput(logfile)

	stream := rtmp.NewStream(&l)
	stream.InitStream()
}
