package span

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/miku/span/container"
	"github.com/miku/span/finc"
	"github.com/miku/span/holdings"
)

// Filter wraps the decision, whether a given IntermediateSchema record should
// be attached or not. The decision may be based on record properties, simple
// ISSN lists or more involved holdings files.
type Filter interface {
	Apply(finc.IntermediateSchema) bool
}

// Any always returns true.
type Any struct{}

// Apply filter.
func (f Any) Apply(is finc.IntermediateSchema) bool { return true }

// SourceFilter allows to attach ISIL on records of a given source.
type SourceFilter struct {
	SourceID string
}

// Apply filter.
func (f SourceFilter) Apply(is finc.IntermediateSchema) bool {
	return is.SourceID == f.SourceID
}

// MarshalJSON provides custom serialization.
func (f SourceFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.SourceID)
}

// HoldingFilter looks at licensing information from an OVID files. Ref is the
// reference date for moving wall calculations and Table contains a map from
// ISSNs to licenses.
type HoldingFilter struct {
	Ref   time.Time
	Table holdings.Licenses
}

// NewHoldingFilter loads the holdings information for a single institution.
// Returns a single error, if one or more errors have been encountered.
func NewHoldingFilter(r io.Reader) (HoldingFilter, error) {
	licenses, errs := holdings.ParseHoldings(r)
	if len(errs) > 0 {
		for _, e := range errs {
			log.Println(e)
		}
		err := fmt.Errorf("%d errors in holdings file", len(errs))
		return HoldingFilter{Ref: time.Now(), Table: licenses}, err
	}
	return HoldingFilter{Ref: time.Now(), Table: licenses}, nil
}

// MarshalJSON provides custom serialization.
func (f HoldingFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Table)
}

// CoveredAndValid checks coverage and moving wall. If there is no entry for
// an ISSN in the holdings file, we assume, there is no valid license. The
// given signature is a string, that encodes an items location in time.
func (f HoldingFilter) CoveredAndValid(signature, issn string) bool {
	licenses, ok := f.Table[issn]
	if !ok {
		return false
	}
	for _, license := range licenses {
		if !license.Covers(signature) {
			continue
		}
		if f.Ref.After(license.Wall(f.Ref)) {
			return true
		}
	}
	return false
}

// HoldingFilter compares the (year, volume, issue) of an intermediate schema
// record with licensing information, including moving walls.
func (f HoldingFilter) Apply(is finc.IntermediateSchema) bool {
	signature := holdings.CombineDatum(fmt.Sprintf("%d", is.Date.Year()), is.Volume, is.Issue, "")
	for _, issn := range append(is.ISSN, is.EISSN...) {
		if f.CoveredAndValid(signature, issn) {
			return true
		}
	}
	return false
}

// ListFilter will include records, whose ISSN is contained in a given set.
type ListFilter struct {
	Set *container.StringSet
}

// NewAttachByList reads one record per line from reader. Empty lines are ignored.
func NewListFilter(r io.Reader) (ListFilter, error) {
	br := bufio.NewReader(r)
	f := ListFilter{Set: container.NewStringSet()}
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return f, err
		}
		line = strings.TrimSpace(line)
		if line != "" {
			f.Set.Add(line)
		}
	}
	return f, nil
}

// MarshalJSON provides custom serialization.
func (f ListFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Set.Values())
}

// Apply filter.
func (f ListFilter) Apply(is finc.IntermediateSchema) bool {
	for _, issn := range append(is.ISSN, is.EISSN...) {
		if f.Set.Contains(issn) {
			return true
		}
	}
	return false
}

// CollectionFilter allows any record that belongs to a collection, that is
// listed in Names.
type CollectionFilter struct {
	Set *container.StringSet
}

func NewCollectionFilter(r io.Reader) (CollectionFilter, error) {
	br := bufio.NewReader(r)
	f := CollectionFilter{Set: container.NewStringSet()}
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return f, err
		}
		line = strings.TrimSpace(line)
		if line != "" {
			f.Set.Add(line)
		}
	}
	return f, nil
}

// MarshalJSON provides custom serialization.
func (f CollectionFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.Set.Values())
}

// Apply filter.
func (f CollectionFilter) Apply(is finc.IntermediateSchema) bool {
	if f.Set.Contains(is.MegaCollection) {
		return true
	}
	return false
}

// ISILTagger maps ISILs to one or more Filters. If any of these filters
// return true, the ISIL shall be attached (therefore order of the filters
// does not matter).
type ISILTagger map[string][]Filter

// Tags returns all ISILs that could be attached to a given intermediate
// schema record.
func (t ISILTagger) Tags(is finc.IntermediateSchema) []string {
	isils := container.NewStringSet()
	for isil, filters := range t {
		for _, f := range filters {
			if f.Apply(is) {
				isils.Add(isil)
			}
		}
	}
	return isils.Values()
}
