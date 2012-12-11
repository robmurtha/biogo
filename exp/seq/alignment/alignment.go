// Copyright ©2011-2012 Dan Kortschak <dan.kortschak@adelaide.edu.au>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package alignment handles aligned sequences stored as columns.
package alignment

import (
	"code.google.com/p/biogo/exp/alphabet"
	"code.google.com/p/biogo/exp/feat"
	"code.google.com/p/biogo/exp/seq"
	"code.google.com/p/biogo/exp/seq/linear"
	"code.google.com/p/biogo/util"
	"errors"
	"fmt"
)

// A Seq is an aligned sequence.
type Seq struct {
	seq.Annotation
	SubAnnotations []seq.Annotation
	Seq            alphabet.Columns
	ColumnConsense seq.ConsenseFunc
}

// NewSeq creates a new Seq with the given id, letter sequence and alphabet.
func NewSeq(id string, subids []string, b [][]alphabet.Letter, alpha alphabet.Alphabet, cons seq.ConsenseFunc) (*Seq, error) {
	var (
		lids, lseq = len(subids), len(b)
		subann     []seq.Annotation
	)
	switch {
	case lids == 0 && len(b) == 0:
	case lseq != 0 && lids == len(b[0]):
		if lids == 0 {
			subann = make([]seq.Annotation, len(b[0]))
			for i := range subids {
				subann[i].ID = fmt.Sprintf("%s:%d", id, i)
			}
		} else {
			subann = make([]seq.Annotation, lids)
			for i, sid := range subids {
				subann[i].ID = sid
			}
		}
	default:
		return nil, errors.New("alignment: id/seq number mismatch")
	}

	return &Seq{
		Annotation: seq.Annotation{
			ID:    id,
			Alpha: alpha,
		},
		SubAnnotations: subann,
		Seq:            append([][]alphabet.Letter(nil), b...),
		ColumnConsense: cons,
	}, nil
}

// Interface guarantees
var (
	_ feat.Feature = &Seq{}
	_ feat.Feature = Row{}
	_ seq.Sequence = Row{}
)

// Slice returns the sequence data as a alphabet.Slice.
func (s *Seq) Slice() alphabet.Slice { return s.Seq }

// SetSlice sets the sequence data represented by the Seq. SetSlice will panic if sl
// is not a Columns.
func (s *Seq) SetSlice(sl alphabet.Slice) { s.Seq = sl.(alphabet.Columns) }

// Len returns the length of the alignment.
func (s *Seq) Len() int { return len(s.Seq) }

// Rows returns the number of rows in the alignment.
func (s *Seq) Rows() int { return s.Seq.Rows() }

// Start returns the start position of the sequence in global coordinates.
func (s *Seq) Start() int { return s.Offset }

// End returns the end position of the sequence in global coordinates.
func (s *Seq) End() int { return s.Offset + s.Len() }

// Copy returns a copy of the sequence.
func (s *Seq) Copy() seq.Rower {
	c := *s
	c.Seq = make(alphabet.Columns, len(s.Seq))
	for i, cs := range s.Seq {
		c.Seq[i] = append([]alphabet.Letter(nil), cs...)
	}

	return &c
}

// New returns an empty *Seq sequence.
func (s *Seq) New() *Seq {
	return &Seq{}
}

// RevComp reverse complements the sequence. RevComp will panic if the alphabet used by
// the receiver is not a Complementor.
func (s *Seq) RevComp() {
	rs, comp := s.Seq, s.Alpha.(alphabet.Complementor).ComplementTable()
	i, j := 0, len(rs)-1
	for ; i < j; i, j = i+1, j-1 {
		for r := range rs[i] {
			rs[i][r], rs[j][r] = comp[rs[j][r]], comp[rs[i][r]]
		}
	}
	if i == j {
		for r := range rs[i] {
			rs[i][r] = comp[rs[i][r]]
		}
	}
	s.Strand = -s.Strand
}

