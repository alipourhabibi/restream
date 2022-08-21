package main

import (
	"log"
	"os"
	"os/signal"

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
	defer logfile.Close()

	l := log.Logger{}
	l.SetFlags(log.Ltime | log.Ldate | log.LUTC)
	if settings.ServerSettings.Items.RunMode == "Release" {
		l.SetOutput(logfile)
	} else {
		// Debug
		l.SetOutput(os.Stderr)
	}

	stream := rtmp.NewStream(&l)
	stream.InitStream()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c
	os.Exit(0)
}
