// Given a set of API responses from crossref, generate a file that contains only
// the latest version of a record, determined by DOI and deposit date.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"bufio"

	"github.com/miku/parallel"
	"github.com/miku/span/formats/crossref"
)

func main() {
	// First the filename, DOI and deposited date are extracted into a temporary file.
	// This file is sorted by DOI and deposited date, only the latest date is kept.
	// Then, for each file extract only the newest records (must keep a list of DOI in
	// memory or maybe in an embedded key value store, say bolt).
	// ...
	// For each file (sha), keep the extracted list compressed and cached at
	// ~/.cache/span-crossref-snapshot/. Also, keep a result cache for a set of files.

	flag.Parse()

	f, err := ioutil.TempFile("", "span-crossref-snapshot-")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	log.Println(f.Name())
	w := bufio.NewWriter(f)

	for _, filename := range flag.Args() {
		log.Println(filename)
		f, err := os.Open(filename)
		if err != nil {
			log.Fatal(err)
		}
		br := bufio.NewReader(f)

		// Close over filename, so we can safely use it with goroutines.
		var createProcessor = func(filename string) *parallel.Processor {
			p := parallel.NewProcessor(br, w, func(b []byte) ([]byte, error) {
				var resp crossref.BulkResponse
				if err := json.Unmarshal(b, &resp); err != nil {
					return nil, err
				}
				var items [][]byte
				for _, doc := range resp.Message.Items {
					date, err := doc.Deposited.Date()
					if err != nil {
						return nil, err
					}
					s := fmt.Sprintf("%s\t%s\t%s", filename, date.Format("2006-01-02"), doc.DOI)
					items = append(items, []byte(s))
				}
				return bytes.Join(items, []byte("\n")), nil
			})
			return p
		}

		// Create, configure, run.
		p := createProcessor(filename)
		p.BatchSize = 5 // Each item might be large.
		if err := p.Run(); err != nil {
			log.Fatal(err)
		}
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}

	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}

	g, err := ioutil.TempFile("", "span-crossref-snapshot-")
	if err != nil {
		log.Fatal(err)
	}
	defer g.Close()

	log.Println("Rewinding.")
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		log.Fatal(err)
	}

	log.Println("Sorting.")
	cmd := exec.Command("sort", "-S25%", "-k3,3", "-k2,2", "-u")
	cmd.Stdin = f
	cmd.Stdout = g
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	log.Println(g.Name())
}
