// Copyright 2015-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package date prints the current time or a file's modification time.
//
// This implementation is adapted from u-root's cmds/core/date command. Wanix
// intentionally omits setting the system clock because browser-hosted Wasm has
// no system clock to mutate.
package date

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/u-root/u-root/pkg/core"
	"github.com/u-root/u-root/pkg/uroot/unixflag"
)

type command struct {
	core.Base
	now      func() time.Time
	location *time.Location
}

// New creates a new date command.
func New() core.Command {
	return newCommand(time.Now, time.Local)
}

func newCommand(now func() time.Time, location *time.Location) *command {
	c := &command{
		now:      now,
		location: location,
	}
	c.Init()
	return c
}

func (c *command) Run(args ...string) error {
	return c.RunContext(context.Background(), args...)
}

func (c *command) RunContext(ctx context.Context, args ...string) error {
	_ = ctx
	var utc bool
	var reference string

	fs := flag.NewFlagSet("date", flag.ContinueOnError)
	fs.SetOutput(c.Stderr)
	fs.BoolVar(&utc, "u", false, "display Coordinated Universal Time (UTC)")
	fs.StringVar(&reference, "r", "", "display the last modification time of FILE")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: date [-u] [-r FILE] [+FORMAT]")
		fmt.Fprintln(fs.Output(), "Display the current time or a file's modification time")
		fs.PrintDefaults()
	}

	if err := fs.Parse(unixflag.ArgsToGoArgs(args)); err != nil {
		return err
	}

	positional := fs.Args()
	if len(positional) > 1 {
		fs.Usage()
		return fmt.Errorf("too many arguments")
	}
	if len(positional) == 1 && !strings.HasPrefix(positional[0], "+") {
		return fmt.Errorf("setting the system clock is not supported")
	}

	t := c.now()
	if reference != "" {
		info, err := os.Stat(c.ResolvePath(reference))
		if err != nil {
			return fmt.Errorf("unable to stat %s: %w", reference, err)
		}
		t = info.ModTime()
	}

	location := c.location
	if utc {
		location = time.UTC
	}
	t = t.In(location)

	format := "%a %b %e %H:%M:%S %Z %Y"
	if len(positional) == 1 {
		format = strings.TrimPrefix(positional[0], "+")
	}
	fmt.Fprintln(c.Stdout, formatDate(t, format))
	return nil
}

func formatDate(t time.Time, format string) string {
	var out strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 == len(format) {
			out.WriteByte(format[i])
			continue
		}

		i++
		switch format[i] {
		case '%':
			out.WriteByte('%')
		case 'a':
			out.WriteString(t.Format("Mon"))
		case 'A':
			out.WriteString(t.Format("Monday"))
		case 'b', 'h':
			out.WriteString(t.Format("Jan"))
		case 'B':
			out.WriteString(t.Format("January"))
		case 'c':
			out.WriteString(t.Format(time.UnixDate))
		case 'C':
			fmt.Fprintf(&out, "%02d", t.Year()/100)
		case 'd':
			out.WriteString(t.Format("02"))
		case 'D', 'x':
			out.WriteString(t.Format("01/02/06"))
		case 'e':
			out.WriteString(t.Format("_2"))
		case 'F':
			out.WriteString(t.Format("2006-01-02"))
		case 'g':
			year, _ := t.ISOWeek()
			fmt.Fprintf(&out, "%02d", year%100)
		case 'G':
			year, _ := t.ISOWeek()
			fmt.Fprintf(&out, "%04d", year)
		case 'H':
			out.WriteString(t.Format("15"))
		case 'I':
			out.WriteString(t.Format("03"))
		case 'j':
			fmt.Fprintf(&out, "%03d", t.YearDay())
		case 'm':
			out.WriteString(t.Format("01"))
		case 'M':
			out.WriteString(t.Format("04"))
		case 'n':
			out.WriteByte('\n')
		case 'p':
			out.WriteString(t.Format("PM"))
		case 'r':
			out.WriteString(t.Format("03:04:05 PM"))
		case 's':
			fmt.Fprintf(&out, "%d", t.Unix())
		case 'S':
			out.WriteString(t.Format("05"))
		case 't':
			out.WriteByte('\t')
		case 'T', 'X':
			out.WriteString(t.Format("15:04:05"))
		case 'u':
			weekday := int(t.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			fmt.Fprintf(&out, "%d", weekday)
		case 'U':
			fmt.Fprintf(&out, "%02d", weekNumber(t, time.Sunday))
		case 'V':
			_, week := t.ISOWeek()
			fmt.Fprintf(&out, "%02d", week)
		case 'w':
			fmt.Fprintf(&out, "%d", t.Weekday())
		case 'W':
			fmt.Fprintf(&out, "%02d", weekNumber(t, time.Monday))
		case 'y':
			out.WriteString(t.Format("06"))
		case 'Y':
			out.WriteString(t.Format("2006"))
		case 'z':
			out.WriteString(t.Format("-0700"))
		case 'Z':
			out.WriteString(t.Format("MST"))
		default:
			out.WriteByte('%')
			out.WriteByte(format[i])
		}
	}
	return out.String()
}

func weekNumber(t time.Time, firstDay time.Weekday) int {
	dayOfYear := t.YearDay() - 1
	weekday := (int(t.Weekday()) - int(firstDay) + 7) % 7
	return (dayOfYear + 7 - weekday) / 7
}
