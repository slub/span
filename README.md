Span
====

Span formats.

Godocs: http://godoc.org/github.com/miku/span

Formats
-------

* CrossRef
* Finc
* Google Holdings
* OVID

----

Work-in-progress
----------------

Inputs:

* an input LDJ containing all crossref works metadata (`crossref.Document`)
* a number of XML files, containing holdings information for various institutions

One can transform the documents via `span`:

	span -holdings DE-15 file.xml -h DE-10 other.xml crossref.ldj

Additionally, if one has a cached file of members API responses, one can
use it as input, so the API does not need to be called at all:

	span -holdings DE-15 file.xml DE-10 other.xml -members members.ldj crossref.ldj

The output is an LDJ in `finc.Schema`.
