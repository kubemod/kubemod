package util

import (
	"fmt"
	"regexp"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
)

var (
	headerRex  *regexp.Regexp = regexp.MustCompile(`(---|\+\+\+) (a|b)\n`)
	emptyBytes []byte         = []byte{}
)

func Diff(data1, data2 []byte) ([]byte, error) {
	a, b := string(data1), string(data2)
	edits := myers.ComputeEdits(span.URIFromPath(""), a, b)
	diff := fmt.Sprint(gotextdiff.ToUnified("a", "b", a, edits))
	return headerRex.ReplaceAll([]byte(diff), emptyBytes), nil
}
