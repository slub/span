// Converts intermediate schema docs into solr docs.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/miku/span"
	"github.com/miku/span/finc"
	"github.com/miku/span/holdings"
)

// Options for worker.
type options struct {
	Holdings holdings.IsilIssnHolding
}

// worker iterates over string batches
func worker(queue chan []string, out chan []byte, opts options, wg *sync.WaitGroup) {
	defer wg.Done()
	for batch := range queue {
		for _, s := range batch {
			is := new(finc.IntermediateSchema)
			err := json.Unmarshal([]byte(s), is)
			if err != nil {
				log.Fatal(err)
			}
			ss, err := is.ToSolrSchema(opts.Holdings)
			if err != nil {
				log.Fatal(err)
			}
			b, err := json.Marshal(ss)
			if err != nil {
				log.Fatal(err)
			}
			out <- b
		}
	}
}

func main() {

	hspec := flag.String("hspec", "", "ISIL PATH pairs")
	showVersion := flag.Bool("v", false, "prints current program version")
	size := flag.Int("b", 20000, "batch size")
	numWorkers := flag.Int("w", runtime.NumCPU(), "number of workers")

	flag.Parse()

	runtime.GOMAXPROCS(*numWorkers)

	if *showVersion {
		fmt.Println(span.AppVersion)
		os.Exit(0)
	}

	opts := options{
		Holdings: make(holdings.IsilIssnHolding),
	}

	if *hspec != "" {
		pathmap, err := span.ParseHoldingSpec(*hspec)
		if err != nil {
			log.Fatal(err)
		}
		for isil, path := range pathmap {
			file, err := os.Open(path)
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close()
			opts.Holdings[isil] = holdings.HoldingsMap(bufio.NewReader(file))
		}
	}

	queue := make(chan []string)
	out := make(chan []byte)
	done := make(chan bool)
	go span.ByteSink(os.Stdout, out, done)

	var wg sync.WaitGroup

	for i := 0; i < *numWorkers; i++ {
		wg.Add(1)
		go worker(queue, out, opts, &wg)
	}

	var batch []string
	var i int

	var readers []io.Reader

	if flag.NArg() == 0 {
		readers = append(readers, os.Stdin)
	} else {
		for _, filename := range flag.Args() {
			file, err := os.Open(filename)
			if err != nil {
				log.Fatal(err)
			}
			defer file.Close()
			readers = append(readers, file)
		}
	}

	for _, r := range readers {
		br := bufio.NewReader(r)
		for {
			line, err := br.ReadString('\n')
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			batch = append(batch, line)
			if i%*size == 0 {
				b := make([]string, len(batch))
				copy(b, batch)
				queue <- b
				batch = batch[:0]
			}
			i++
		}
	}

	b := make([]string, len(batch))
	copy(b, batch)
	queue <- b

	close(queue)
	wg.Wait()
	close(out)
	<-done

}
