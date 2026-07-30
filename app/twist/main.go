// Command twist detects typosquats by edit distance: given a list of target
// strings, it reports each query's nearest target within a small edit budget.
//
//	twist near -t <targets-file> [-all] [-index] [queries...]   # nearest target per query (stdin if no args)
//	twist domain -t <brands-file> [-all] [domains...]           # tld-swap / typo detection over registrable domains
//	twist version
//
// Targets are one per line. Queries are the command arguments, or one per line on
// stdin if none are given. Each near-miss prints as: query <tab> nearest <tab> dist.
// A query with no near-miss is skipped unless -all is set, which marks it "-".
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/netstar-labs/twist"
)

// stamped by build/twist via -ldflags -X.
var (
	version = "dev"
	build   = "none"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "near":
		err = near(os.Args[2:])
	case "domain":
		err = domainCmd(os.Args[2:])
	case "version", "-version", "--version", "-v":
		fmt.Printf("twist %s (%s)\n", version, build)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "twist:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: twist <near|domain|version> [flags] [queries...]")
	os.Exit(2)
}

func near(args []string) error {
	fs := flag.NewFlagSet("near", flag.ExitOnError)
	tfile := fs.String("t", "", "targets file, newline-delimited (required)")
	all := fs.Bool("all", false, "also print queries with no near-miss, marked with -")
	useIndex := fs.Bool("index", false, "query via the BK-tree index (same result, sublinear on a large list)")
	fs.Parse(args)

	if *tfile == "" {
		return errors.New("near: -t <targets-file> is required")
	}
	targets, err := readLines(*tfile)
	if err != nil {
		return err
	}
	s := twist.New(targets)
	// nearest is Set.Nearest, or the equivalent BK-tree lookup with -index — identical
	// (target, dist, ok), sublinear instead of a bucket scan for a large target list.
	nearest := s.Nearest
	if *useIndex {
		nearest = s.Index().Nearest
	}

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	emit := func(q string) {
		if q == "" {
			return
		}
		if near, dist, ok := nearest(q); ok {
			fmt.Fprintf(w, "%s\t%s\t%d\n", q, near, dist)
		} else if *all {
			fmt.Fprintf(w, "%s\t-\t-\n", q)
		}
	}

	if queries := fs.Args(); len(queries) > 0 {
		for _, q := range queries {
			emit(q)
		}
		return nil
	}
	// No query arguments: read one query per line from stdin.
	sc := newLineScanner(os.Stdin)
	for sc.Scan() {
		emit(strings.TrimSpace(sc.Text()))
	}
	return sc.Err()
}

// readLines reads path as newline-delimited entries, trimming surrounding
// whitespace and skipping blank lines.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := newLineScanner(f)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			out = append(out, line)
		}
	}
	return out, sc.Err()
}

// newLineScanner returns a bufio.Scanner over r that tolerates lines up to 1 MiB.
func newLineScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	return sc
}
