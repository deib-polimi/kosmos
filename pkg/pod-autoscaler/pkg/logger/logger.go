package logger

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"k8s.io/apimachinery/pkg/runtime"
)

// Logger is used to log objects in a given format to a file
type Logger struct {
	path string
}

// NewLogger opens the log file and creates a new Logger instance
func NewLogger(path string) (*Logger, error) {
	abs, err := filepath.Abs(path)

	if err != nil {
		return nil, err
	}

	return &Logger{
		path: abs,
	}, nil
}

// Log the Object in JSON format
func (l Logger) Log(obj runtime.Object) error {
	data, err := json.Marshal(obj)

	if err != nil {
		return err
	}

	return ioutil.WriteFile(l.path, append(data, []byte("\n")...), 0777)
}
