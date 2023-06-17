package mime

import (
	"encoding/xml"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

func (c catagories) FindCatagoryFor(what string) (details *Details) {
	details = &Details{}
	details.SubClass = make([]string, 0)
	for k, v := range c {
		details.Catagory = k
		for _, item := range v {
			var matched bool = false
			details.Type = item.Type
			if strings.EqualFold(item.Type, what) || item.AliasMatches(what) {
				if len(item.Globs) > 0 {
					details.Extension = strings.Replace(item.Globs[0].Pattern, "*", "", 1)
				}
				matched = true
			}

			if item.GlobMatches(what) {
				details.Extension = path.Ext(what)
				matched = true
			}
			if matched {
				for _, sc := range item.SubClass {
					details.SubClass = append(details.SubClass, sc.Type)
				}
				return
			}

		}
	}

	if details.Catagory == what {
		return
	}

	// If we haven't got a match, check magic
	if _, err := os.Stat(what); err == nil {
		if mtype, err := mimetype.DetectFile(what); err == nil {
			return c.FindCatagoryFor(mtype.String())
		}
	}
	return nil
}

func loadCategory(dir string) (mimes []Type, err error) {
	mimes = make([]Type, 0)

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			var mime Type
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			// ignore errors and just load what we can
			if err := xml.NewDecoder(file).Decode(&mime); err == nil {
				var found bool = false
				for _, v := range mimes {
					if v.Type == mime.Type {
						found = true
						break
					}
				}
				if !found {
					mimes = append(mimes, mime)
				}
			}
		}
		return nil
	})
	return
}
