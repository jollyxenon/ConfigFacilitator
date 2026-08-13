package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

// successEnvelope is the stable JSON representation of a successful command.
type successEnvelope struct {
	OK   bool `json:"ok"`
	Data any  `json:"data"`
}

// errorEnvelope is the stable JSON representation of a failed command.
type errorEnvelope struct {
	OK    bool             `json:"ok"`
	Error commandErrorBody `json:"error"`
}

// commandErrorBody contains stable machine fields for one command failure.
type commandErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// output renders one command result through the selected human or JSON contract.
type output struct {
	stdout io.Writer
	json   bool
}

// HumanResult contains concise human output plus structured JSON data for one success.
type HumanResult struct {
	Message string
	Data    any
}

// renderSuccess writes exactly one JSON object or one concise human result.
func (renderer output) renderSuccess(result HumanResult) error {
	if renderer.json {
		return encodeJSON(renderer.stdout, successEnvelope{OK: true, Data: result.Data})
	}
	if result.Message != "" {
		_, err := fmt.Fprintln(renderer.stdout, result.Message)
		return err
	}
	return nil
}

// renderCommandError writes exactly one JSON error object or one concise human error line.
func renderCommandError(stderr io.Writer, commandErr *CommandError, jsonMode bool) {
	if jsonMode {
		_ = encodeJSON(stderr, errorEnvelope{
			OK: false,
			Error: commandErrorBody{
				Code:    commandErr.Code,
				Message: commandErr.Message,
				Details: commandErr.Details,
			},
		})
		return
	}
	_, _ = fmt.Fprintln(stderr, commandErr.Message)
}

// encodeJSON writes one compact newline-terminated JSON object.
func encodeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

// requestedJSON reports whether the invocation explicitly requested machine output.
func requestedJSON(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
}
