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
	"archive/zip"
	"bufio"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/miku/span"
	"github.com/miku/span/atomic"
	"github.com/miku/span/formats/finc"
	"github.com/miku/span/licensing"
	"github.com/miku/span/licensing/kbart"
	"github.com/sethgrid/pester"
	log "github.com/sirupsen/logrus"
)

var (
	force       = flag.Bool("f", false, "force all external referenced links to be downloaded")
	dbFile      = flag.String("db", "", "path to an sqlite3 file generated by span-amsl-discovery -db file.db ...")
	cpuprofile  = flag.String("cpuprofile", "", "file to cpu profile")
	showVersion = flag.Bool("v", false, "prints current program version")
	debug       = flag.Bool("debug", false, "only output id and ISIL")

	// counter for cases
	counter = make(map[string]int)
)

// SLUBEZBKBART link to DE-14 KBART, to be included across all sources.
const (
	SLUBEZBKBART         = "https://dbod.de/SLUB-EZB-KBART.zip"
	DE15FIDISSNWHITELIST = "DE15FIDISSNWHITELIST"
)

// ConfigRow decribing a single entry (e.g. an attachment request).
type ConfigRow struct {
	ShardLabel                     string
	ISIL                           string
	SourceID                       string
	TechnicalCollectionID          string
	MegaCollection                 string
	HoldingsFileURI                string
	HoldingsFileLabel              string
	LinkToHoldingsFile             string
	EvaluateHoldingsFileForLibrary string
	ContentFileURI                 string
	ContentFileLabel               string
	LinkToContentFile              string
	ExternalLinkToContentFile      string
	ProductISIL                    string
	DokumentURI                    string
	DokumentLabel                  string
}

// HFCache wraps access to holdings files.
type HFCache struct {
	// entries maps a link or filename (or any identifier) to a map from ISSN
	// to licensing entries.
	entries map[string]map[string][]licensing.Entry
}

// cacheFilename returns the path to the locally cached version of this URL.
func (c *HFCache) cacheFilename(hflink string) string {
	h := sha1.New()
	_, _ = io.WriteString(h, hflink)
	return filepath.Join(xdg.CacheHome, "span", fmt.Sprintf("%x", h.Sum(nil)))
}

// populate fills the entries map from a given URL.
func (c *HFCache) populate(hflink string) error {
	if _, ok := c.entries[hflink]; ok {
		return nil
	}
	var (
		filename = c.cacheFilename(hflink)
		dir      = path.Dir(filename)
	)
	if fi, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	} else if !fi.IsDir() {
		return fmt.Errorf("expected cache directory at: %s", dir)
	}
	// TODO: Accept links and files.
	if _, err := os.Stat(filename); os.IsNotExist(err) || *force {
		if *force {
			log.Printf("redownloading %s", hflink)
		}
		if err := download(hflink, filename); err != nil {
			return err
		}
	}
	var (
		h       = new(kbart.Holdings)
		zr, err = zip.OpenReader(filename)
	)
	if err == nil {
		defer zr.Close()
		for _, f := range zr.File {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			if _, err := h.ReadFrom(rc); err != nil {
				return err
			}
			rc.Close()
		}
	} else {
		f, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := h.ReadFrom(f); err != nil {
			return err
		}
	}
	c.entries[hflink] = h.SerialNumberMap()
	if len(c.entries[hflink]) == 0 {
		log.Printf("warning: %s may not be KBART", hflink)
	} else {
		log.Printf("parsed %s", hflink)
		log.Printf("parsed %d entries from %s (%d)",
			len(c.entries[hflink]), filename, len(c.entries))
	}
	return nil
}

// Covered returns true, if a document is covered by all given kbart files
// (e.g. like "and" filter in former filterconfig). TODO: Merge Covered and
// Covers methods.
func (c *HFCache) Covered(doc *finc.IntermediateSchema, hfs ...string) (ok bool, err error) {
	for _, hf := range hfs {
		if hf == "" {
			continue
		}
		ok, err := c.Covers(hf, doc)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, err
		}
	}
	return true, nil
}

// Covers returns true, if a holdings file, given by link or filename, covers
// the document. The cache takes care of downloading the file, if necessary.
func (c *HFCache) Covers(hflink string, doc *finc.IntermediateSchema) (ok bool, err error) {
	if err = c.populate(hflink); err != nil {
		return false, err
	}
	for _, issn := range append(doc.ISSN, doc.EISSN...) {
		for _, entry := range c.entries[hflink][issn] {
			err = entry.Covers(doc.RawDate, doc.Volume, doc.Issue)
			if err == nil {
				return true, nil
			}
		}
	}
	return false, nil
}

