package config

import (
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

type dryRun struct {
	sync.RWMutex
	paths map[string]string
}

func (d *dryRun) contains(t, what string) bool {
	d.Lock()
	defer dryrun.Unlock()
	if v, ok := d.paths[what]; ok && (v == t) {
		return true
	}
	return false
}

func (d *dryRun) clear() {
	d.Lock()
	d.paths = make(map[string]string)
	d.Unlock()
}

func (d *dryRun) deletepath(path string) {
	d.RLock()
	delete(d.paths, path)
	d.RUnlock()
}

var dryrun *dryRun = &dryRun{
	paths: make(map[string]string),
}

var watchedPaths []string = make([]string, 0)

// DryRun Executes a dry run on the current configuration
//
// This function iterates all defined Paths for all defined Paths looking
// for Path elements which recurse each other.
// e.g. processor for path 1 writes into path 2 whose  processor writes
// back into path 1.
//
// This is not definitive, nor perfect as it cannot detect deeply nested recursion
// but should catch the majority of cases.
func DryRun(cnf *Config, handlePath handler) {
	paths := cnf.Paths
	for _, path := range paths {
		watchedPaths = append(watchedPaths, path.Path)
	}

	var i = 0
	for {
		var path = paths[0]
		var dritems = getDryRunItems()
		log.Infof("Starting dryrun %d (path %s)", i, path.Path)
		log.SetOutput(ioutil.Discard)
		for _, item := range dritems {
			var testPath string = GetDryRunPath(item.Type, path.Path, item.Extension)
			if testPath != "" {
				// Previous path should have been tested already and we're on a similar type
				dryrun.Lock()
				delete(dryrun.paths, item.Type)
				dryrun.Unlock()
			}
			testPath = filepath.Join(path.Path, dryRunFilename(10)+item.Extension)

			log.Infof("dry-run: Adding path %s for type %s", testPath, item.Type)
			AddDryRunPath(item.Type, testPath)

			var dt *m.Details = m.Catagories.FindCatagoryFor(testPath)
			if dt != nil {
				dt.DryRun = true
				// trigger this path so it adds the entry for testing against
				handlePath(testPath, *dt, path.Processors, false)
				runTests(notCurrent(path, paths), handlePath)
			}
		}
		log.SetOutput(os.Stderr)

		dryrun.clear()
		paths = append(paths[1:], paths[0])
		if i == len(paths)-1 {
			break
		}
		i++
	}
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

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

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

type handler func(path string, details m.Details, processors []Processor, czb bool) (err error)

func getDryRunItems() (d []*m.Details) {
	d = make([]*m.Details, 0)
	for _, items := range m.Catagories {
		for _, item := range items {
			dt := m.Catagories.FindCatagoryFor(item.Type)
			dt.DryRun = true
			var found bool = false
			for _, v := range d {
				if v.Type == dt.Type {
					found = true
				}
			}
			if !found {
				d = append(d, dt)
			}
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

func runTests(paths []Path, handlePath handler) {
	for _, test := range paths {
		for _, processor := range test.Processors {
			log.Infof("Checking path %s processor %s", test.Path, processor.Type)
			items := getDryRunPathsForType(test.Path, processor.Type)
			log.Infof("dry-run: found %d items for type %s", len(items), processor.Type)
			for _, item := range items {
				var dt *m.Details = m.Catagories.FindCatagoryFor(item)
				if dt != nil {
					dt.DryRun = true
					log.Infof("testing mime type %s on path %s", processor.Type, test.Path)
					handlePath(item, *dt, test.Processors, false)
					dryrun.deletepath(item)
				}
			}
		}
	}
}
