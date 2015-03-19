// Package finc holds finc SolrSchema (SOLR) and intermediate schema related types and methods.
package finc

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kennygrant/sanitize"
	"github.com/miku/span/holdings"
	"github.com/miku/span/sets"
)

const (
	AIRecordType              = "ai"
	AIAccessFacet             = "Electronic Resources"
	IntermediateSchemaVersion = "0.9"
)

var (
	errFromYear        = errors.New("from-year mismatch")
	errFromVolume      = errors.New("from-volume mismatch")
	errFromIssue       = errors.New("from-issue mismatch")
	errToYear          = errors.New("to-year mismatch")
	errToVolume        = errors.New("to-volume mismatch")
	errToIssue         = errors.New("to-issue mismatch")
	errMovingWall      = errors.New("moving-wall violation")
	errUnparsableValue = errors.New("value not parsable")
)

// Author representes an author, "inspired" by OpenURL.
type Author struct {
	ID           string `json:"x.id,omitempty"`
	Name         string `json:"rft.au,omitempty"`
	LastName     string `json:"rft.aulast,omitempty"`
	FirstName    string `json:"rft.aufirst,omitempty"`
	Initial      string `json:"rft.auinit,omitempty"`
	FirstInitial string `json:"rft.auinit1,omitempty"`
	MiddleName   string `json:"rft.auinitm,omitempty"`
	Suffix       string `json:"rft.ausuffix,omitempty"`
	Corporation  string `json:"rft.aucorp,omitempty"`
}

// String returns a formatted author string.
// TODO(miku): make this complete.
func (author *Author) String() string {
	if author.LastName != "" {
		if author.FirstName != "" {
			return fmt.Sprintf("%s, %s", author.LastName, author.FirstName)
		}
		return author.LastName
	}
	return author.ID
}

// IntermediateSchema abstract and collects the values of various input formats.
// Goal is to simplify further processing by using a single format, from which
// the next artifacts can be derived, e.g. records for solr indices.
// This format can be viewed as a catch-all format. The dotted notation
// hints at the origin of the field, e.g. OpenURL, RIS, finc.
//
// Notes on the format:
// - The x namespace is experimental.
// - RawDate must be in ISO8601 format.
// - Version is mandatory.
// - Headings and Subjects are not bound to any format yet.
// - Use plural for slices, if possible.
type IntermediateSchema struct {
	Format         string `json:"finc.format,omitempty"`
	MegaCollection string `json:"finc.mega_collection,omitempty"`
	RecordID       string `json:"finc.record_id,omitempty"`
	SourceID       string `json:"finc.source_id,omitempty"`

	Database     string `json:"ris.db,omitempty"`
	DataProvider string `json:"ris.dp,omitempty"`
	RefType      string `json:"ris.type,omitempty"`

	ArticleNumber string   `json:"rft.artnum,omitempty"`
	ArticleTitle  string   `json:"rft.atitle,omitempty"`
	BookTitle     string   `json:"rft.btitle,omitempty"`
	Chronology    string   `json:"rft.chron,omitempty"`
	Edition       string   `json:"rft.edition,omitempty"`
	EISBN         []string `json:"rft.isbn,omitempty"`
	EISSN         []string `json:"rft.eissn,omitempty"`
	EndPage       string   `json:"rft.epage,omitempty"`
	Genre         string   `json:"rft.genre,omitempty"`
	ISBN          []string `json:"rft.isbn,omitempty"`
	ISSN          []string `json:"rft.issn,omitempty"`
	Issue         string   `json:"rft.issue,omitempty"`
	JournalTitle  string   `json:"rft.jtitle,omitempty"`
	PageCount     string   `json:"rft.tpages,omitempty"`
	Pages         string   `json:"rft.pages,omitempty"`
	Part          string   `json:"rft.part,omitempty"`
	Places        []string `json:"rft.place,omitempty"`
	Publishers    []string `json:"rft.pub,omitempty"`
	Quarter       string   `json:"rft.quarter,omitempty"`
	RawDate       string   `json:"rft.date,omitempty"`
	Season        string   `json:"rft.ssn,omitempty"`
	Series        string   `json:"rft.series,omitempty"`
	ShortTitle    string   `json:"rft.stitle,omitempty"`
	StartPage     string   `json:"rft.spage,omitempty"`
	Volume        string   `json:"rft.volume,omitempty"`

	Abstract  string   `json:"abstract,omitempty"`
	Authors   []Author `json:"authors,omitempty"`
	DOI       string   `json:"doi,omitempty"`
	Languages []string `json:"languages,omitempty"`
	URL       []string `json:"url,omitempty"`
	Version   string   `json:"version,omitempty"`

	ArticleSubtitle string   `json:"x.subtitle,omitempty"`
	Fulltext        string   `json:"x.fulltext,omitempty"`
	Headings        []string `json:"x.headings,omitempty"`
	Subjects        []string `json:"x.subjects,omitempty"`
	Type            string   `json:"x.type,omitempty"`
}

