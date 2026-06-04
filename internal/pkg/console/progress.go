package console

import (
	"fmt"
	"io"
	"strings"
)

// textProgress is a minimal carriage-return progress bar written to out.
type textProgress struct {
	out     io.Writer
	total   int
	current int
}

func (p *textProgress) Advance(step ...int) {
	n := 1
	if len(step) > 0 {
		n = step[0]
	}
	p.current += n
	if p.current > p.total {
		p.current = p.total
	}
	p.render()
}

func (p *textProgress) Finish() {
	p.current = p.total
	p.render()
	fmt.Fprintln(p.out)
}

func (p *textProgress) render() {
	const width = 30
	ratio := 0.0
	if p.total > 0 {
		ratio = float64(p.current) / float64(p.total)
	}
	filled := int(ratio * width)
	bar := strings.Repeat("=", filled) + strings.Repeat(" ", width-filled)
	fmt.Fprintf(p.out, "\r[%s] %d/%d", bar, p.current, p.total)
}
