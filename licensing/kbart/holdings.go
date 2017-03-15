// Package kbart implements support for KBART (Knowledge Bases And Related Tools
// working group, http://www.uksg.org/kbart/) holding files
// (http://www.uksg.org/kbart/s5/guidelines/data_format).
//
// > This is a generic format that minimizes the effort involved in receiving and
// loading the data, and reduces the likelihood of errors being introduced during
// exchange. Tab-delimited formats are preferable to comma-separated formats, as
// commas appear regularly within the distributed data and, though they can be
// "commented out", doing so leaves a greater opportunity for error than the use
// of a tab-delimited format. Tab-delimited formats can be easily exported from
// all commonly used spreadsheet programs.
package kbart

import (
	"io"

	"github.com/miku/span"
	"github.com/miku/span/encoding/tsv"
	"github.com/miku/span/licensing"
)

// Holdings contains a list of entries about licenced or available content. In
// addition to access to all entries, this type exposes a couple of helper
// methods.
type Holdings struct {
	Entries []licensing.Entry
	cache   map[string][]licensing.Entry // Cache lookups by ISSN.
}

// ReadFrom create holdings struct from a reader. Expects a tab separated CSV with
// a single header line.
func (h *Holdings) ReadFrom(r io.Reader) (int64, error) {
	var wc span.WriteCounter
	dec := tsv.NewDecoder(io.TeeReader(r, &wc))
	for {
		var entry licensing.Entry
		err := dec.Decode(&entry)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		h.Entries = append(h.Entries, entry)
	}
	h.buildLookupCache()
	return int64(wc.Count()), nil
}

// buildLookupCache for ISSN lookups.
func (h *Holdings) buildLookupCache() {
	h.cache = make(map[string][]licensing.Entry)
	for _, e := range h.Entries {
		for _, issn := range e.ISSNList() {
			h.cache[issn] = append(h.cache[issn], e)
		}
	}
}

// ByISSN returns all licensing entries for given ISSN.
func (h *Holdings) ByISSN(issn string) (entries []licensing.Entry) {
	entries, _ = h.cache[issn]
	return
}
