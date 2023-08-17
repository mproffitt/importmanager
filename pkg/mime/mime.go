package mime

import (
	"os"
	"path"

	log "github.com/sirupsen/logrus"
)

// Catagories Set of all available mime types sorted by catagory
var Catagories catagories

// SplitPathByMime splits a path into component parts dir, basename, extension
func SplitPathByMime(filename string) (dirname, basename, extension string) {
	dirname, basename = path.Split(filename)
	if d := Catagories.FindBestMatchFor(filename); d != nil {
		extension = d.Extension
	}
	var fnlen int = len(basename) - len(extension)
	extension = basename[fnlen:]
	basename = basename[:fnlen]
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
	log.Info("Finished loading catagories")
}