// download retrieves a link and saves its content atomically in filename.
func download(link, filename string) error {
	resp, err := pester.Get(link)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return atomic.WriteFile(filename, b, 0644)
}

// cacheKey returns a key for a document, containing a subset (e.g. sid and
// collcetions) of fields, e.g.  to be used to cache subset of the about 250k
// rows currently in AMSL.
func cacheKey(doc *finc.IntermediateSchema) string {
	v := doc.MegaCollections
	sort.Strings(v)
	return doc.SourceID + "@" + strings.Join(v, "@")
}

// Labeler updates an intermediate schema document.
// We need mostly: ISIL, SourceID, MegaCollection, TechnicalCollectionID, HoldFileURI,
// EvaluateHoldingsFileForLibrary
type Labeler struct {
	dbFile         string
	db             *sqlx.DB
	cache          map[string][]ConfigRow
	hfcache        *HFCache
	whitelistCache map[string]map[string]struct{} // Name (e.g. DE15FIDISSNWHITELIST) -> Set (a set of ISSN)
}

// open opens the database connection, read-only.
func (l *Labeler) open() (err error) {
	if l.db == nil {
		l.db, err = sqlx.Connect("sqlite3", fmt.Sprintf("%s?ro=1", l.dbFile))
	}
	return
}

// matchingRows returns a list of relevant rows for a given document.
func (l *Labeler) matchingRows(doc *finc.IntermediateSchema) (result []ConfigRow, err error) {
	if l.cache == nil {
		l.cache = make(map[string][]ConfigRow)
	}
	key := cacheKey(doc)
	if v, ok := l.cache[key]; ok {
		return v, nil
	}
	if len(doc.MegaCollections) == 0 {
		// TODO: Why zero? Log this to /var/log/span.log or something.
		return result, nil
	}
	// At a minimum, the sid and tcid or collection name must match.
	q, args, err := sqlx.In(`
		SELECT isil, sid, tcid, mc, hflink, hfeval, cflink, cfelink FROM amsl WHERE sid = ? AND (mc IN (?) OR tcid IN (?))
	`, doc.SourceID, doc.MegaCollections, doc.MegaCollections)
	if err != nil {
		return nil, err
	}
	rows, err := l.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cr ConfigRow
		err = rows.Scan(&cr.ISIL,
			&cr.SourceID,
			&cr.TechnicalCollectionID,
			&cr.MegaCollection,
			&cr.LinkToHoldingsFile,
			&cr.EvaluateHoldingsFileForLibrary,
			&cr.ContentFileURI,
			&cr.ExternalLinkToContentFile)
		if err != nil {
			return nil, err
		}
		result = append(result, cr)
	}
	l.cache[key] = result
	return result, nil
}

