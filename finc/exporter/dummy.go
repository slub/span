//  Copyright 2015 by Leipzig University Library, http://ub.uni-leipzig.de
//                    The Finc Authors, http://finc.info
//                    Martin Czygan, <martin.czygan@uni-leipzig.de>
//
// This file is part of some open source application.
//
// Some open source application is free software: you can redistribute
// it and/or modify it under the terms of the GNU General Public
// License as published by the Free Software Foundation, either
// version 3 of the License, or (at your option) any later version.
//
// Some open source application is distributed in the hope that it will
// be useful, but WITHOUT ANY WARRANTY; without even the implied warranty
// of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with Foobar.  If not, see <http://www.gnu.org/licenses/>.
//
// @license GPL-3.0+ <http://spdx.org/licenses/GPL-3.0+>
//
package exporter

import (
	"fmt"

	"github.com/miku/span/finc"
)

// DummySchema is an example export schema, that only has one field.
type DummySchema struct {
	Title string `json:"title"`
}

// Attach is here, so it satisfies the interface, but implementation is a noop.
func (d *DummySchema) Attach(s []string) {}

// Export converts intermediate schema into this export schema.
func (d *DummySchema) Convert(is finc.IntermediateSchema) error {
	d.Title = fmt.Sprintf("%s (%s)", is.ArticleTitle, is.JournalTitle)
	return nil
}
