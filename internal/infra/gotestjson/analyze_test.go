package gotestjson

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/tasuku43/atsura/internal/infra/strictjson"
)

const fixtureTime = "2026-07-22T00:00:00.123456789Z"

type presentationAnswer struct {
	SchemaVersion int      `json:"schema_version"`
	Task          string   `json:"task"`
	SourceArgv    []string `json:"source_argv"`
	Package       string   `json:"package"`
	TestPassCount uint64   `json:"test_pass_count"`
	PackageCount  int      `json:"package_count"`
	Summary       string   `json:"summary"`
	Omitted       []string `json:"omitted"`
}

func TestFrozenPresentationFixtureMatchesIndependentAnswerKey(t *testing.T) {
	input, err := os.ReadFile("testdata/pass.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	rawAnswer, err := os.ReadFile("testdata/pass.answer.json")
	if err != nil {
		t.Fatal(err)
	}
	var answer presentationAnswer
	if err := strictjson.Decode(rawAnswer, &answer, 3); err != nil {
		t.Fatal(err)
	}
	wantOmitted := []string{"elapsed", "event_order", "package_identity", "test_names", "test_output"}
	if answer.SchemaVersion != 1 || answer.Task != "go test" ||
		!reflect.DeepEqual(answer.SourceArgv, []string{"go", "test", "-json"}) ||
		answer.PackageCount != 1 || !reflect.DeepEqual(answer.Omitted, wantOmitted) {
		t.Fatalf("answer key = %+v", answer)
	}
	facts, eligible := Analyze(input)
	if !eligible || facts.Package != answer.Package || facts.TestPassCount != answer.TestPassCount || facts.Summary != answer.Summary {
		t.Fatalf("Analyze(frozen fixture) = %+v, %v; answer=%+v", facts, eligible, answer)
	}
	if len(facts.Summary) >= len(input) || strings.Contains(facts.Summary, "\n") {
		t.Fatalf("summary is not a strictly smaller newline-free result: %q", facts.Summary)
	}
}

func TestAnalyzeAcceptsRealGoShapedPassStreamWithoutChangingInput(t *testing.T) {
	input := []byte("" +
		`{"Time":"2026-07-22T14:26:30.183508+09:00","Action":"start","Package":"example.com/project/check"}` + "\n" +
		`{"Time":"2026-07-22T14:26:30.482764+09:00","Action":"run","Package":"example.com/project/check","Test":"TestOne"}` + "\n" +
		`{"Time":"2026-07-22T14:26:30.482879+09:00","Action":"output","Package":"example.com/project/check","Test":"TestOne","Output":"=== RUN   TestOne\n"}` + "\n" +
		`{"Time":"2026-07-22T14:26:30.483291+09:00","Action":"output","Package":"example.com/project/check","Test":"TestOne","Output":"--- PASS: TestOne (0.00s)\n"}` + "\n" +
		`{"Time":"2026-07-22T14:26:30.483312+09:00","Action":"pass","Package":"example.com/project/check","Test":"TestOne","Elapsed":0}` + "\n" +
		`{"Time":"2026-07-22T14:26:30.483328+09:00","Action":"run","Package":"example.com/project/check","Test":"TestTwo"}` + "\n" +
		`{"Time":"2026-07-22T14:26:30.483362+09:00","Action":"pass","Package":"example.com/project/check","Test":"TestTwo","Elapsed":0.01}` + "\n" +
		`{"Time":"2026-07-22T14:26:30.483369+09:00","Action":"output","Package":"example.com/project/check","Output":"PASS\n"}` + "\n" +
		`{"Time":"2026-07-22T14:26:30.484725+09:00","Action":"output","Package":"example.com/project/check","Output":"ok  \texample.com/project/check\t0.301s\n"}` + "\n" +
		`{"Time":"2026-07-22T14:26:30.489437+09:00","Action":"pass","Package":"example.com/project/check","Elapsed":0.306}` + "\n")
	original := append([]byte(nil), input...)

	facts, eligible := Analyze(input)
	if !eligible {
		t.Fatal("Analyze() rejected a real Go-shaped passing stream")
	}
	if facts.Package != "example.com/project/check" || facts.TestPassCount != 2 || facts.Summary != "Go test: 2 passed in 1 packages" {
		t.Fatalf("Analyze() facts = %+v", facts)
	}
	if summary, admitted := NewAnalyzer().ExpectedSummary(input); !admitted || summary != facts.Summary {
		t.Fatalf("ExpectedSummary() = %q, %v", summary, admitted)
	}
	if !bytes.Equal(input, original) {
		t.Fatal("Analyze() mutated its source bytes")
	}
}

func TestAnalyzeAcceptsSubtestAndParallelLifecycle(t *testing.T) {
	input := records(
		packageRecord("start", "example.com/project", ""),
		testRecord("run", "example.com/project", "TestParent", ""),
		testRecord("output", "example.com/project", "TestParent", `,"Output":"=== RUN   TestParent\\n"`),
		testRecord("run", "example.com/project", "TestParent/parallel", ""),
		testRecord("output", "example.com/project", "TestParent/parallel", `,"Output":"=== PAUSE TestParent/parallel\\n"`),
		testRecord("pause", "example.com/project", "TestParent/parallel", ""),
		testRecord("output", "example.com/project", "TestParent/parallel", `,"Output":"buffered while paused\\n"`),
		testRecord("run", "example.com/project", "TestSibling", ""),
		testRecord("pass", "example.com/project", "TestSibling", `,"Elapsed":0`),
		testRecord("cont", "example.com/project", "TestParent/parallel", ""),
		testRecord("output", "example.com/project", "TestParent/parallel", `,"Output":"=== CONT TestParent/parallel\\n"`),
		testRecord("pass", "example.com/project", "TestParent/parallel", `,"Elapsed":0.01`),
		testRecord("pass", "example.com/project", "TestParent", `,"Elapsed":0.02`),
		packageRecord("output", "example.com/project", `,"Output":"PASS\\n π \ud83d\ude80"`),
		packageRecord("pass", "example.com/project", `,"Elapsed":0.03`),
	)

	facts, eligible := Analyze(input)
	if !eligible || facts.TestPassCount != 3 || facts.Summary != "Go test: 3 passed in 1 packages" {
		t.Fatalf("Analyze() = %+v, %v", facts, eligible)
	}
}

func TestAnalyzeRejectsHostileFramingLimitsAndJSON(t *testing.T) {
	valid := validOneTestStream()
	invalidUTF8 := append([]byte(nil), valid...)
	invalidUTF8[bytes.Index(invalidUTF8, []byte("example.com/project"))] = 0xff
	oversizedRecord := records(
		packageRecord("start", "example.com/project", ""),
		testRecord("run", "example.com/project", "TestOne", ""),
		testRecord("output", "example.com/project", "TestOne", `,"Output":"`+strings.Repeat("x", maxRecordBytes)+`"`),
		testRecord("pass", "example.com/project", "TestOne", `,"Elapsed":0`),
		packageRecord("pass", "example.com/project", `,"Elapsed":0`),
	)

	tests := []struct {
		name  string
		input []byte
	}{
		{name: "empty", input: nil},
		{name: "only newline", input: []byte("\n")},
		{name: "BOM", input: append([]byte{0xef, 0xbb, 0xbf}, valid...)},
		{name: "literal carriage return", input: bytes.Replace(valid, []byte("\n"), []byte("\r\n"), 1)},
		{name: "blank record", input: bytes.Replace(valid, []byte("\n"), []byte("\n\n"), 1)},
		{name: "missing final LF", input: append([]byte(nil), valid[:len(valid)-1]...)},
		{name: "extra final LF", input: append(append([]byte(nil), valid...), '\n')},
		{name: "invalid UTF-8", input: invalidUTF8},
		{name: "aggregate oversized", input: bytes.Repeat([]byte("x"), maxInputBytes+1)},
		{name: "record oversized", input: oversizedRecord},
		{name: "too many records", input: bytes.Repeat([]byte("{}\n"), maxRecords+1)},
		{name: "duplicate field", input: replaceRecord(valid, 0, packageRecord("start", "example.com/project", `,"Action":"start"`))},
		{name: "unknown field", input: replaceRecord(valid, 0, packageRecord("start", "example.com/project", `,"Unknown":true`))},
		{name: "nested value", input: replaceRecord(valid, 0, `{"Time":{"nested":true},"Action":"start","Package":"example.com/project"}`)},
		{name: "array root", input: replaceRecord(valid, 0, `[]`)},
		{name: "string type mismatch", input: replaceRecord(valid, 0, `{"Time":1,"Action":"start","Package":"example.com/project"}`)},
		{name: "number type mismatch", input: replaceRecord(valid, 3, testRecord("pass", "example.com/project", "TestOne", `,"Elapsed":"0"`))},
		{name: "explicit null", input: replaceRecord(valid, 0, `{"Time":null,"Action":"start","Package":"example.com/project"}`)},
		{name: "trailing JSON value", input: replaceRecord(valid, 0, packageRecord("start", "example.com/project", "")+` {}`)},
		{name: "unpaired high surrogate", input: replaceRecord(valid, 2, testRecord("output", "example.com/project", "TestOne", `,"Output":"\ud800"`))},
		{name: "unpaired low surrogate", input: replaceRecord(valid, 2, testRecord("output", "example.com/project", "TestOne", `,"Output":"\udc00"`))},
		{name: "invalid time", input: replaceRecord(valid, 0, `{"Time":"not-rfc3339","Action":"start","Package":"example.com/project"}`)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertIneligible(t, test.input)
		})
	}
}

