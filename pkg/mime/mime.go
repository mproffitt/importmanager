package mime

import (
	"os"
	"path"

	log "github.com/sirupsen/logrus"
)

// Catagories Set of all available mime types sorted by catagory
var Catagories catagories

// SplitPathByMime splits a path into component parts dir, basename, extension
func SplitPathByMime(filename string) (dirname, basename, extensions string) {
	dirname, basename = path.Split(filename)
	var tmp string = basename
	for {
		var d *Details
		if d = Catagories.FindCatagoryFor(tmp); d == nil {
			break
		}
		tmp = tmp[:len(tmp)-len(d.Extension)]
		extensions = d.Extension + extensions
	}
	basename = basename[:len(basename)-len(extensions)]
	return
}

// Load load all known mimetypes from the defined path or paths
func Load(paths []string) {
	Catagories = make(catagories)
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			log.Errorf("Unable to load path %s. %s", p, err.Error())
			continue
		}
		loadTypes(p)
	}
}
