package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	n "github.com/0xAX/notificator"
	c "github.com/mproffitt/importmanager/pkg/config"
	h "github.com/mproffitt/importmanager/pkg/handler"
	log "github.com/sirupsen/logrus"
)

func notification(notification chan string) {
	var note *n.Notificator = n.New(n.Options{
		DefaultIcon: "icon/default.png",
		AppName:     "ImportManager",
	})

	for {
		log.Debug("Checking for notification message")
		select {
		case msg := <-notification:
			log.Infof("Sending message %s to notification system", msg)
			note.Push("ImportManager", msg, "/home/user/icon.png", n.UR_NORMAL)
		}
	}
}

func main() {
	var (
		filename string
		config   *c.Config
		err      error
		sigc     chan os.Signal = make(chan os.Signal, 1)
		stop     chan bool      = make(chan bool, 1)
		finished chan bool      = make(chan bool, 1)
		done     chan bool      = make(chan bool, 1)
	)
	signal.Notify(sigc, os.Interrupt)

	go func() {
		for range sigc {
			log.Info("Shutting down listeners")
			stop <- true
			if <-finished {
				log.Info("Done")
				done <- true
			}
		}
	}()

	flag.StringVar(&filename, "config", "", "Path to config file")
	flag.Parse()
	if _, err = os.Stat(filename); err != nil || filename == "" {
		log.Fatalf("config file must be provided and must exist")
		return
	}

	if config, err = c.New(filename, h.Handle, true); err != nil {
		log.Fatalf("Config file is invalid or doesn't exist. %q", err)
		return
	}

	var notifications chan string = make(chan string)
	go notification(notifications)

	log.Debug(fmt.Sprintf("%+v", config))
	log.Info("Starting watchers")
	h.Setup(config, stop, finished, notifications)
	<-done
}
