package core

import "fmt"

// ConfigError represents a configuration error
type ConfigError struct {
	msg string
}

func (e ConfigError) Error() string {
	return e.msg
}

// ErrInvalidConfig creates a new configuration error
func ErrInvalidConfig(msg string) error {
	return ConfigError{msg: msg}
}

// ErrInvalidConfigf creates a new formatted configuration error
func ErrInvalidConfigf(format string, args ...interface{}) error {
	return ConfigError{msg: fmt.Sprintf(format, args...)}
}