// Reverse reverses the order of letters in the the sequence without complementing them.
func (s *Seq) Reverse() {
	l := s.Seq
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i], l[j] = l[j], l[i]
	}
	s.Strand = seq.None
}

func (s *Seq) String() string {
	return s.Consensus(false).String()
}

// Add adds the sequences n to Seq. Sequences in n should align start and end with the receiving alignment.
// Additional sequence will be clipped and missing sequence will be filled with the gap letter.
func (s *Seq) Add(n ...seq.Sequence) error {
	for i := s.Start(); i < s.End(); i++ {
		s.Seq[i] = append(s.Seq[i], s.column(n, i)...)
	}
	for i := range n {
		s.SubAnnotations = append(s.SubAnnotations, *n[i].CopyAnnotation())
	}

	return nil
}

func (s *Seq) column(m []seq.Sequence, pos int) []alphabet.Letter {
	c := make([]alphabet.Letter, 0, s.Rows())

	for _, ss := range m {
		if a, ok := ss.(seq.Aligned); ok {
			if a.Start() <= pos && pos < a.End() {
				c = append(c, a.Column(pos, true)...)
			} else {
				c = append(c, s.Alpha.Gap().Repeat(a.Rows())...)
			}
		} else {
			if ss.Start() <= pos && pos < ss.End() {
				c = append(c, ss.At(pos).L)
			} else {
				c = append(c, s.Alpha.Gap())
			}
		}
	}

	return c
}

// TODO
func (s *Seq) Delete(i int) {}

func (s *Seq) Row(i int) seq.Sequence {
	return Row{Align: s, Row: i}
}

// AppendColumns appends each Qletter of each element of a to the appropriate sequence in the reciever.
func (s *Seq) AppendColumns(a ...[]alphabet.QLetter) error {
	for i, r := range a {
		if len(r) != s.Rows() {
			return fmt.Errorf("alignment: column %d does not match Rows(): %d != %d.", i, len(r), s.Rows())
		}
	}

	s.Seq = append(s.Seq, make([][]alphabet.Letter, len(a))...)[:len(s.Seq)]
	for _, r := range a {
		c := make([]alphabet.Letter, len(r))
		for i := range r {
			c[i] = r[i].L
		}
		s.Seq = append(s.Seq, c)
	}

	return nil
}

// AppendEach appends each []alphabet.QLetter in a to the appropriate sequence in the receiver.
func (s *Seq) AppendEach(a [][]alphabet.QLetter) error {
	if len(a) != s.Rows() {
		return fmt.Errorf("alignment: number of sequences does not match Rows(): %d != %d.", len(a), s.Rows())
	}
	max := util.MinInt
	for _, ss := range a {
		if l := len(ss); l > max {
			max = l
		}
	}
	s.Seq = append(s.Seq, make([][]alphabet.Letter, max)...)[:len(s.Seq)]
	for i, b := 0, make([]alphabet.QLetter, 0, len(a)); i < max; i, b = i+1, b[:0] {
		for _, ss := range a {
			if i < len(ss) {
				b = append(b, ss[i])
			} else {
				b = append(b, alphabet.QLetter{L: s.Alpha.Gap()})
			}
		}
		s.AppendColumns(b)
	}

	return nil
}

// Column returns a slice of letters reflecting the column at pos.
func (s *Seq) Column(pos int, _ bool) []alphabet.Letter {
	return s.Seq[pos]
}

// ColumnQL returns a slice of quality letters reflecting the column at pos.
func (s *Seq) ColumnQL(pos int, _ bool) []alphabet.QLetter {
	c := make([]alphabet.QLetter, s.Rows())
	for i, l := range s.Seq[pos] {
		c[i] = alphabet.QLetter{
			L: l,
			Q: seq.DefaultQphred,
		}
	}

	return c
}

