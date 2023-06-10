package mime

import (
	"encoding/xml"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

// MimeGlob XML entry for the glob entry to MimeType
type MimeGlob struct {
	XMLName xml.Name `xml:"glob"`
	Pattern string   `xml:"pattern,attr"`
}

// MimeSubType XML entry for the sub-class-of entry to MimeType
type MimeSubType struct {
	XMLName xml.Name `xml:"sub-class-of"`
	Type    string   `xml:"type,attr"`
}

// MimeAlias XML container for the alias entry to MimeType
type MimeAlias struct {
	XMLName xml.Name `xml:"alias"`
	Type    string   `xml:"type,attr"`
}

// MimeType XML container type for a Mime type
type MimeType struct {
	XMLName  xml.Name      `xml:"mime-type"`
	Xmlns    string        `xml:"xmlns,attr"`
	Type     string        `xml:"type,attr"`
	Globs    []MimeGlob    `xml:"glob"`
	Aliases  []MimeAlias   `xml:"alias"`
	SubClass []MimeSubType `xml:"sub-class-of"`
}

// GlobMatches Test if the file extension matches one of the globs defined for this type
func (m MimeType) GlobMatches(what string) bool {
	for _, v := range m.Globs {
		if strings.EqualFold(path.Ext(what), strings.Replace(v.Pattern, "*", "", 1)) {
			return true
		}
	}
	return false
}

// AliasMatches Test if the file extension matches one of the globs defined for this type
func (m MimeType) AliasMatches(what string) bool {
	for _, v := range m.Aliases {
		if strings.EqualFold(what, v.Type) {
			return true
		}
	}
	return false
}

type catagories map[string][]MimeType

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
var Catagories catagories = make(catagories)

func loadCategory(dir string) (mimes []MimeType, err error) {
	mimes = make([]MimeType, 0)

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			var mime MimeType
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

func loadMimeTypes(path string) (mimes map[string]string, err error) {
	mimes = make(map[string]string)
	files, _ := ioutil.ReadDir(path)
	for _, fi := range files {
		if fi.IsDir() {
			var c []MimeType
			if c, err = loadCategory(filepath.Join(path, fi.Name())); err != nil {
				return
			}
			Catagories[fi.Name()] = c
		}
	}
	return
}

func init() {
	loadMimeTypes("/usr/share/mime")
}
