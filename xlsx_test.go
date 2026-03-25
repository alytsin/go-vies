package vies

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestName(t *testing.T) {

	f, err := os.Open("test.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}

	r := SpreadsheetMlReader{}
	data, err := r.Handle(&content)
	t.Log(err)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(data)

}

func TestBase26(t *testing.T) {
	r := SpreadsheetMlReader{}

	cases := map[string]int{
		"":   -1,
		"&":  -1,
		"A":  1,
		"B":  2,
		"C":  3,
		"D":  4,
		"E":  5,
		"F":  6,
		"G":  7,
		"H":  8,
		"I":  9,
		"J":  10,
		"K":  11,
		"Z":  26,
		"AA": 26*1 + 1,
		"AY": 26*1 + 25,
		"AZ": 26*1 + 26,
		"BA": 26*2 + 1,
		"CA": 26*3 + 1,
		"ZZ": 26*26 + 26,
	}

	for c, d := range cases {
		t.Run(c, func(t *testing.T) {
			assert.Equal(t, d, r.decodeOneBasedBase26(c))
		})
	}

}