// Label updates document in place. This may contain hard-coded values for
// special attachment cases.
func (l *Labeler) Labels(doc *finc.IntermediateSchema) ([]string, error) {
	if err := l.open(); err != nil {
		return nil, err
	}
	rows, err := l.matchingRows(doc)
	if err != nil {
		return nil, err
	}
	var labels = make(map[string]struct{}) // ISIL to attach

	// TODO: Distinguish and simplify cases, e.g. with or w/o HF,
	// https://git.io/JvdmC, also log some stats.
	// INFO[12576] lthf => 531,701,113
	// INFO[12576] plain => 112,694,196
	// INFO[12576] 34-music => 3692
	// INFO[12576] 34-DE-15-FID-film => 770
	for _, row := range rows {
		// Fields, where KBART links might be, empty strings are just skipped.
		kbarts := []string{row.LinkToHoldingsFile, row.LinkToContentFile, row.ExternalLinkToContentFile}
		// DE-14 uses a KBART (probably) across all sources, so we hard code
		// their link here. Use `-f` to force download all external files.
		if row.ISIL == "DE-14" {
			kbarts = append(kbarts, SLUBEZBKBART)
		}
		switch {
		case row.ISIL == "DE-15-FID" && strings.Contains(row.LinkToHoldingsFile, "FID_ISSN_Filter"):
			// Here, the holdingfile URL contains a list of ISSN.  URI like ...
			// discovery/metadata-usage/Dokument/FID_ISSN_Filter - but that
			// might change.
			if _, ok := l.whitelistCache[DE15FIDISSNWHITELIST]; !ok {
				// Load from file, once. One value per line.
				f, err := os.Open(row.LinkToHoldingsFile)
				if err != nil {
					return nil, err
				}
				defer f.Close()
				l.whitelistCache[DE15FIDISSNWHITELIST] = make(map[string]struct{})
				if err := setFromLines(f, l.whitelistCache[DE15FIDISSNWHITELIST]); err != nil {
					return nil, err
				}
			}
			whitelist, ok := l.whitelistCache[DE15FIDISSNWHITELIST]
			if !ok {
				return nil, fmt.Errorf("whitelist cache broken")
			}
			for _, issn := range doc.ISSNList() {
				if _, ok := whitelist[issn]; ok {
					labels[row.ISIL] = struct{}{}
					counter["de-15-fid-issn-whitelist"]++
				}
			}
		case row.EvaluateHoldingsFileForLibrary == "yes" && row.LinkToHoldingsFile != "" && row.LinkToContentFile != "":
			// Both, holding and content file need to match (AND).
			ok, err := l.hfcache.Covered(doc, kbarts...)
			if err != nil {
				return nil, err
			}
			if ok {
				labels[row.ISIL] = struct{}{}
				counter["lthf+ltcf"]++
			}
		case row.EvaluateHoldingsFileForLibrary == "yes" && row.LinkToHoldingsFile != "" && row.ExternalLinkToContentFile != "":
			// Both, holding and content file need to match (AND).
			ok, err := l.hfcache.Covered(doc, kbarts...)
			if err != nil {
				return nil, err
			}
			if ok {
				labels[row.ISIL] = struct{}{}
				counter["lthf+eltcf"]++
			}
		case row.EvaluateHoldingsFileForLibrary == "yes" && row.LinkToHoldingsFile != "" && row.LinkToContentFile == "" && row.ExternalLinkToContentFile == "":
			// Both, holding and content file need to match (AND).
			ok, err := l.hfcache.Covered(doc, kbarts...)
			if err != nil {
				return nil, err
			}
			if ok {
				labels[row.ISIL] = struct{}{}
				counter["lthf"]++
			}
			// TODO: add case, where we limit by both holding and content file.
		case doc.SourceID == "34":
			switch {
			case stringsContain([]string{"DE-L152", "DE-1156", "DE-1972", "DE-Kn38"}, row.ISIL):
				// refs #10495, a subject filter for a few hard-coded ISIL; https://git.io/JvFjE
				if stringsOverlap(doc.Subjects, []string{"Music", "Music education"}) {
					labels[row.ISIL] = struct{}{}
					counter["34-music"]++
				}
			case row.ISIL == "DE-15-FID":
				// refs #10495, maybe use a TSV with custom column name to use a subject list? https://git.io/JvFjd
				if stringsOverlap(doc.Subjects, []string{"Film studies", "Information science", "Mass communication"}) {
					labels[row.ISIL] = struct{}{}
					counter["34-DE-15-FID-film"]++
				}
			}
		case row.EvaluateHoldingsFileForLibrary == "yes" && row.LinkToHoldingsFile == "":
			return nil, fmt.Errorf("no holding file to evaluate: %v", row)
		case row.EvaluateHoldingsFileForLibrary == "no" && row.LinkToHoldingsFile != "":
			return nil, fmt.Errorf("config provides holding file, but does not want to evaluate it: %v", row)
		case row.ExternalLinkToContentFile != "":
			// https://git.io/JvFjx
			ok, err := l.hfcache.Covered(doc, kbarts...)
			if err != nil {
				return nil, err
			}
			if ok {
				labels[row.ISIL] = struct{}{}
				counter["eltcf"]++
			}
		case row.LinkToContentFile != "":
			// https://git.io/JvFjp
			ok, err := l.hfcache.Covered(doc, kbarts...)
			if err != nil {
				return nil, err
			}
			if ok {
				labels[row.ISIL] = struct{}{}
				counter["ltcf"]++
			}
		case row.EvaluateHoldingsFileForLibrary == "no":
			labels[row.ISIL] = struct{}{}
			counter["plain"]++
		case row.ContentFileURI != "":
			counter["todo-cf"]++
		default:
			return nil, fmt.Errorf("none of the attachment modes match for %v", doc)
		}
	}
	var keys []string
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

// setFromLines populates a set from lines in a reader.
func setFromLines(r io.Reader, m map[string]struct{}) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		m[line] = struct{}{}
	}
	return nil
}

// stringsSliceContains returns true, if value appears in a string slice.
func stringsContain(ss []string, v string) bool {
	for _, w := range ss {
		if v == w {
			return true
		}
	}
	return false
}

// stringsOverlap returns true, if at least one value is in both ss and vv.
// Inefficient.
func stringsOverlap(ss, vv []string) bool {
	for _, s := range ss {
		for _, v := range vv {
			if s == v {
				return true
			}
		}
	}
	return false
}

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
	var (
		labeler = &Labeler{
			dbFile: *dbFile,
			hfcache: &HFCache{
				// link -> issn -> rows (kbart)
				entries: make(map[string]map[string][]licensing.Entry),
			},
		}
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
	for name, count := range counter {
		log.Printf("%s => %d", name, count)
	}
}
