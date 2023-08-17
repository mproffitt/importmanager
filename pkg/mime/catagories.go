package mime

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

func (c *catagories) FindBestMatchFor(what string) (details *Details) {
	var (
		lastlen int = 0
		d       []Details
	)

	if d = c.FindAllMatchesFor(what); d != nil || len(d) > 0 {
		for i, test := range d {
			if len(test.Extension) > lastlen {
				lastlen = len(test.Extension)
				details = &d[i]
			}
		}
	}
	return
}

// FindAllMatchesFor Find all catagories that match the given filename
//
// Arguments:
// - what string The filename or mime type to test
//
// Return
//
// - []Details a list of mime.Details about each matched mime type
//
func (c *catagories) FindAllMatchesFor(what string) (details []Details) {
	details = make([]Details, 0)
	for k, v := range *c {
		for _, item := range v {
			var matched bool = false
			var d Details = Details{
				SubClass: make([]string, 0),
				Catagory: k,
				Type:     item.Type,
			}
			if strings.EqualFold(item.Type, what) || item.AliasMatches(what) {
				matched = true
				if len(item.Globs) > 0 {
					d.Extension = strings.Replace(item.Globs[0].Pattern, "*", "", 1)
				}
			} else if matches, match := item.GlobMatches("." + what); matches {
				matched = true
				d.Extension = strings.Replace(match, "*", "", 1)
			}
			if matched {
				for _, sc := range item.SubClass {
					d.SubClass = append(d.SubClass, sc.Type)
				}
				details = append(details, d)
			}
		}
	}

	if len(details) > 0 {
		return
	}

	// If we haven't got a match, check magic
	if _, err := os.Stat(what); err == nil {
		if mtype, err := mimetype.DetectFile(what); err == nil {
			return c.FindAllMatchesFor(mtype.String())
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
