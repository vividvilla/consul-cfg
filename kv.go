package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
)

// Run KV commands.
func runKVCmd(cmd *cobra.Command, args []string) {
	// Check if input format is supported.
	if !isValidInputFormat(kvInputType, kvAvailableFormats) {
		errLog.Fatalf("Invalid input file format - %s. Available options are: %s", kvInputType, formatsToString(kvAvailableFormats))
	}

	var inputs []io.Reader
	var output []consulKVPair

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

	for _, i := range inputs {
		// Process toml inputs
		m, err := configToMap(kvInputType, i)
		if err != nil {
			errLog.Fatalf("Error: error parsing input - %v", err)
		}

		// Recursively parse map and add KV pairs
		mapToKVPairs(&output, kvKeyPrefix, m)
	}

	// Print JSON output
	bytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		errLog.Fatalf("error marshelling output: %v", err)
	}

	sysLog.Println(string(bytes[:]))
}

// Recursively traverse map and insert KV Pair to output if it can't be further traversed.
func mapToKVPairs(ckv *[]consulKVPair, prefix string, inp map[string]interface{}) {
	for k, v := range inp {
		var newPrefix string

		// If prefix is empty then don't append "/" else form a new prefix with current key.
		if prefix == "" {
			newPrefix = k
		} else {
			newPrefix = prefix + "/" + k
		}

		// Check if value is a map. If map then traverse further else write to output as a KVPair.
		vKind := reflect.TypeOf(v).Kind()
		if vKind == reflect.Map {
			// Check if map value is of interface type and keys are string.
			m, ok := v.(map[string]interface{})
			if !ok {
				errLog.Fatalf("not ok: %v - %v\n", k, v)
			}

			// Recursion.
			mapToKVPairs(ckv, newPrefix, m)
		} else {
			// If its not  string then encode it using JSON
			// CAVEAT: TOML supports array of maps but consul KV doesn't support this so it will be JSON marshalled.

			// Custom JSON marshaller with safe escaped html is disabled. Since default JSON marshaller escapes &, > and <.
			val := bytes.NewBufferString("")
			jEncoder := json.NewEncoder(val)
			jEncoder.SetEscapeHTML(false)

			// JSON encode value to preserve the type.
			if err := jEncoder.Encode(v); err != nil {
				errLog.Fatalf("error while marshalling value: %v err: %v", v, err)
			}

			// Trim new lines if any at the end
			trimmed := strings.TrimSuffix(val.String(), "\n")

			// Base64 encode JSON encoded values since Consul reads only base64 encoded values.
			b64Encoded := base64.StdEncoding.EncodeToString([]byte(trimmed))

			// Append KV pair to results.
			*ckv = append(*ckv, consulKVPair{
				Flags: 0,
				Key:   newPrefix,
				Value: b64Encoded,
			})
		}
	}
}
