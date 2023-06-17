package handler

import (
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"

	c "github.com/mproffitt/importmanager/pkg/config"
	m "github.com/mproffitt/importmanager/pkg/mime"
	p "github.com/mproffitt/importmanager/pkg/processing"
	"github.com/rjeczalik/notify"
	log "github.com/sirupsen/logrus"
)

func contains(what string, where []string) bool {
	for _, p := range where {
		if what == p {
			return true
		}
	}
	return false
}

// Setup Sets up watches for each path in config
//
// Arguments:
//
// - config        *config.Config  The full configuration object
// - stop          chan bool        Write into this channel to instruct the watchers to shut down
// - finished      chan bool        Read from this channel to know when all watchers have completed
// - notifications chan string      A channel to write notifications back into
//
// Return:
//
// - void
func Setup(config *c.Config, stop, finished chan bool, notifications chan string) {
	channels := make(map[string]watch)
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

// Handle Finds the processor for a given filepath and triggers it
//
// Arguments:
//
// - path:       string             The path to a file to process
// - details:    mime.Details       Mime information about the given file
// - processors: []config.Processor A list of processors for the files base path
// - czb:        bool               Clear zero byte files If true will automatically delete empty files
//
// Return:
//
// - error The past known error
func Handle(path string, details m.Details, processors []c.Processor, czb bool) (err error) {
	log.Infof("Handling path %s", path)
	var (
		dryrun bool = details.DryRun
		fi     fs.FileInfo
	)
	if !dryrun {
		if fi, err = os.Stat(path); err == nil {
			if fi.Size() == 0 && czb {
				log.Infof("Deleting path '%s'. File is empty", path)
				os.Remove(path)
				return
			}
		}
	}

	var processor *c.Processor

	// try find an exact processor for this mimetype
	for i, p := range processors {
		if p.Type == details.Type && !p.Negated {
			processor = &processors[i]
			break
		}
	}

	// If we don't have an exact match and this is a subclass, try that
	if processor == nil && details.IsSubClass() {
		for i, p := range processors {
			if details.IsSubClassOf(p.Type) && !p.Negated {
				processor = &processors[i]
				break
			}
		}
	}

	// If we still don't have a processor fall back to catagory level
	// This will also allow wildcard for processor.Type so anything not
	// handled can be handled by a fallback.
	if processor == nil {
		for i, p := range processors {
			if (p.Type == details.Catagory || p.Type == "*") && !p.Negated {
				processor = &processors[i]
				break
			}
		}
		if processor == nil {
			log.Errorf("No processor defined for type '%s | %s | %s'", details.Type, details.SubClass, details.Catagory)
			if details.DryRun {
				c.DeleteDryRunPath(path)
			}
			return
		}
	}

	log.Infof("Found processor '%s' for path %s", processor.String(), path)
	if err := p.Process(path, &details, processor); err != nil {
		log.Errorf("Unable to process path %s - %s", path, err.Error())
	}
	log.Infof("Completed parsing for %s", path)
	return
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
		log.Fatalf("Failed to set up watch for path %s - %s", path.Path, err.Error())
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
				var details = m.Catagories.FindCatagoryFor(ei.Path())
				if _, err := os.Stat(ei.Path()); err != nil || details == nil {
					continue
				}
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
		Handle(j.path, j.details, j.processors, j.czb)
		running <- false
	}
}
