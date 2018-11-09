// Common methods uses by template and KV commands

package main

import (
	"io"
	"strings"

	"github.com/spf13/viper"
)

// Parse config file to a map
func configToMap(cType string, r io.Reader) (map[string]interface{}, error) {
	viper.SetConfigType(cType)
	err := viper.ReadConfig(r)
	if err != nil {
		return nil, err
	}

	return viper.AllSettings(), nil
}

// Check if given input format is supported.
func isValidInputFormat(format string, availFormats []string) bool {
	for _, f := range availFormats {
		if f == format {
			return true
		}
	}

	return false
}

// Convert array of string to string
func formatsToString(formats []string) string {
	return strings.Join(formats, ", ")
}
