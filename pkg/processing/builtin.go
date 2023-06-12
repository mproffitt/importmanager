package processing

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	a "github.com/codeclysm/extract/v3"
	c "github.com/mproffitt/importmanager/pkg/config"
	"github.com/mproffitt/importmanager/pkg/mime"
	log "github.com/sirupsen/logrus"
)

func pcopy(source, dest string, details *mime.Details, processor *c.Processor) (final string, err error) {
	var basename string = path.Base(source)
	if b, _ := strconv.ParseBool(processor.Properties["strip-extension"]); b {
		basename = strings.TrimSuffix(basename, details.Extension)

		if b, _ := strconv.ParseBool(processor.Properties["lowercase-destination"]); b {
			basename = strings.ToLower(basename)
		}
	}

	final = filepath.Join(dest, basename)
	if _, err = os.Stat(final); err == nil {
		log.Warnf("File already exists at '%s'. Removing source", final)
		if _, err = pdelete(source); err != nil {
			return
		}
		err = fmt.Errorf("copy-deleted")
		return
	}

	var (
		r *os.File
		w *os.File
	)
	if r, err = os.Open(source); err != nil {
		return
	}
	defer r.Close()

	if w, err = os.Create(final); err != nil {
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

func pmove(source, dest string, details *mime.Details, processor *c.Processor) (final string, err error) {
	if final, err = pcopy(source, dest, details, processor); err != nil {
		if err.Error() == "copy-deleted" {
			err = nil
		}
		return
	}
	_, err = pdelete(source)
	return
}

func pextract(source, dest string, details *mime.Details, processor *c.Processor) (final string, err error) {
	var (
		file     *os.File
		basename string = path.Base(source)
	)
	log.Debugf("Stripping extension '%s'", details.Extension)
	basename = strings.TrimSuffix(basename, details.Extension)
	if strings.HasSuffix(basename, ".tar") {
		basename = strings.TrimSuffix(basename, ".tar")
	}
	final = filepath.Join(dest, basename)

	if file, err = os.Open(source); err != nil {
		return
	}
	defer file.Close()

	log.Infof("Extracting '%s' to '%s'", source, final)
	if err = a.Archive(context.TODO(), file, final, nil); err != nil {
		return
	}

	for k, v := range processor.Properties {
		switch k {
		case "cleanup-source":
			if b, _ := strconv.ParseBool(v); b {
				if _, err = pdelete(source); err != nil {
					return
				}
			}
		}
	}
	return
}

func pinstall(source, dest string, details *mime.Details, processor *c.Processor) (final string, err error) {
	// To protect the overall system, we only "install" AppImages and scripts which are
	// "installed" by moving them to ~/bin and setting the executable flag
	if final, err = pmove(source, dest, details, processor); err == nil {
		// this is handled by the post processor
		(*processor).Properties["setexec"] = final
	}
	return
}

func pdelete(source string) (final string, err error) {
	log.Infof("Deleting path '%s'.", source)
	err = os.Remove(source)
	return
}
