package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/hashicorp/hcl"
	"github.com/hashicorp/hcl/hcl/printer"
	"github.com/magiconair/properties"
	toml "github.com/pelletier/go-toml"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

// Run template command.
func runTmplCmd(cmd *cobra.Command, args []string) {
	// Check if input format is supported.
	if !isValidInputFormat(tmplInputType, tmplAvailableFormats) {
		errLog.Fatalf("Invalid input file format - %s. Available options are: %s", tmplInputType, formatsToString(tmplAvailableFormats))
	}

	var inputs []io.Reader

	// Add stdin as default input if files are not provided else add given input files.
	if len(args) == 0 {
		inputs = append(inputs, os.Stdin)
	} else {
		// Add all files as inputs
		for _, fname := range args {
			f, err := os.Open(fname)
			if err != nil {
				errLog.Fatalf("Error: error opening input file - %v", err)
			}

			inputs = append(inputs, f)
		}
	}

	var out string
	for _, i := range inputs {
		// Get config  map
		cMap, err := configToMap(tmplInputType, i)
		// Recursively update values for keys of config map.
		updateValue(tmplKeyPrefix, "", cMap)

		// Convert map to config
		o, err := mapToConfigString(tmplInputType)
		if err != nil {
			errLog.Fatalf("Error: error converting config map to config - %v", err)
		}

		// Append outputs.
		out += o
	}

	// Remove " from template wrap
	out = strings.Replace(out, `"{{`, "{{", -1)
	out = strings.Replace(out, `}}"`, "}}", -1)
	// Replace custom pattern for " with "
	out = strings.Replace(out, `{{{`, `"`, -1)
	out = strings.Replace(out, `}}}`, `"`, -1)

	// Print output
	sysLog.Println(out)
}

// Convert map to give config file type.
func mapToConfigString(configType string) (string, error) {
	cMap := viper.AllSettings()
	switch configType {
	case "json":
		b, err := json.MarshalIndent(cMap, "", "  ")
		if err != nil {
			return "", err
		}

		return string(b), nil

	case "hcl":
		b, err := json.Marshal(cMap)
		ast, err := hcl.Parse(string(b))
		if err != nil {
			return "", err
		}

		buffer := bytes.NewBufferString("")
		err = printer.Fprint(buffer, ast.Node)
		if err != nil {
			return "", err
		}

		return buffer.String(), nil

	case "prop", "props", "properties":
		p := properties.NewProperties()
		for _, key := range viper.AllKeys() {
			_, _, err := p.Set(key, viper.GetString(key))
			if err != nil {
				return "", err
			}
		}

		buffer := bytes.NewBufferString("")
		_, err := p.WriteComment(buffer, "#", properties.UTF8)
		if err != nil {
			return "", err
		}

		return buffer.String(), nil

	case "toml":
		t, err := toml.TreeFromMap(cMap)
		if err != nil {
			return "", err
		}
		return t.String(), nil

	case "yaml", "yml":
		b, err := yaml.Marshal(cMap)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "", nil
	}
}

// Recursively update value of key with consul template keyOrDefault. If value is a map recurse further.
func updateValue(prefix string, key string, inp map[string]interface{}) {
	for k, v := range inp {
		var newKey string
		var newPrefix string
		// If prefix is empty then don't append "/" else form a new prefix with current key.
		if prefix == "" {
			newPrefix = k
		} else {
			newPrefix = prefix + "/" + k
		}

		// Update key to be used to update viper config.
		// Nested viper keys are represented by . notation. Ex: a.b.c
		if key == "" {
			newKey = k
		} else {
			newKey = key + "." + k
		}

		// Check if value is a map. If map then traverse further else update the value with consul template keyOrUpdate.
		vKind := reflect.TypeOf(v).Kind()
		if vKind == reflect.Map {
			m, ok := v.(map[string]interface{})
			if !ok {
				errLog.Fatalf("invalid value - %v: %v", k, v)
			}

			// Recursion.
			updateValue(newPrefix, newKey, m)
		} else {
			// Custom JSON encoder to not escape HTML characters like &, < and >.
			val := bytes.NewBufferString("")
			jEncoder := json.NewEncoder(val)
			jEncoder.SetEscapeHTML(false)

			// JSON encode value to preserve the type.
			if err := jEncoder.Encode(v); err != nil {
				errLog.Fatalf("error while marshalling value: %v err: %v", v, err)
			}

			// Remove white space from buffer io value.
			out := strings.TrimSuffix(val.String(), "\n")

			// Update value for current key.
			viper.Set(newKey, fmt.Sprintf(`{{ keyOrDefault {{{%v}}} {{{%v}}} }}`, newPrefix, out))
		}
	}
}
