package config

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	m "github.com/mproffitt/importmanager/pkg/mime"
	log "github.com/sirupsen/logrus"
)

var (
	dryrun *dryRun = &dryRun{
		paths: make(map[string]string),
	}

	seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))

	watchedPaths []string = make([]string, 0)
)

func (d *dryRun) contains(t, what string) bool {
	d.Lock()
	defer dryrun.Unlock()
	if v, ok := d.paths[what]; ok && (v == t) {
		return true
	}
	return false
}

func (d *dryRun) clear() {
	d.RLock()
	d.paths = make(map[string]string)
	d.RUnlock()
}

func (d *dryRun) deletepath(path string) {
	d.RLock()
	delete(d.paths, path)
	d.RUnlock()
}

// DryRun Executes a dry run on the current configuration
//
// This function iterates all defined Paths for all defined Paths looking
// for Path elements which recurse each other.
// e.g. processor for path 1 writes into path 2 whose  processor writes
// back into path 1.
//
// This is not definitive, nor perfect as it cannot detect deeply nested recursion
// but should catch the majority of cases.
func DryRun(cnf *Config) {
	paths := cnf.Paths
	for _, path := range paths {
		watchedPaths = append(watchedPaths, path.Path)
	}
	log.Infof("Preparing dry run for %d paths", len(watchedPaths))
	log.SetOutput(ioutil.Discard)

	var (
		completed int       = len(cnf.Paths)
		done      chan bool = make(chan bool)
	)
	for i, path := range cnf.Paths {
		go executeDryRun(i, path, cnf, done)
	}
	for completed > 0 {
		select {
		case <-done:
			completed--
		}
	}
}

var loggingLock sync.Mutex

func dryrunLog(msg string, args ...interface{}) {
	// To prevent race conditions between threads when changing log output
	// lock temporarily
	loggingLock.Lock()
	defer loggingLock.Unlock()

	log.SetOutput(log.StandardLogger().Out)
	log.Infof(msg, args...)
	log.SetOutput(ioutil.Discard)
}

func executeDryRun(index int, path Path, cnf *Config, done chan bool) {
	var dritems = getDryRunItems()
	dryrunLog("Starting dryrun %d (path %s)", index, path.Path)

	for _, item := range dritems {
		var testPath string = GetDryRunPath(item.Type, path.Path, item.Extension)
		if testPath != "" {
			// Previous path should have been tested already and we're on a similar type
			dryrun.Lock()
			delete(dryrun.paths, item.Type)
			dryrun.Unlock()
		}
		testPath = filepath.Join(path.Path, dryRunFilename(10)+item.Extension)

		fmt.Println("hello world")
		dryrunLog("dry-run: Adding path %s for type %s", testPath, item.Type)
		AddDryRunPath(item.Type, testPath)

		if dt := m.Catagories.FindBestMatchFor(testPath); dt != nil {
			dt.DryRun = true
			// trigger this path so it adds the entry for testing against
			cnf.pathHandler(testPath, *dt, path.Processors, false)
			runTests(notCurrent(path, cnf.Paths), cnf.pathHandler)
		}
	}

	dryrun.clear()
	done <- true
}

// AddDryRunPath Adds a path to the dry-run set. Fatal error if path already exists
func AddDryRunPath(t, path string) (err error) {
	if dryrun.contains(t, path) {
		log.SetOutput(os.Stderr)
		log.Fatalf("recursive configuration detected on path %s", path)
		return
	}

	var watched bool = false
	for _, p := range watchedPaths {
		if strings.HasPrefix(path, p) {
			watched = true
			break
		}
	}
	if !watched {
		return
	}
	dryrun.RLock()
	dryrun.paths[path] = t
	dryrun.RUnlock()
	return
}

// GetDryRunPath Gets a path to test within a given handler
func GetDryRunPath(t, prefix, ext string) string {
	dryrun.Lock()
	defer dryrun.Unlock()
	for path, tpe := range dryrun.paths {
		if tpe == t && (strings.HasPrefix(path, prefix) && strings.HasSuffix(path, ext)) {
			return path
		}
	}
	return ""
}

// DeleteDryRunPath Delete a path from the dryrun set
func DeleteDryRunPath(path string) {
	dryrun.deletepath(path)
}

func dryRunFilename(length int) string {
	var (
		charset string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		b       []byte = make([]byte, length)
	)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func getDryRunItems() (d []*m.Details) {
	d = make([]*m.Details, 0)
	for k, items := range m.Catagories {
		for _, item := range items {
			dt := m.Details{
				Catagory: k,
				SubClass: make([]string, 0),
				Type:     item.Type,
				DryRun:   true,
			}
			if len(item.Globs) > 0 {
				dt.Extension = item.Globs[0].Pattern[1:]
			}
			for _, sc := range item.SubClass {
				dt.SubClass = append(dt.SubClass, sc.Type)
			}

			d = append(d, &dt)
		}
	}
	return
}

func getDryRunPathsForType(testpath, t string) (p []string) {
	p = make([]string, 0)
	dryrun.Lock()
	defer dryrun.Unlock()
	for path, tpe := range dryrun.paths {
		if strings.HasPrefix(path, testpath) {
			if tpe == t || strings.HasPrefix(tpe, t) || t == "*" {
				p = append(p, path)
			}
		}
	}
	return
}

func notCurrent(path Path, paths []Path) (p []Path) {
	p = make([]Path, 0)
	for _, pth := range paths {
		if pth.Path == path.Path {
			continue
		}
		p = append(p, pth)
	}
	return
}

func runTests(paths []Path, pathHandler handler) {
	for _, test := range paths {
		for _, processor := range test.Processors {
			log.Infof("Checking path %s processor %s", test.Path, processor.Type)
			items := getDryRunPathsForType(test.Path, processor.Type)
			log.Infof("dry-run: found %d items for type %s", len(items), processor.Type)
			for _, item := range items {
				var dt *m.Details = m.Catagories.FindBestMatchFor(item)
				if dt != nil {
					dt.DryRun = true
					log.Infof("testing mime type %s on path %s", processor.Type, test.Path)
					pathHandler(item, *dt, test.Processors, false)
					dryrun.deletepath(item)
				}
			}
		}
	}
}
