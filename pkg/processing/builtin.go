package processing

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	a "github.com/codeclysm/extract/v3"
	c "github.com/mproffitt/importmanager/pkg/config"
	"github.com/mproffitt/importmanager/pkg/mime"
	log "github.com/sirupsen/logrus"
)

func pcopy(source, dest string, details *mime.Details, processor *c.Processor) (err error) {
	var (
		r *os.File
		w *os.File
	)
	if r, err = os.Open(source); err != nil {
		return
	}
	defer r.Close() // ok to ignore error: file was opened read-only.

	if w, err = os.Create(dest); err != nil {
		return
	}

	defer func() {
		if err = w.Close(); err != nil {
			return
		}
	}()

	_, err = io.Copy(w, r)
	return
}

func pmove(source, dest string, details *mime.Details, processor *c.Processor) (err error) {
	err = os.Rename(source, dest)
	return
}

func pextract(source, dest string, details *mime.Details, processor *c.Processor) (err error) {
	var (
		file     *os.File
		basename string = path.Base(source)
		untar    bool   = false
	)
	basename = strings.TrimSuffix(basename, details.Extension)
	if strings.HasSuffix(basename, ".tar") {
		basename = strings.TrimSuffix(basename, ".tar")
		untar = true
	}

	if untar {
		if file, err = os.Open(source); err != nil {
			return
		}
		defer file.Close()
		var tmpdest string = filepath.Join("/tmp", path.Base(dest)+".tar")
		if err = a.Archive(context.TODO(), file, tmpdest, nil); err != nil {
			return
		}
		source = tmpdest
	}

	if err = file.Close(); err != nil {
		return
	}

	if file, err = os.Open(source); err != nil {
		return
	}
	defer file.Close()

	err = a.Archive(context.TODO(), file, dest, nil)
	return
}

func pinstall(source, dest string, details *mime.Details, processor *c.Processor) {}

func pdelete(source string) {
	log.Infof("Deleting path '%s'.", source)
	os.Remove(source)
}
