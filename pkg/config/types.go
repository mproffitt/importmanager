package config

import (
	"sync"
	"time"

	m "github.com/mproffitt/importmanager/pkg/mime"
)

// Path A path object for processors
type Path struct {
	Path       string      `yaml:"path"`
	Processors []Processor `yaml:"processors"`
}

// Config Global config for the application
type Config struct {
	sync.RWMutex
	Paths           []Path        `yaml:"paths"`
	DelayInSeconds  time.Duration `yaml:"delayInSeconds"`
	CleanupZeroByte bool          `yaml:"cleanupZeroByte"`
	PluginPath      string        `yaml:"pluginDirectory"`
	BufferSize      int           `yaml:"bufferSize"`
	LogLevel        string        `yaml:"logLevel"`
	MimeDirectories []string      `yaml:"mimeDirectories"`
	pathHandler     handler
}

// Processor How to handle a particular file type
type Processor struct {
	Type       string            `yaml:"type"`
	Path       string            `yaml:"path"`
	Handler    string            `yaml:"handler"`
	Properties map[string]string `yaml:"properties"`
	Negated    bool
}

// handler type to allow the passing of the handler.Handle function into the dryrun
//
// See: `handle.Handle`
type handler func(path string, details m.Details, processors []Processor, czb bool) (err error)

// Lockable type to handle dry run paths
type dryRun struct {
	sync.RWMutex
	paths map[string]string
}
