package mime

import "strings"

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
