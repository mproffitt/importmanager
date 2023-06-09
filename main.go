package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	c "github.com/mproffitt/importmanager/pkg/config"
	m "github.com/mproffitt/importmanager/pkg/mime"
	p "github.com/mproffitt/importmanager/pkg/processing"

	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

type event struct {
	event   notify.Event
	time    time.Time
	details m.Details
}

func setupWatches(config *c.Config) {
	channels := make([]chan notify.EventInfo, len(config.Watch))
	for i := 0; i < len(config.Watch); i++ {
		channels[i] = make(chan notify.EventInfo, 1)
		go func(path string, processors []c.Processor, channel chan notify.EventInfo) {
			if err := notify.Watch(path, channel, notify.All, notify.InCloseWrite); err != nil {
				log.Fatal(err)
				return
			}
			log.Info("Starting listening to: ", path)
			var paths = make(map[string]event)
			for {
				select {
				case ei := <-channel:
					switch ei.Event() {
					case notify.Remove:
						delete(paths, ei.Path())
					default:
						if _, err := os.Stat(ei.Path()); err == nil {
							var details = m.Catagories.FindCatagoryFor(ei.Path())
							log.Debug(details)
							if details.Type != "application/x-partial-download" {
								paths[ei.Path()] = event{
									event:   ei.Event(),
									time:    time.Now(),
									details: *details,
								}
							}
						}
					}
				case <-time.After(5 * time.Second):
					for p, e := range paths {
						// Wait 5 seconds after last event before handling
						if e.time.Before(time.Now().Add(-config.DelayInSeconds * time.Second)) {
							delete(paths, p)
							go handlePath(p, e.details, processors, config.CleanupZeroByte)
						}
					}
				}
			}
		}(config.Watch[i], config.Processors, channels[i])
	}
}

func handlePath(path string, details m.Details, processors []c.Processor, czb bool) {
	log.Info("Handling path", path)
	if fi, err := os.Stat(path); err == nil {
		if fi.Size() == 0 && czb {
			log.Infof("Deleting path '%s'. File is empty", path)
			os.Remove(path)
			return
		}
	}

	var processor *c.Processor

	// try find an exact processor for this mimetype
	for i, p := range processors {
		if p.Type == details.Type {
			processor = &processors[i]
			break
		}
	}

	// If we don't have an exact match and this is a subclass, try that
	if processor == nil && details.IsSubClass() {
		for i, p := range processors {
			if details.IsSubClassOf(p.Type) {
				processor = &processors[i]
				break
			}
		}
	}

	// If we still don't have a processor fall back to catagory level
	if processor == nil {
		for i, p := range processors {
			if p.Type == details.Catagory {
				processor = &processors[i]
				break
			}
		}
		if processor == nil {
			log.Errorf("No processor defined for type '%s | %s | %s'", details.Type, details.SubClass, details.Catagory)
			return
		}

		log.Infof("Found processor %s for path %s", processor.String(), path)
		p.Process(path, &details, processor)
	}
}

func main() {
	var (
		filename string
		config   *c.Config
		err      error
	)
	sigc := make(chan os.Signal, 1)
	done := make(chan bool)
	signal.Notify(sigc, os.Interrupt)

	go func() {
		for range sigc {
			log.Info("Shutting down listeners")
			done <- true
		}
	}()

	flag.StringVar(&filename, "config", "", "Path to config file")
	flag.Parse()
	if _, err = os.Stat(filename); err != nil || filename == "" {
		log.Fatalf("config file must be provided and must exist")
		os.Exit(1)
	}

	if config, err = c.New(filename); err != nil {
		log.Fatalf("Config file is invalid or doesn't exist. %q", err)
		os.Exit(1)
	}
	log.SetLevel(log.DebugLevel)

	log.Debug(fmt.Sprintf("%+v", config))
	log.Info("Starting watchers")
	setupWatches(config)
	<-done
}