func NewIntermediateSchema() *IntermediateSchema {
	return &IntermediateSchema{Version: IntermediateSchemaVersion}
}

// Date returns the publication or issuing date of this record.
// Fails on non ISO-8601 compliant dates.
func (is *IntermediateSchema) Date() (time.Time, error) {
	t, err := time.Parse("2006-01-02", is.RawDate)
	if err != nil {
		return t, fmt.Errorf("invalid date: %s", is.RawDate)
	}
	return t, nil
}

// ISSNList returns a deduplicated list of all ISSN and EISSN.
func (is *IntermediateSchema) ISSNList() []string {
	set := make(map[string]struct{})
	for _, issn := range append(is.ISSN, is.EISSN...) {
		set[issn] = struct{}{}
	}
	var issns []string
	for k := range set {
		issns = append(issns, k)
	}
	return issns
}

// Allfields returns a combination of various fields.
func (is *IntermediateSchema) Allfields() (string, error) {
	var authors []string
	for _, author := range is.Authors {
		authors = append(authors, author.String())
	}
	fields := [][]string{authors,
		is.Subjects, is.ISSN, is.EISSN, is.Publishers, is.Places, is.URL,
		[]string{is.ArticleTitle, is.ArticleSubtitle, is.JournalTitle, is.Fulltext}}
	var buf bytes.Buffer
	for _, f := range fields {
		for _, value := range f {
			for _, token := range strings.Fields(value) {
				_, err := buf.WriteString(fmt.Sprintf("%s ", strings.TrimSpace(token)))
				if err != nil {
					return "", err
				}
			}
		}
	}
	return strings.TrimSpace(buf.String()), nil
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Imprint MARC 260 a, b, c (trad.)
func (is *IntermediateSchema) Imprint() (s string) {
	var places, publisher string
	date, _ := is.Date()
	year := date.Year()
	places = strings.Join(is.Places, " ; ")
	if len(is.Publishers) > 0 {
		publisher = is.Publishers[0]
	}

	mask := btoi(places != "") + 2*btoi(publisher != "") + 4*btoi(year != 0)

	switch mask {
	case 1:
		s = places
	case 2:
		s = publisher
	case 3:
		s = fmt.Sprintf("%s : %s", places, publisher)
	case 4:
		s = fmt.Sprintf("%d", year)
	case 5:
		s = fmt.Sprintf("%s : %d", places, year)
	case 6:
		s = fmt.Sprintf("%s, %d", publisher, year)
	case 7:
		s = fmt.Sprintf("%s : %s, %d", places, publisher, year)
	}
	return
}

// CoveredBy returns nil, if a given entitlement covers the current document.
// If the given entitlement does not cover the document, the error returned
// will contain a reason.
func (is *IntermediateSchema) CoveredBy(e holdings.Entitlement) error {
	date, err := is.Date()
	if err != nil {
		return err
	}
	if e.FromYear != 0 && date.Year() != 0 {
		if e.FromYear > date.Year() {
			return errFromYear
		}
		if e.FromYear == date.Year() {
			volume, err := strconv.Atoi(is.Volume)
			if err != nil {
				return errUnparsableValue
			}
			if e.FromVolume != 0 && e.FromVolume > volume {
				return errFromVolume
			}
			if e.FromVolume == volume {
				issue, err := strconv.Atoi(is.Issue)
				if err != nil {
					return errUnparsableValue
				}
				if e.FromIssue != 0 && e.FromIssue > issue {
					return errFromIssue
				}
			}
		}
	}

	if e.ToYear != 0 && date.Year() != 0 {
		if e.ToYear < date.Year() {
			return errToYear
		}
		if e.ToYear == date.Year() {
			volume, err := strconv.Atoi(is.Volume)
			if err != nil {
				return errUnparsableValue
			}
			if e.ToVolume != 0 && e.ToVolume < volume {
				return errToVolume
			}
			if e.ToVolume == volume {
				issue, err := strconv.Atoi(is.Issue)
				if err != nil {
					return errUnparsableValue
				}
				if e.ToIssue != 0 && e.ToIssue < issue {
					return errToIssue
				}
			}
		}
	}

	boundary, err := e.Boundary()
	if err != nil {
		return err
	}
	if date.After(boundary) {
		return errMovingWall
	}
	return nil
}

// Institutions returns a slice of ISILs for which this document finds
// valid entitlements in a IsilIssnHolding map.
func (is *IntermediateSchema) Institutions(iih holdings.IsilIssnHolding) []string {
	isils := sets.NewStringSet()
	for _, isil := range iih.Isils() {
		for _, issn := range is.ISSNList() {
			h, exists := iih[isil][issn]
			if !exists {
				continue
			}
			for _, entitlement := range h.Entitlements {
				err := is.CoveredBy(entitlement)
				if err != nil {
					continue
				}
				isils.Add(isil)
				break
			}
		}
	}
	values := isils.Values()
	sort.Strings(values)
	return values
}

// ToSolrSchema converts an intermediate Schema to a finc.Solr schema.
// Note that this method can and will include all kinds of source
// specific alterations, which are not expressed in the intermediate format.
func (is *IntermediateSchema) ToSolrSchema(iih holdings.IsilIssnHolding) (*SolrSchema, error) {
	output := new(SolrSchema)

	date, err := is.Date()
	if err != nil {
		return output, err
	}

	all, err := is.Allfields()
	if err != nil {
		return output, err
	}

	output.Allfields = all
	output.Formats = append(output.Formats, is.Format)
	output.Fullrecord = "blob:" + is.RecordID
	output.Fulltext = is.Fulltext
	output.HierarchyParentTitle = is.JournalTitle
	output.ID = is.RecordID
	output.ISSN = is.ISSNList()
	output.Imprint = is.Imprint()
	output.MegaCollections = append(output.MegaCollections, is.MegaCollection)
	output.PublishDateSort = date.Year()
	output.Publishers = is.Publishers
	output.RecordType = AIRecordType
	output.SourceID = is.SourceID
	output.Topics = is.Subjects
	output.URL = is.URL

	sanitized := sanitize.HTML(is.ArticleTitle)
	output.Title = sanitized
	output.TitleFull = sanitized
	output.TitleShort = sanitized

	for _, lang := range is.Languages {
		output.Languages = append(output.Languages, ReferenceName(lang))
	}

	for _, author := range is.Authors {
		output.SecondaryAuthors = append(output.SecondaryAuthors, author.String())
	}

	if len(output.SecondaryAuthors) > 0 {
		output.Author = output.SecondaryAuthors[0]
	}

	// source and finc specific alterations
	// TODO(miku): reuse some mapping files if necessary

	output.AccessFacet = AIAccessFacet
	switch is.SourceID {
	case "49":
		output.Institutions = is.Institutions(iih)
	case "50":
		output.Institutions = []string{"DE-15"}
	}
	return output, nil
}
