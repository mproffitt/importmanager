package processing

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	c "github.com/mproffitt/importmanager/pkg/config"
	m "github.com/mproffitt/importmanager/pkg/mime"
	log "github.com/sirupsen/logrus"
)

type arguments struct {
	Source      string            `json:"source"`
	Destination string            `json:"destination"`
	Details     m.Details         `json:"details"`
	Properties  map[string]string `json:"properties"`
}

func toArgumentJSON(source, dest string, details *m.Details, properties map[string]string) string {
	var a arguments = arguments{
		Source:      source,
		Destination: dest,
		Details:     *details,
		Properties:  properties,
	}

	b, _ := json.Marshal(a)
	return string(b)
}

func runPlugin(source, dest string, details *m.Details, processor *c.Processor) (final string, err error) {
	if _, err = os.Stat(processor.Handler); os.IsNotExist(err) {
		err = fmt.Errorf("Plugin file has been moved or deleted from disk. %s", err)
		return
	}

	var (
		executable string
		args       string = toArgumentJSON(source, dest, details, processor.Properties)
		response   []byte
	)

	switch strings.ToLower(filepath.Ext(processor.Handler)) {
	case ".py":
		executable = "python"
	case ".sh":
		executable = "sh"
	case ".bash":
		executable = "bash"
	default:
		err = fmt.Errorf("Invalid plugin filetype")
		return
	}

	cmd := exec.Command(executable, []string{processor.Handler, args}...)
	reader, _ := cmd.StdoutPipe()
	cmd.Stderr = cmd.Stdout
	done := make(chan bool)
	scanner := bufio.NewScanner(reader)

	go func() {
		var line string
		for scanner.Scan() {
			line = scanner.Text()
			log.Info(line)
		}
		// if the last line of output is a valid system path,
		// we use that as final for post processing
		if _, err = os.Stat(line); err == nil {
			final = line
		}
		done <- true
	}()

	log.Infof("Triggering plugin command: %s", cmd.String())
	if err = cmd.Start(); err != nil {
		err = fmt.Errorf("Error in plugin %s: %s - %s", processor.Handler, err.Error(), string(response))
		return
	}
	<-done

	err = cmd.Wait()
	log.Infof("Using '%s' as final destination", final)
	return
}
