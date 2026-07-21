// Command sourcefixture is a synthetic JSON-producing source CLI for local Atsura verification.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

type item struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"`
	Ignored string `json:"ignored"`
}

func main() {
	limit := flag.Int("limit", 2, "number of synthetic records")
	format := flag.String("format", "", "output format")
	flag.Parse()
	if *format != "json" || *limit < 0 || *limit > 2 {
		_, _ = fmt.Fprintln(os.Stderr, "sourcefixture requires --format=json and --limit from 0 through 2")
		os.Exit(2)
	}
	items := []item{
		{Number: 101, Title: "Review policy", State: "OPEN", Ignored: "not returned"},
		{Number: 102, Title: "Verify output", State: "CLOSED", Ignored: "not returned"},
	}
	if err := json.NewEncoder(os.Stdout).Encode(items[:*limit]); err != nil {
		os.Exit(1)
	}
}