// Consensus returns a quality sequence reflecting the consensus of the receiver determined by the
// ColumnConsense field.
func (s *Seq) Consensus(_ bool) *linear.QSeq {
	cs := make([]alphabet.QLetter, 0, s.Len())
	alpha := s.Alphabet()
	for i := range s.Seq {
		cs = append(cs, s.ColumnConsense(s, alpha, i, false))
	}

	qs := linear.NewQSeq("Consensus:"+s.ID, cs, s.Alpha, alphabet.Sanger)
	qs.Strand = s.Strand
	qs.SetOffset(s.Offset)
	qs.Conform = s.Conform

	return qs
}

// A Row is a pointer into an alignment that satifies the seq.Sequence interface.
type Row struct {
	Align *Seq
	Row   int
}

// At returns the letter at position pos.
func (r Row) At(i int) alphabet.QLetter {
	return alphabet.QLetter{
		L: r.Align.Seq[i-r.Align.Offset][r.Row],
		Q: seq.DefaultQphred,
	}
}

// Set sets the letter at position pos to l.
func (r Row) Set(i int, l alphabet.QLetter) {
	r.Align.Seq[i-r.Align.Offset][r.Row] = l.L
}

// Len returns the length of the row.
func (r Row) Len() int { return len(r.Align.Seq) }

// Start returns the start position of the sequence in global coordinates.
func (r Row) Start() int { return r.Align.SubAnnotations[r.Row].Offset }

// End returns the end position of the sequence in global coordinates.
func (r Row) End() int { return r.Start() + r.Len() }

// Location returns the feature containing the row's sequence.
func (r Row) Location() feat.Feature { return r.Align.SubAnnotations[r.Row].Loc }

func (r Row) Alphabet() alphabet.Alphabet         { return r.Align.Alpha }
func (r Row) Conformation() feat.Conformation     { return r.Align.Conform }
func (r Row) SetConformation(c feat.Conformation) { r.Align.SubAnnotations[r.Row].Conform = c }
func (r Row) Name() string                        { return r.Align.SubAnnotations[r.Row].ID }
func (r Row) Description() string                 { return r.Align.SubAnnotations[r.Row].Desc }
func (r Row) SetOffset(o int)                     { r.Align.SubAnnotations[r.Row].Offset = o }

func (r Row) RevComp() {
	rs, comp := r.Align.Seq, r.Alphabet().(alphabet.Complementor).ComplementTable()
	i, j := 0, len(rs)-1
	for ; i < j; i, j = i+1, j-1 {
		rs[i][r.Row], rs[j][r.Row] = comp[rs[j][r.Row]], comp[rs[i][r.Row]]
	}
	if i == j {
		rs[i][r.Row] = comp[rs[i][r.Row]]
	}
	r.Align.SubAnnotations[r.Row].Strand = -r.Align.SubAnnotations[r.Row].Strand
}
func (r Row) Reverse() {
	l := r.Align.Seq
	for i, j := 0, len(l)-1; i < j; i, j = i+1, j-1 {
		l[i][r.Row], l[j][r.Row] = l[j][r.Row], l[i][r.Row]
	}
	r.Align.SubAnnotations[r.Row].Strand = seq.None
}
func (r Row) New() seq.Sequence { return Row{} }
func (r Row) Copy() seq.Sequence {
	b := make([]alphabet.Letter, r.Len())
	for i, c := range r.Align.Seq {
		b[i] = c[r.Row]
	}
	return linear.NewSeq(r.Name(), b, r.Alphabet())
}
func (r Row) CopyAnnotation() *seq.Annotation { return r.Align.SubAnnotations[r.Row].CopyAnnotation() }

// SetSlice uncoditionally panics.
func (r Row) SetSlice(_ alphabet.Slice) { panic("alignment: cannot alter row slice") }

// Slice uncoditionally panics.
func (r Row) Slice() alphabet.Slice { panic("alignment: cannot get row slice") }