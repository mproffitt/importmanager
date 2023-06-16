package processing

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	a "github.com/codeclysm/extract/v3"
	c "github.com/mproffitt/importmanager/pkg/config"
	m "github.com/mproffitt/importmanager/pkg/mime"
	log "github.com/sirupsen/logrus"
)

func pcopy(source, dest string, details *m.Details, processor *c.Processor) (final string, err error) {
	var _, basename, extension = m.SplitPathByMime(source)
	final = dest
	if b, _ := strconv.ParseBool(processor.Properties["strip-extension"]); !b {
		basename = basename + extension
	}

	if b, _ := strconv.ParseBool(processor.Properties["lowercase-destination"]); b {
		basename = strings.ToLower(basename)
	}

	// If destination looks like a filename, we keep that.
	if !strings.EqualFold(path.Ext(dest), extension) {
		final = filepath.Join(dest, basename)
	}

	log.Infof("Copy: Testing final %s", final)
	if _, err = os.Stat(final); err == nil {
		if b, _ := strconv.ParseBool(processor.Properties["compare-sha"]); b {
			final, err = copyWithShaCheck(source, final, details, processor)
			return
		}
		log.Warnf("File already exists at '%s'. Removing source", final)
		if _, err = pdelete(source); err != nil {
			return
		}
		err = fmt.Errorf("copy-deleted")
		return
	}

	log.Info("Using standard copy (not comparing sha)")
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

func pmove(source, dest string, details *m.Details, processor *c.Processor) (final string, err error) {
	log.Infof("triggering move for path %s", source)
	if final, err = pcopy(source, dest, details, processor); err != nil {
		if err.Error() == "copy-deleted" {
			err = nil
		}
		return
	}
	_, err = pdelete(source)
	return
}

func pextract(source, dest string, details *m.Details, processor *c.Processor) (final string, err error) {
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

func pinstall(source, dest string, details *m.Details, processor *c.Processor) (final string, err error) {
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

// This is a much slower operation so should be used sparingly
//
// If sha256 matches on both files, delete the source
// If Sha256 does not match, add 1 to the filename and retest until we find a unique filename
func copyWithShaCheck(source, dest string, details *m.Details, processor *c.Processor) (final string, err error) {
	log.Infof("Using `copyWithShaCheck` for %s", source)
	if final, err = cleanupIfEqual(source, dest); err != nil {
		return
	}
	var (
		i                            int = 1
		filename                     string
		dirname, basename, extension = m.SplitPathByMime(dest)
	)

	for {
		filename = fmt.Sprintf("%s%s_%d%s", dirname, basename, i, extension)
		log.Infof("Using filename %s for sha test", filename)
		if _, err = os.Stat(filename); err == nil {
			if final, err = cleanupIfEqual(source, filename); err != nil {
				return
			}
			i++
			continue
		}
		break
	}
	(*processor).Properties["compare-sha"] = "false"
	final, err = pcopy(source, filename, details, processor)
	return
}

func cleanupIfEqual(source, dest string) (string, error) {
	if sha256Equal(source, dest) {
		pdelete(source)
		return dest, fmt.Errorf("sha256-match %s - source deleted", source)
	}
	return dest, nil
}

func sha256Equal(source, dest string) bool {
	log.Infof("Comparing sha256 between %s and %s", source, dest)
	var a, b string = getSha256(source), getSha256(dest)
	return a == b
}

func getSha256(filename string) string {
	f, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}
