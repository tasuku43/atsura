// Package gotestjson validates the one frozen Go test JSONL result admitted by
// the RTK pass-only optimizer contract.
package gotestjson

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/tasuku43/atsura/internal/infra/strictjson"
)

const (
	maxInputBytes  = 4 * 1024 * 1024
	maxRecordBytes = 256 * 1024
	maxRecords     = 65_536
)

// Facts are the independently derived semantic facts needed to validate the
// pass-only optimizer result. Summary never contains a trailing newline.
type Facts struct {
	Package       string
	TestPassCount uint64
	Summary       string
}

// Analyzer adapts Analyze to the application-owned summary-admission port
// without exposing Go-specific event facts above infrastructure.
type Analyzer struct{}

// NewAnalyzer returns the stateless frozen Go test admission adapter.
func NewAnalyzer() *Analyzer { return &Analyzer{} }

// ExpectedSummary returns the independently derived newline-free summary only
// when input satisfies the complete pass-only grammar.
func (*Analyzer) ExpectedSummary(input []byte) (string, bool) {
	facts, eligible := Analyze(input)
	return facts.Summary, eligible
}

type rawEvent struct {
	Time        json.RawMessage `json:"Time"`
	Action      json.RawMessage `json:"Action"`
	Package     json.RawMessage `json:"Package"`
	Test        json.RawMessage `json:"Test"`
	Elapsed     json.RawMessage `json:"Elapsed"`
	Output      json.RawMessage `json:"Output"`
	FailedBuild json.RawMessage `json:"FailedBuild"`
}

type event struct {
	action      string
	packageName string
	test        string
	hasTest     bool
}

type testState uint8

const (
	testRunning testState = iota + 1
	testPaused
	testPassed
)

// Analyze returns independently derived facts only when input is eligible for
// the frozen pass-only optimizer. All syntactic and semantic mismatches are
// ordinary ineligibility: Analyze returns a zero Facts and false, does not
// mutate input, and does not expose a partial interpretation.
func Analyze(input []byte) (Facts, bool) {
	if !validFraming(input) {
		return Facts{}, false
	}

	recordCount := bytes.Count(input, []byte{'\n'})
	if recordCount == 0 || recordCount > maxRecords {
		return Facts{}, false
	}

	var packageName string
	states := make(map[string]testState)
	var passCount uint64
	packageStarted := false
	packagePassed := false
	recordStart := 0

	for recordIndex := 0; recordIndex < recordCount; recordIndex++ {
		relativeEnd := bytes.IndexByte(input[recordStart:], '\n')
		if relativeEnd < 0 {
			return Facts{}, false
		}
		recordEnd := recordStart + relativeEnd
		record := input[recordStart:recordEnd]
		if len(record) == 0 || len(record) > maxRecordBytes || packagePassed {
			return Facts{}, false
		}

		current, ok := decodeEvent(record)
		if !ok {
			return Facts{}, false
		}
		if recordIndex == 0 {
			if current.action != "start" || current.hasTest {
				return Facts{}, false
			}
			packageName = current.packageName
			packageStarted = true
		} else if current.packageName != packageName {
			return Facts{}, false
		}

		switch current.action {
		case "start":
			if recordIndex != 0 || !packageStarted {
				return Facts{}, false
			}
		case "run":
			if _, exists := states[current.test]; exists {
				return Facts{}, false
			}
			states[current.test] = testRunning
		case "output":
			if current.hasTest {
				state, exists := states[current.test]
				if !exists || state == testPassed {
					return Facts{}, false
				}
			}
		case "pause":
			if states[current.test] != testRunning {
				return Facts{}, false
			}
			states[current.test] = testPaused
		case "cont":
			if states[current.test] != testPaused {
				return Facts{}, false
			}
			states[current.test] = testRunning
		case "pass":
			if current.hasTest {
				if states[current.test] != testRunning {
					return Facts{}, false
				}
				states[current.test] = testPassed
				passCount++
			} else {
				if recordIndex != recordCount-1 || passCount == 0 || !allTestsPassed(states) {
					return Facts{}, false
				}
				packagePassed = true
			}
		default:
			return Facts{}, false
		}

		recordStart = recordEnd + 1
	}

	if !packageStarted || !packagePassed || recordStart != len(input) {
		return Facts{}, false
	}
	summary, ok := summaryIfSmaller(passCount, len(input))
	if !ok {
		return Facts{}, false
	}
	return Facts{Package: packageName, TestPassCount: passCount, Summary: summary}, true
}

func validFraming(input []byte) bool {
	if len(input) == 0 || len(input) > maxInputBytes || !utf8.Valid(input) {
		return false
	}
	if bytes.HasPrefix(input, []byte{0xef, 0xbb, 0xbf}) || bytes.IndexByte(input, '\r') >= 0 {
		return false
	}
	if input[len(input)-1] != '\n' {
		return false
	}
	return len(input) == 1 || input[len(input)-2] != '\n'
}

