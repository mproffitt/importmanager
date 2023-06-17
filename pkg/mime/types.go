package mime

import "encoding/xml"

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

type catagories map[string][]Type

// Details Contains basic information about the type
type Details struct {
	Catagory  string   `json:"category"`
	Type      string   `json:"type"`
	SubClass  []string `json:"subclass"`
	Extension string   `json:"extension"`

	// Used to test configuration
	DryRun bool
}
