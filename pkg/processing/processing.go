package processing

import (
	"fmt"
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
	m "hg.sr.ht/~dchapes/mode"
)

// Process start the processing for the given path
func Process(path string, details *mime.Details, processor *c.Processor) (err error) {
	var dest string
	dest, err = preProcess(path, processor.Path, details, processor)
	if err != nil {
		return
	}

	if isBuiltIn(processor.Type) {
		if err = builtIn(path, dest, details, processor); err != nil {
			return
		}
	}

	err = postProcess(path, details, processor)
	return
}

func isBuiltIn(pt string) bool {
	for _, v := range c.DefaultHandlers {
		if strings.EqualFold(pt, v) {
			return true
		}
	}
	return false
}

func builtIn(source, dest string, details *mime.Details, processor *c.Processor) (err error) {
	dest = path.Join(dest, path.Base(source))
	switch processor.Type {
	case "copy":
		pcopy(source, dest, details, processor)
	case "move":
		pmove(source, dest, details, processor)
	case "extract":
		pextract(source, dest, details, processor)
	case "install":
		pinstall(source, dest, details, processor)
	case "delete":
		pdelete(source)
	}
	return
}

func preProcess(path, dest string, details *mime.Details, processor *c.Processor) (string, error) {
	var fmtArgs []string = make([]string, 0)
	for key, value := range processor.Properties {
		switch strings.ToLower(key) {
		case "uppercase-extension":
			if b, _ := strconv.ParseBool(value); !b {
				continue
			}
			var ext string = strings.ToUpper(details.Extension)
			fmtArgs = append(fmtArgs, []string{"ext", strings.Replace(ext, ".", "", 1)}...)
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
			fmtArgs = append(fmtArgs, []string{"date", date}...)
		}
	}

	if strings.HasPrefix(path, "~/") {
		var (
			usr, _ = user.Current()
			dir    = usr.HomeDir
		)
		dest = filepath.Join(dir, dest[2:])
	}
	// create the destination directory
	dest = format(dest, fmtArgs...)
	if err := os.MkdirAll(dest, 0750); err != nil {
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
				continue
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

func format(format string, args ...string) string {
	r := strings.NewReplacer(args...)
	return fmt.Sprint(r.Replace(format))
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