func decodeEvent(record []byte) (event, bool) {
	var raw rawEvent
	if err := strictjson.Decode(record, &raw, 1); err != nil {
		return event{}, false
	}
	timestamp, ok := requiredString(raw.Time)
	if !ok {
		return event{}, false
	}
	if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
		return event{}, false
	}
	action, ok := requiredString(raw.Action)
	if !ok {
		return event{}, false
	}
	packageName, ok := requiredString(raw.Package)
	if !ok || packageName == "" {
		return event{}, false
	}

	test, hasTest, ok := optionalString(raw.Test)
	if !ok || (hasTest && test == "") || len(raw.FailedBuild) != 0 {
		return event{}, false
	}
	hasElapsed := len(raw.Elapsed) != 0
	hasOutput := len(raw.Output) != 0

	switch action {
	case "start":
		if hasTest || hasElapsed || hasOutput {
			return event{}, false
		}
	case "run", "pause", "cont":
		if !hasTest || hasElapsed || hasOutput {
			return event{}, false
		}
	case "output":
		if hasElapsed || !hasOutput {
			return event{}, false
		}
		if _, ok := requiredString(raw.Output); !ok {
			return event{}, false
		}
	case "pass":
		if !hasElapsed || hasOutput {
			return event{}, false
		}
		elapsed, ok := requiredNumber(raw.Elapsed)
		if !ok || elapsed < 0 {
			return event{}, false
		}
	default:
		return event{}, false
	}

	return event{action: action, packageName: packageName, test: test, hasTest: hasTest}, true
}

func requiredString(raw json.RawMessage) (string, bool) {
	value, present, ok := optionalString(raw)
	return value, present && ok
}

func optionalString(raw json.RawMessage) (string, bool, bool) {
	if len(raw) == 0 {
		return "", false, true
	}
	trimmed := bytes.TrimSpace(raw)
	if !validJSONString(trimmed) {
		return "", true, false
	}
	var value string
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return "", true, false
	}
	return value, true, true
}

func requiredNumber(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || (trimmed[0] != '-' && (trimmed[0] < '0' || trimmed[0] > '9')) {
		return 0, false
	}
	var value float64
	if err := json.Unmarshal(trimmed, &value); err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}

func validJSONString(raw []byte) bool {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return false
	}
	for index := 1; index < len(raw)-1; {
		if raw[index] != '\\' {
			if raw[index] < 0x20 || raw[index] == '"' {
				return false
			}
			_, width := utf8.DecodeRune(raw[index : len(raw)-1])
			if width == 0 {
				return false
			}
			index += width
			continue
		}

		index++
		if index >= len(raw)-1 {
			return false
		}
		switch raw[index] {
		case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			index++
		case 'u':
			codeUnit, next, ok := decodeCodeUnit(raw, index)
			if !ok {
				return false
			}
			index = next
			if codeUnit >= 0xd800 && codeUnit <= 0xdbff {
				if index+1 >= len(raw)-1 || raw[index] != '\\' || raw[index+1] != 'u' {
					return false
				}
				low, afterLow, ok := decodeCodeUnit(raw, index+1)
				if !ok || low < 0xdc00 || low > 0xdfff {
					return false
				}
				index = afterLow
			} else if codeUnit >= 0xdc00 && codeUnit <= 0xdfff {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func decodeCodeUnit(raw []byte, uIndex int) (uint16, int, bool) {
	if uIndex+4 >= len(raw)-1 || raw[uIndex] != 'u' {
		return 0, 0, false
	}
	var value uint16
	for index := uIndex + 1; index <= uIndex+4; index++ {
		digit, ok := hexValue(raw[index])
		if !ok {
			return 0, 0, false
		}
		value = value*16 + uint16(digit)
	}
	return value, uIndex + 5, true
}

func hexValue(value byte) (byte, bool) {
	switch {
	case value >= '0' && value <= '9':
		return value - '0', true
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10, true
	case value >= 'A' && value <= 'F':
		return value - 'A' + 10, true
	default:
		return 0, false
	}
}

func allTestsPassed(states map[string]testState) bool {
	if len(states) == 0 {
		return false
	}
	for _, state := range states {
		if state != testPassed {
			return false
		}
	}
	return true
}

func summaryIfSmaller(passCount uint64, inputBytes int) (string, bool) {
	if passCount == 0 || inputBytes <= 0 {
		return "", false
	}
	summary := "Go test: " + strconv.FormatUint(passCount, 10) + " passed in 1 packages"
	return summary, len(summary) < inputBytes
}
