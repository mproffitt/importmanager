package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	n "github.com/0xAX/notificator"
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

type job struct {
	path       string
	details    m.Details
	processors []c.Processor
	czb        bool
	ready      bool
	complete   chan bool
}

type lockable struct {
	sync.RWMutex
	paths map[string]event
}

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

func contains(what string, where []string) bool {
	for _, p := range where {
		if what == p {
			return true
		}
	}
	return false
}

func setupPaths(config *c.Config, stop, finished chan bool) {
	channels := make(map[string]watch)
	var notifications chan string = make(chan string)
	go notification(notifications)

	for {
		var configpaths []string = make([]string, 0)
		for i, p := range config.Paths {
			configpaths = append(configpaths, p.Path)
			if _, ok := channels[p.Path]; !ok {
				log.Infof("Adding channel '%s'", p.Path)
				channels[p.Path] = watch{
					stop:     make(chan bool, 1),
					complete: make(chan bool, 1),
					events:   make(chan notify.EventInfo),
				}
				go watchLocation(&config.Paths[i], channels[p.Path], config, notifications)
			}
		}

		for k := range channels {
			if !contains(k, configpaths) {
				log.Infof("Deleting channel '%s'", k)
				channels[k].stop <- true
				<-channels[k].complete
				delete(channels, k)
			}
		}
		var breakLoop bool = false
		select {
		case <-stop:
			breakLoop = true
			break
		default:
			<-time.After(1 * time.Second)
		}

		if breakLoop {
			break
		}
	}

	for k := range channels {
		channels[k].stop <- true
		<-channels[k].complete
	}
	finished <- true

}

func watchLocation(path *c.Path, channel watch, config *c.Config, notifications chan string) {
	var (
		active        []bool
		wg            sync.WaitGroup
		jobs          chan job    = make(chan job)
		activeWorkers []chan bool = make([]chan bool, 0)
		events        lockable    = lockable{
			paths: make(map[string]event),
		}
		notifyOnComplete bool = false
	)

	for i := 0; i < config.BufferSize; i++ {
		wg.Add(1)
		log.Debugf("Starting %s worker %d", path.Path, i)
		activeWorkers = append(activeWorkers, make(chan bool))
		go pathWorker(jobs, activeWorkers[len(activeWorkers)-1], &wg)
	}

	active = make([]bool, len(activeWorkers))

	if err := notify.Watch(path.Path, channel.events, notify.All); err != nil {
		log.Fatal(err)
		return
	}

	log.Info("Starting listening to: ", path.Path)

	var finished bool = false
	for {
		select {
		case ei := <-channel.events:
			switch ei.Event() {
			case notify.Remove:
				events.RLock()
				delete(events.paths, ei.Path())
				events.RUnlock()
			default:
				if _, err := os.Stat(ei.Path()); err != nil {
					continue
				}
				var details = m.Catagories.FindCatagoryFor(ei.Path())
				log.Debug(details)
				if details.Type != "application/x-partial-download" {
					events.RLock()
					events.paths[ei.Path()] = event{
						event:   ei.Event(),
						time:    time.Now(),
						details: *details,
					}
					events.RUnlock()
					notifyOnComplete = true
				}
			}
		case <-channel.stop:
			log.Infof("Shutting down listener for path %s", path.Path)
			for {
				// Wait for all jobs to finish
				if a, _ := allFinished(active, activeWorkers); a {
					log.Info("All jobs finished")
					close(jobs)
					break
				}
				<-time.After(1 * time.Millisecond)
			}
			log.Debug("Waiting for all goroutines to close")
			wg.Wait()
			log.Debug("All goroutines closed")
			notify.Stop(channel.events)
			channel.complete <- true

		case <-time.After(1 * time.Second):
			finished, active = allFinished(active, activeWorkers)

			go func() {
				events.Lock()
				for p, e := range events.paths {
					if e.time.Before(time.Now().Add(-config.DelayInSeconds * time.Second)) {
						delete(events.paths, p)
						log.Debugf("Creating job for path '%s'", p)
						jobs <- job{
							path:       p,
							details:    e.details,
							processors: path.Processors,
							czb:        config.CleanupZeroByte,
							ready:      true,
						}
					}
				}
				events.Unlock()

				if len(events.paths) == 0 && notifyOnComplete && finished {
					notifyOnComplete = false
					notifications <- fmt.Sprintf("Processing for path %s completed.", path.Path)
				}
			}()
		}
	}
}

func allFinished(active []bool, activeWorkers []chan bool) (bool, []bool) {
	for i := 0; i < len(activeWorkers); i++ {
		select {
		case a := <-activeWorkers[i]:
			active[i] = a
		default:
			continue
		}
	}

	for _, b := range active {
		if b {
			log.Debug("Found an active worker")
			return false, active
		}
	}
	return true, active
}

func pathWorker(jobs chan job, running chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		var j job = <-jobs
		if !j.ready {
			return
		}
		log.Infof("received job %s)", j.path)
		running <- true
		handlePath(j.path, j.details, j.processors, j.czb)
		running <- false
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
	// This will also allow wildcard for prrocessor.Type so anything not
	// handled can be handled by a fallback.
	if processor == nil {
		for i, p := range processors {
			if p.Type == details.Catagory || p.Type == "*" {
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
				log.Info("Done")
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
	setupPaths(config, stop, finished)
	<-done
}
