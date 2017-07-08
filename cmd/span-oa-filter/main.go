// span-oa-filter will set x.oa to true, if the ISSN of the record is contained in
// a given ISSN list.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"io/ioutil"

	"bytes"

	"github.com/miku/span"
	"github.com/miku/span/bytebatch"
	"github.com/miku/span/container"
	"github.com/miku/span/finc"
)

func main() {
	showVersion := flag.Bool("v", false, "prints current program version")
	issnFile := flag.String("f", "", "path to file with one issn per line")
	flag.Parse()

	if *showVersion {
		fmt.Println(span.AppVersion)
		os.Exit(0)
	}

	b, err := ioutil.ReadFile(*issnFile)
	if err != nil {
		log.Fatal(err)
	}

	issnset := container.NewStringSet()

	for _, v := range bytes.Split(b, []byte("\n")) {
		issnset.Add(string(bytes.TrimSpace(v)))
	}

	p := bytebatch.NewLineProcessor(os.Stdin, os.Stdout, func(b []byte) ([]byte, error) {
		var is finc.IntermediateSchema
		if err := json.Unmarshal(b, &is); err != nil {
			return nil, err
		}
		for _, issn := range is.ISSNList() {
			if issnset.Contains(issn) {
				is.OpenAccess = true
				break
			}
		}
		bb, err := json.Marshal(is)
		if err != nil {
			return bb, err
		}
		bb = append(bb, '\n')
		return bb, nil
	})

	if err := p.Run(); err != nil {
		log.Fatal(err)
	}
}