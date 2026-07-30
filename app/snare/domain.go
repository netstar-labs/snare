package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/netstar-labs/snare"
)

// domainCmd reports, for each input domain, whether it is a tld-swap or an
// edit-distance typo of one of the brand registrable domains in -t (the v0.2
// [snare.DomainSet]). Each hit prints as: domain <tab> kind <tab> brand <tab> dist.
// A domain with no hit is skipped unless -all is set, which marks it "-".
func domainCmd(args []string) error {
	fs := flag.NewFlagSet("domain", flag.ExitOnError)
	tfile := fs.String("t", "", "brand registrable-domains file, newline-delimited (required)")
	all := fs.Bool("all", false, "also print domains with no hit, marked with -")
	fs.Parse(args)

	if *tfile == "" {
		return errors.New("domain: -t <brands-file> is required")
	}
	brands, err := readLines(*tfile)
	if err != nil {
		return err
	}
	d := snare.NewDomainSet(brands)

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	emit := func(dom string) {
		if dom == "" {
			return
		}
		if r, ok := d.Check(dom); ok {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", dom, r.Kind, r.Brand, r.Dist)
		} else if *all {
			fmt.Fprintf(w, "%s\t-\t-\t-\n", dom)
		}
	}

	if doms := fs.Args(); len(doms) > 0 {
		for _, dom := range doms {
			emit(dom)
		}
		return nil
	}
	// No domain arguments: read one per line from stdin.
	sc := newLineScanner(os.Stdin)
	for sc.Scan() {
		emit(strings.TrimSpace(sc.Text()))
	}
	return sc.Err()
}