func TestAnalyzeRejectsFrozenFieldAndActionViolations(t *testing.T) {
	valid := validOneTestStream()
	tests := []struct {
		name   string
		record int
		value  string
	}{
		{name: "start missing Time", record: 0, value: `{"Action":"start","Package":"example.com/project"}`},
		{name: "start missing Action", record: 0, value: `{"Time":"` + fixtureTime + `","Package":"example.com/project"}`},
		{name: "start missing Package", record: 0, value: `{"Time":"` + fixtureTime + `","Action":"start"}`},
		{name: "empty Package", record: 0, value: packageRecord("start", "", "")},
		{name: "start has Test", record: 0, value: testRecord("start", "example.com/project", "TestOne", "")},
		{name: "start has Elapsed", record: 0, value: packageRecord("start", "example.com/project", `,"Elapsed":0`)},
		{name: "start has Output", record: 0, value: packageRecord("start", "example.com/project", `,"Output":"x"`)},
		{name: "run missing Test", record: 1, value: packageRecord("run", "example.com/project", "")},
		{name: "run has empty Test", record: 1, value: testRecord("run", "example.com/project", "", "")},
		{name: "run has Output", record: 1, value: testRecord("run", "example.com/project", "TestOne", `,"Output":"x"`)},
		{name: "run has Elapsed", record: 1, value: testRecord("run", "example.com/project", "TestOne", `,"Elapsed":0`)},
		{name: "output missing Output", record: 2, value: testRecord("output", "example.com/project", "TestOne", "")},
		{name: "output has Elapsed", record: 2, value: testRecord("output", "example.com/project", "TestOne", `,"Output":"x","Elapsed":0`)},
		{name: "pause missing Test", record: 2, value: packageRecord("pause", "example.com/project", "")},
		{name: "pause has Output", record: 2, value: testRecord("pause", "example.com/project", "TestOne", `,"Output":"x"`)},
		{name: "cont missing Test", record: 2, value: packageRecord("cont", "example.com/project", "")},
		{name: "cont has Elapsed", record: 2, value: testRecord("cont", "example.com/project", "TestOne", `,"Elapsed":0`)},
		{name: "pass missing Elapsed", record: 3, value: testRecord("pass", "example.com/project", "TestOne", "")},
		{name: "pass has Output", record: 3, value: testRecord("pass", "example.com/project", "TestOne", `,"Elapsed":0,"Output":"x"`)},
		{name: "negative Elapsed", record: 3, value: testRecord("pass", "example.com/project", "TestOne", `,"Elapsed":-0.1`)},
		{name: "null Elapsed", record: 3, value: testRecord("pass", "example.com/project", "TestOne", `,"Elapsed":null`)},
		{name: "FailedBuild on admitted action", record: 3, value: testRecord("pass", "example.com/project", "TestOne", `,"Elapsed":0,"FailedBuild":"example.com/project"`)},
		{name: "skip", record: 3, value: testRecord("skip", "example.com/project", "TestOne", `,"Elapsed":0`)},
		{name: "fail", record: 3, value: testRecord("fail", "example.com/project", "TestOne", `,"Elapsed":0`)},
		{name: "bench", record: 3, value: testRecord("bench", "example.com/project", "TestOne", `,"Output":"x"`)},
		{name: "build action", record: 3, value: packageRecord("build", "example.com/project", `,"FailedBuild":"example.com/project"`)},
		{name: "unknown action", record: 3, value: testRecord("future", "example.com/project", "TestOne", "")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertIneligible(t, replaceRecord(valid, test.record, test.value))
		})
	}
}

