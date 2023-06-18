package mime

import (
	"io/ioutil"
	"path/filepath"
	re "regexp"
	"strings"
)

// GlobMatches Test if the file extension matches one of the globs defined for this type
func (m *Type) GlobMatches(what string) bool {
	var (
		matcher *re.Regexp
		err     error
	)
	for _, v := range m.Globs {
		if matcher, err = re.Compile("(?i)^." + v.Pattern + "$"); err != nil {
			continue
		}
		if matcher.Match([]byte(what)) {
			return true
		}
	}
	return false
}

// AliasMatches Test if the file extension matches one of the globs defined for this type
func (m *Type) AliasMatches(what string) bool {
	for _, v := range m.Aliases {
		if strings.EqualFold(what, v.Type) {
			return true
		}
	}
	return false
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
