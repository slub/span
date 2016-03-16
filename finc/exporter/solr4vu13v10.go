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

import "github.com/miku/span/finc"

// Solr4vu13v6 add missing publishDate field (#6266)
type Solr4Vufind13v10 struct {
	Solr4Vufind13v7
}

// Attach attaches the ISILs to a record.
func (s *Solr4Vufind13v10) Attach(isils []string) {
	s.Institutions = isils
}

// Export method from intermediate schema to solr 4/13 schema.
func (s *Solr4Vufind13v10) Convert(is finc.IntermediateSchema) error {
	if err := s.Solr4Vufind13v7.Convert(is); err != nil {
		return err
	}
	s.Institutions = is.Labels
	return nil
}