func TestAnalyzeRejectsIdentityLifecycleAndTerminalConflicts(t *testing.T) {
	pkg := "example.com/project"
	tests := []struct {
		name  string
		input []byte
	}{
		{name: "first event is not package start", input: records(testRecord("run", pkg, "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "second package start", input: records(packageRecord("start", pkg, ""), packageRecord("start", pkg, ""), testRecord("run", pkg, "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "package identity conflict", input: records(packageRecord("start", pkg, ""), testRecord("run", "example.com/other", "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "duplicate run", input: records(packageRecord("start", pkg, ""), testRecord("run", pkg, "TestOne", ""), testRecord("run", pkg, "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "output before run", input: records(packageRecord("start", pkg, ""), testRecord("output", pkg, "TestOne", `,"Output":"x"`), testRecord("run", pkg, "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "pause before run", input: records(packageRecord("start", pkg, ""), testRecord("pause", pkg, "TestOne", ""), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "continue before pause", input: records(packageRecord("start", pkg, ""), testRecord("run", pkg, "TestOne", ""), testRecord("cont", pkg, "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "duplicate pause", input: records(packageRecord("start", pkg, ""), testRecord("run", pkg, "TestOne", ""), testRecord("pause", pkg, "TestOne", ""), testRecord("pause", pkg, "TestOne", ""), testRecord("cont", pkg, "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "pass while paused", input: records(packageRecord("start", pkg, ""), testRecord("run", pkg, "TestOne", ""), testRecord("pause", pkg, "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "output after terminal", input: records(packageRecord("start", pkg, ""), testRecord("run", pkg, "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), testRecord("output", pkg, "TestOne", `,"Output":"late"`), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "duplicate test pass", input: records(packageRecord("start", pkg, ""), testRecord("run", pkg, "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "unfinished test at package pass", input: records(packageRecord("start", pkg, ""), testRecord("run", pkg, "TestOne", ""), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "no test pass", input: records(packageRecord("start", pkg, ""), packageRecord("output", pkg, `,"Output":"PASS\\n"`), packageRecord("pass", pkg, `,"Elapsed":0`))},
		{name: "missing package pass", input: records(packageRecord("start", pkg, ""), testRecord("run", pkg, "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`))},
		{name: "event after package pass", input: records(packageRecord("start", pkg, ""), testRecord("run", pkg, "TestOne", ""), testRecord("pass", pkg, "TestOne", `,"Elapsed":0`), packageRecord("pass", pkg, `,"Elapsed":0`), packageRecord("output", pkg, `,"Output":"late"`))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertIneligible(t, test.input)
		})
	}
}

func TestSummaryMustBeStrictlySmaller(t *testing.T) {
	summary, ok := summaryIfSmaller(1, 1_000)
	if !ok || summary != "Go test: 1 passed in 1 packages" {
		t.Fatalf("summaryIfSmaller() = %q, %v", summary, ok)
	}
	if _, ok := summaryIfSmaller(1, len(summary)); ok {
		t.Fatal("summary equal in size to its input was admitted")
	}
	if _, ok := summaryIfSmaller(1, len(summary)-1); ok {
		t.Fatal("summary larger than its input was admitted")
	}
	if got, ok := summaryIfSmaller(1, len(summary)+1); !ok || got != summary {
		t.Fatalf("strictly smaller summary = %q, %v", got, ok)
	}
	if _, ok := summaryIfSmaller(0, 1_000); ok {
		t.Fatal("zero passing tests were admitted")
	}
}

func validOneTestStream() []byte {
	return records(
		packageRecord("start", "example.com/project", ""),
		testRecord("run", "example.com/project", "TestOne", ""),
		testRecord("output", "example.com/project", "TestOne", `,"Output":"=== RUN   TestOne\\n"`),
		testRecord("pass", "example.com/project", "TestOne", `,"Elapsed":0`),
		packageRecord("output", "example.com/project", `,"Output":"PASS\\n"`),
		packageRecord("pass", "example.com/project", `,"Elapsed":0.01`),
	)
}

func packageRecord(action, packageName, tail string) string {
	return fmt.Sprintf(`{"Time":"%s","Action":"%s","Package":"%s"%s}`, fixtureTime, action, packageName, tail)
}

func testRecord(action, packageName, testName, tail string) string {
	return fmt.Sprintf(`{"Time":"%s","Action":"%s","Package":"%s","Test":"%s"%s}`, fixtureTime, action, packageName, testName, tail)
}

func records(values ...string) []byte {
	return []byte(strings.Join(values, "\n") + "\n")
}

func replaceRecord(input []byte, index int, replacement string) []byte {
	values := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	values[index] = replacement
	return records(values...)
}

func assertIneligible(t *testing.T, input []byte) {
	t.Helper()
	original := append([]byte(nil), input...)
	facts, eligible := Analyze(input)
	if eligible || facts != (Facts{}) {
		t.Fatalf("Analyze() = %+v, %v; want zero facts and false", facts, eligible)
	}
	if !bytes.Equal(input, original) {
		t.Fatal("Analyze() mutated ineligible source bytes")
	}
}
