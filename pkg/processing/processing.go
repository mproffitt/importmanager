package processing

import (
	"bytes"
	"html/template"
	"os"
	"os/user"
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
func Process(path string, details *mime.Details, processor *c.Processor) (err error) {
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
	dest, err = preProcess(path, processor.Path, details, processor)
	if err != nil {
		return
	}

	log.Infof("Checking processor type '%s'", processor.Handler)
	if c.IsBuiltIn(processor.Handler) {
		log.Info("Using builtin handler")
		if err = builtIn(path, dest, details, processor); err != nil {
			return
		}
	} else {
		log.Info("Using plugin handler")
		if err = runPlugin(path, dest, details, processor); err != nil {
			return
		}
	}

	err = postProcess(path, details, processor)
	return
}

func builtIn(source, dest string, details *mime.Details, processor *c.Processor) (err error) {
	switch processor.Handler {
	case "copy":
		_, err = pcopy(source, dest, details, processor)
	case "move":
		_, err = pmove(source, dest, details, processor)
	case "extract":
		err = pextract(source, dest, details, processor)
	case "install":
		err = pinstall(source, dest, details, processor)
	case "delete":
		err = pdelete(source)
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
			if b, _ := strconv.ParseBool(value); !b {
				continue
			}
			fi, _ := os.Stat(path)

			// Default to STAT Modification time
			var date string = fi.ModTime().Format("2006-01-02")

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

	if err = os.MkdirAll(dest, 0750); err != nil {
		return "", err
	}

	return dest, nil
}

func postProcess(path string, details *mime.Details, processor *c.Processor) (err error) {
	for k, v := range processor.Properties {
		switch strings.ToLower(k) {
		case "chown":
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
			if err = os.Chown(path, uid, gid); err != nil {
				return
			}
		case "chmod":
			var set m.Set
			if set, err = m.Parse(v); err != nil {
				return
			}
			if _, _, err = set.Chmod(path); err != nil {
				return
			}
		case "setexec":
			if b, _ := strconv.ParseBool(v); !b {
				// We reuse `setexec` property when coming from
				// `install` - probably other places will too.
				if _, err := os.Stat(v); !os.IsNotExist(err) {
					continue
				}
				path = v
			}

			var set m.Set
			if set, err = m.Parse("+x"); err != nil {
				return
			}
			if _, _, err = set.Chmod(path); err != nil {
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
