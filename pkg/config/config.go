package config

import (
	"fmt"
	"io/ioutil"
	"time"

	yaml "gopkg.in/yaml.v2"
)

// DefaultHandlers a set of valid handler
var DefaultHandlers []string = []string{
	"copy",
	"move",
	"extract",
	"install",
	"delete",
}

// Processor How to handle a particular file type
type Processor struct {
	Type       string            `yaml:"type"`
	Path       string            `yaml:"path"`
	Handler    string            `yaml:"handler"`
	Properties map[string]string `yaml:"properties"`
}

func (p *Processor) String() string {
	return fmt.Sprintf("%s (%s)", p.Handler, p.Type)
}

// Config Global config for the application
type Config struct {
	Watch           []string      `yaml:"watch"`
	Processors      []Processor   `yaml:"processors"`
	DelayInSeconds  time.Duration `yaml:"delayInSeconds"`
	CleanupZeroByte bool          `yaml:"cleanupZeroByte"`
}

// New Load the config file
func New(configFile string) (c *Config, err error) {
	f, err := ioutil.ReadFile(configFile)
	if err != nil {
		return
	}

	if err = yaml.Unmarshal(f, &c); err != nil {
		return
	}
	return c, nil
}
