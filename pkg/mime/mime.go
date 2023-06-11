package mime

import (
	"encoding/xml"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	log "github.com/sirupsen/logrus"
)

// Glob XML entry for the glob entry to Type
type Glob struct {
	XMLName xml.Name `xml:"glob"`
	Pattern string   `xml:"pattern,attr"`
}

// SubType XML entry for the sub-class-of entry to Type
type SubType struct {
	XMLName xml.Name `xml:"sub-class-of"`
	Type    string   `xml:"type,attr"`
}

// Alias XML container for the alias entry to Type
type Alias struct {
	XMLName xml.Name `xml:"alias"`
	Type    string   `xml:"type,attr"`
}

// Type XML container type for a  type
type Type struct {
	XMLName  xml.Name  `xml:"mime-type"`
	Xmlns    string    `xml:"xmlns,attr"`
	Type     string    `xml:"type,attr"`
	Globs    []Glob    `xml:"glob"`
	Aliases  []Alias   `xml:"alias"`
	SubClass []SubType `xml:"sub-class-of"`
}

// GlobMatches Test if the file extension matches one of the globs defined for this type
func (m Type) GlobMatches(what string) bool {
	for _, v := range m.Globs {
		if strings.EqualFold(path.Ext(what), strings.Replace(v.Pattern, "*", "", 1)) {
			return true
		}
	}
	return false
}

// AliasMatches Test if the file extension matches one of the globs defined for this type
func (m Type) AliasMatches(what string) bool {
	for _, v := range m.Aliases {
		if strings.EqualFold(what, v.Type) {
			return true
		}
	}
	return false
}

type catagories map[string][]Type

// Details Contains basic information about the type
type Details struct {
	Catagory  string   `json:"category"`
	Type      string   `json:"type"`
	SubClass  []string `json:"subclass"`
	Extension string   `json:"extension"`
}

// IsExecutable - Test if the current mime version should be executable
func (m *Details) IsExecutable() bool {
	for _, sc := range m.SubClass {
		if strings.EqualFold(sc, "application/x-executable") {
			return true
		}
	}
	return false
}

// IsSubClassOf Test if the current item is a subclass of the type
func (m *Details) IsSubClassOf(class string) bool {
	for _, sc := range m.SubClass {
		if strings.EqualFold(class, sc) {
			return true
		}
	}
	return false
}

// IsSubClass Is this mime type a subclass type
func (m *Details) IsSubClass() bool {
	return len(m.SubClass) > 0
}

func (c catagories) FindCatagoryFor(what string) (details *Details) {
	details = &Details{}
	details.SubClass = make([]string, 0)
	for k, v := range c {
		details.Catagory = k
		for _, item := range v {
			var matched bool = false
			details.Type = item.Type
			if strings.EqualFold(item.Type, what) || item.AliasMatches(what) {
				details.Extension = strings.Replace(item.Globs[0].Pattern, "*", "", 1)
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

	// If we haven't got a match, check magic
	if _, err := os.Stat(what); err == nil {
		if mtype, err := mimetype.DetectFile(what); err == nil {
			return c.FindCatagoryFor(mtype.String())
		}
	}
	return nil
}

// Catagories Set of all available mime types sorted by catagory
var Catagories catagories

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
				mimes = append(mimes, mime)
			}
		}
		return nil
	})
	return
}

func loadTypes(path string) (mimes map[string]string, err error) {
	mimes = make(map[string]string)
	files, _ := ioutil.ReadDir(path)
	for _, fi := range files {
		if fi.IsDir() {
			var c []Type
			if c, err = loadCategory(filepath.Join(path, fi.Name())); err != nil {
				return
			}
			if _, ok := Catagories[fi.Name()]; !ok {
				Catagories[fi.Name()] = make([]Type, 0)
			}
			Catagories[fi.Name()] = append(Catagories[fi.Name()], c...)
		}
	}
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

//func init() {
//	loadTypes("/usr/share/mime")
//}
