package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
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

type watch struct {
	stop     chan bool
	complete chan bool
	events   chan notify.EventInfo
}

func setupWatches(config *c.Config, stop, finished chan bool) {
	channels := make([]watch, len(config.Watch))
	for i := 0; i < len(config.Watch); i++ {
		channels[i] = watch{
			stop:     make(chan bool, 1),
			complete: make(chan bool, 1),
			events:   make(chan notify.EventInfo),
		}
		go watchLocation(config.Watch[i], channels[i], config)
	}
	<-stop
	for i := 0; i < len(channels); i++ {
		channels[i].stop <- true
		<-channels[i].complete
	}
	finished <- true
}

type job struct {
	path       string
	details    m.Details
	processors []c.Processor
	czb        bool
}

func watchLocation(path string, channel watch, config *c.Config) {
	var jobs chan job = make(chan job)
	var wg sync.WaitGroup
	for i := 0; i < config.BufferSize; i++ {
		wg.Add(1)
		log.Debugf("Starting %s worker %d", path, i)
		go pathWorker(jobs, &wg)
	}
	if err := notify.Watch(path, channel.events, notify.All, notify.InCloseWrite); err != nil {
		log.Fatal(err)
		return
	}
	log.Info("Starting listening to: ", path)
	var paths = make(map[string]event)
	for {
		select {
		case ei := <-channel.events:
			switch ei.Event() {
			case notify.Remove:
				delete(paths, ei.Path())
			default:
				if _, err := os.Stat(ei.Path()); err != nil {
					continue
				}
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
		case <-time.After(config.DelayInSeconds * time.Second):
			// TODO: when moving large numbers of files, the number of paths
			// may be fairly extensive. Probably want to implement a queue
			// here to handle files in batches and prevent overloading
			for p, e := range paths {
				// Wait 5 seconds after last event before handling
				if e.time.Before(time.Now().Add(-config.DelayInSeconds * time.Second)) {
					delete(paths, p)
					jobs <- job{
						path:       p,
						details:    e.details,
						processors: config.Processors,
						czb:        config.CleanupZeroByte,
					}
				}
			}
		case <-channel.stop:
			log.Infof("Shutting down listener for path %s", path)
			close(jobs)
			wg.Wait()
			notify.Stop(channel.events)
			channel.complete <- true
		}
	}
}

func pathWorker(jobs chan job, wg *sync.WaitGroup) {
	var j job = <-jobs
	defer wg.Done()
	if j.path == "" && len(j.processors) == 0 {
		return
	}
	handlePath(j.path, j.details, j.processors, j.czb)
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
	}

	log.Infof("Found processor '%s' for path %s", processor.String(), path)
	if err := p.Process(path, &details, processor); err != nil {
		log.Error(err)
	}
	log.Infof("Completed parsing for %s", path)
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
				done <- true
			}
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

	log.Debug(fmt.Sprintf("%+v", config))
	log.Info("Starting watchers")
	setupWatches(config, stop, finished)
	<-done
	log.Info("Done")
}
