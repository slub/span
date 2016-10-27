package formeta

import (
	"testing"
	"time"
)

func TestEncoding(t *testing.T) {
	var cases = []struct {
		in  interface{}
		out string
		err error
	}{
		{in: "", out: "", err: nil},
		{in: "x", out: "", err: ErrValueNotAllowed},
		{in: struct{ A string }{A: "B"}, out: `{ A: 'B',  }`, err: nil},
		{in: struct{ A string }{A: "B 'A"}, out: `{ A: 'B \'A',  }`, err: nil},
		{in: struct{ A []string }{A: []string{"B", "C"}}, out: `{ A: 'B', A: 'C',  }`, err: nil},
		{in: struct{ A int }{A: 1}, out: `{ A: 1,  }`, err: nil},
		{in: struct{ A int64 }{A: 1}, out: `{ A: 1,  }`, err: nil},
		{
			in: struct{ A string }{A: `B
A`}, out: `{ A: 'B\nA',  }`, err: nil,
		},
		{
			in: struct{ A string }{A: `B\ A`}, out: `{ A: 'B\\ A',  }`, err: nil,
		},
		{
			in: struct{ A string }{A: `B\
'A \`}, out: `{ A: 'B\\\n\'A \\',  }`, err: nil,
		},
	}

	for _, c := range cases {
		b, err := Marshal(c.in)
		if err != c.err {
			t.Errorf("Marshal got %v, want %v", err, c.err)
		}
		if string(b) != c.out {
			t.Errorf("Marshal got %v, want %v", string(b), c.out)
		}
	}
}

type TestPosition struct {
	Longitude float64
	Latitude  float64
}

type TestPeak struct {
	Name     string
	Location TestPosition
	Ascent   time.Time
	Variants []string
	Camps    []TestPosition
}

func TestNested(t *testing.T) {
	p := TestPeak{
		Name: "пик Сталина",
		Location: TestPosition{
			38.916667, 72.016667,
		},
		Variants: []string{
			"Ismoil Somoni Peak",
			"Қуллаи Исмоили Сомонӣ",
		},
		Camps: []TestPosition{
			{38.916667, 72.016667},
			{38.916667, 72.016667},
			{38.916667, 72.016667},
		},
	}

	want := `{ Name: 'пик Сталина', Location { Longitude: 38.916667, Latitude: 72.016667,  } Ascent: '0001-01-01T00:00:00Z', Variants: 'Ismoil Somoni Peak', Variants: 'Қуллаи Исмоили Сомонӣ', Camps { Longitude: 38.916667, Latitude: 72.016667,  } Camps { Longitude: 38.916667, Latitude: 72.016667,  } Camps { Longitude: 38.916667, Latitude: 72.016667,  }  }`

	b, err := Marshal(p)
	if err != nil {
		t.Errorf(err.Error())
	}
	if string(b) != want {
		t.Errorf("Marshal got %v, want %v", string(b), want)
	}
}
