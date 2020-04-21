// WIP: span-tagger will be a replacement of span-tag, with improvements:
//
// 1. Get rid of a filterconfig JSON format, only use AMSL discovery output
// (turned into an sqlite3 db, via span-amsl-discovery -db ...); that should
// get rid of siskin/amsl.py, span-tag, span-freeze and the whole span/filter
// tree.
//
// 2. Allow for updated file output or just TSV of attachments (which we could
// diff for debugging or other things).
//
// Usage:
//
//     $ span-amsl-discovery -db amsl.db -live https://live.server
//     $ taskcat AIIntermediateSchema | span-tagger -db amsl.db > tagged.ndj
//
// TODO:
//
// * [ ] cover all attachment modes from https://git.io/JvdmC
// * [ ] add tests
// * [ ] logs
//
// Performance:
//
// Single threaded 170M records, about 4 hours, thanks to caching (but only
// about 10M/s); 210m29.179s for 173759327 records; 13G output.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/pprof"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/miku/span"
	"github.com/miku/span/formats/finc"
	"github.com/miku/span/tagging"
	log "github.com/sirupsen/logrus"
)

var (
	force       = flag.Bool("f", false, "force all external referenced links to be downloaded")
	dbFile      = flag.String("db", "", "path to an sqlite3 file generated by span-amsl-discovery -db file.db ...")
	cpuprofile  = flag.String("cpuprofile", "", "file to cpu profile")
	showVersion = flag.Bool("v", false, "prints current program version")
	debug       = flag.Bool("debug", false, "only output id and ISIL")
)

func main() {
	flag.Parse()
	if *showVersion {
		fmt.Println(span.AppVersion)
		os.Exit(0)
	}
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}
	if *dbFile == "" {
		log.Fatal("we need a configuration database")
	}
	labeler, err := tagging.New(*dbFile)
	if err != nil {
		log.Fatal(err)
	}
	var (
		br      = bufio.NewReader(os.Stdin)
		i       = 0
		started = time.Now()
	)
	bw := bufio.NewWriter(os.Stdout)
	defer bw.Flush()
	enc := json.NewEncoder(bw)
	for {
		if i%10000 == 0 {
			log.Printf("%d %0.2f", i, float64(i)/time.Since(started).Seconds())
		}
		b, err := br.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		var doc finc.IntermediateSchema // TODO: try reduced schema
		if err := json.Unmarshal(b, &doc); err != nil {
			log.Fatal(err)
		}
		// TODO: return ISIL
		labels, err := labeler.Labels(&doc)
		if err != nil {
			log.Fatal(err)
		}
		if *debug {
			fmt.Printf("%s\t%s\n", doc.ID, strings.Join(labels, ", "))
		} else {
			doc.Labels = labels
			if err := enc.Encode(doc); err != nil {
				log.Fatal(err)
			}
		}
		i++
	}
}
