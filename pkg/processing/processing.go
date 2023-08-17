package processing

import (
	"bytes"
	"html/template"
	"io/fs"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	exif "github.com/barasher/go-exiftool"
	c "github.com/mproffitt/importmanager/pkg/config"
	"github.com/mproffitt/importmanager/pkg/mime"
	log "github.com/sirupsen/logrus"
	m "hg.sr.ht/~dchapes/mode"
)

// Process start the processing for the given path
func Process(source string, details *mime.Details, processor *c.Processor) (err error) {
	log.Infof("Parsing path properties for '%s'", processor.Type)
	if processor.Properties == nil {
		(*processor).Properties = make(map[string]string)
	}

	if strings.Contains(processor.Path, "{{.date}}") {
		processor.Properties["include-date-directory"] = "true"
	}

	if strings.Contains(processor.Path, "{{.ext}}") {
		processor.Properties["extension-directory"] = "true"
	}

	if strings.Contains(processor.Path, "{{.ucext}}") {
		processor.Properties["uppercase-extension-directory"] = "true"
	}

	var dest string
	if dest, err = preProcess(source, processor.Path, details, processor); err != nil {
		return
	}

	// This will form part of the dry-run logic for validating handlers.
	//
	// At present I see no viable reason to action beyond this point
	// as the validation is about testing handlers do not `ping-pong`
	// files between directories
	if details.DryRun {
		var path string = path.Join(dest, filepath.Base(source))
		log.Warnf(
			"Processing: Adding dry run path %s for type %s (processor %s %s)",
			path, details.Type, processor.Type, processor.Handler)
		c.AddDryRunPath(details.Type, path)
		return
	}

	var final string
	log.Infof("Checking processor type '%s'", processor.Handler)
	if c.DefaultHandlers.IsBuiltIn(processor.Handler) {
		log.Info("Using builtin handler")
		if final, err = builtIn(source, dest, details, processor); err != nil {
			return
		}
	} else {
		log.Info("Using plugin handler")
		if final, err = runPlugin(source, dest, details, processor); err != nil {
			return
		}
	}

	if final != "" {
		err = postProcess(final, details, processor)
	}
	return
}

func builtIn(source, dest string, details *mime.Details, processor *c.Processor) (final string, err error) {
	switch processor.Handler {
	case "copy":
		final, err = pcopy(source, dest, details, processor)
	case "move":
		final, err = pmove(source, dest, details, processor)
	case "extract":
		final, err = pextract(source, dest, details, processor)
	case "install":
		final, err = pinstall(source, dest, details, processor)
	case "delete":
		final, err = pdelete(source)
	}
	return
}

type properties map[string]interface{}

func preProcess(path, dest string, details *mime.Details, processor *c.Processor) (string, error) {
	log.Infof("Triggering preProcessing for '%s'", processor.Type)
	var p properties = properties{
		"ext": strings.Replace(details.Extension, ".", "", 1),
	}
	for key, value := range processor.Properties {
		switch strings.ToLower(key) {
		case "uppercase-extension-directory":
			if b, _ := strconv.ParseBool(value); !b {
				continue
			}
			var ext string = strings.ToUpper(details.Extension)
			p["ucext"] = strings.Replace(ext, ".", "", 1)

		case "include-date-directory":
			if b, _ := strconv.ParseBool(value); !b || details.DryRun {
				continue
			}

			var date string = "2023:09:12 23:34:00+00:00"
			fi, _ := os.Stat(path)
			// Default to STAT Modification time
			date = fi.ModTime().Format("2006-01-02")

			// If this is an image, try and use the ExifData
			if details.Catagory == "image" {
				if info, err := exifData(path); err == nil {
					var d string
					// Default images to CreateDate
					if v, ok := info["CreateDate"]; ok {
						d = v.(string)
					}

					// If we have exif-date in properties, we can try that instead
					if v, ok := processor.Properties["exif-date"]; ok {
						d = info[v].(string)
					}

					// dont change date until we're sure we have something valid
					if t, err := time.Parse("2006:01:02 15:04:05-07:00", d); err == nil {
						date = t.Format("2006-01-02")
					}
				}
			}
			p["date"] = date
		}
	}

	var err error
	if dest, err = formatT(dest, p); err != nil {
		return "", err
	}

	if !details.DryRun {
		if err = os.MkdirAll(dest, 0750); err != nil {
			return "", err
		}
	}

	return dest, nil
}

func isDir(path string) bool {
	if fi, err := os.Stat(path); err == nil {
		return fi.IsDir()
	}
	return false
}

func pathInBinDir(path string) bool {
	return strings.EqualFold(filepath.Base(filepath.Dir(path)), "bin")
}

func postProcess(dest string, details *mime.Details, processor *c.Processor) (err error) {
	log.Infof("Triggering postProcessor for %s", dest)
	for k, v := range processor.Properties {
		switch strings.ToLower(k) {
		case "chown":
			log.Infof("checking chown %s %s", v, dest)
			who := strings.Split(v, ":")
			var (
				u        *user.User
				g        *user.Group
				uid, gid int
			)
			if u, err = user.Lookup(who[0]); err != nil {
				return
			}
			uid, _ = strconv.Atoi(u.Uid)
			if g, err = user.LookupGroup(who[1]); err != nil {
				return
			}
			gid, _ = strconv.Atoi(g.Gid)
			if isDir(dest) {
				err = filepath.WalkDir(dest, func(path string, d fs.DirEntry, e error) (err error) {
					if e != nil {
						log.Errorf("Unable to walk directory for chown %s - %s", dest, e.Error())
						return
					}
					err = os.Chown(path, uid, gid)
					return
				})
				break
			}
			if err = os.Chown(dest, uid, gid); err != nil {
				return
			}
		case "chmod":
			log.Infof("checking chmod %s %s", v, dest)
			var set m.Set
			if set, err = m.Parse(v); err != nil {
				return
			}
			if isDir(dest) {
				err = filepath.WalkDir(dest, func(path string, d fs.DirEntry, e error) (err error) {
					if e != nil {
						log.Errorf("Unable to walk directory %s for chmod - %s", dest, e.Error())
						return
					}
					_, _, err = set.Chmod(path)
					// Set the executable bit for directories and
					// any file found in a `bin´ folder
					if isDir(path) || pathInBinDir(path) {
						var s m.Set
						if s, err = m.Parse("+x"); err != nil {
							return
						}
						_, _, err = s.Chmod(path)
					}
					return
				})
				break
			}
			if _, _, err = set.Chmod(dest); err != nil {
				return
			}
		case "setexec":
			log.Infof("checking setexec %s", v)
			if b, _ := strconv.ParseBool(v); !b {
				// We reuse `setexec` property when coming from
				// `install` - probably other places will too.
				if _, err := os.Stat(v); err != nil {
					continue
				}
				dest = v
			}

			var set m.Set
			if set, err = m.Parse("+x"); err != nil {
				return
			}
			if _, _, err = set.Chmod(dest); err != nil {
				return
			}
		}
	}
	return
}

func formatT(format string, args properties) (formatted string, err error) {
	log.Debugf("Templating '%s' with %+v", format, args)
	t := template.Must(template.New("").Parse(format))
	var doc bytes.Buffer
	if err = t.Execute(&doc, args); err == nil {
		formatted = doc.String()
	}
	return
}

func exifData(path string) (map[string]interface{}, error) {
	et, err := exif.NewExiftool()
	if err != nil {
		return nil, err
	}
	defer et.Close()

	fi := et.ExtractMetadata(path)[0]
	if fi.Err != nil {
		return nil, fi.Err
	}

	return fi.Fields, nil
}
