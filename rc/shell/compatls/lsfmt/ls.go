// Copyright 2019 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lsfmt implements formatting tools to list files like Linux ls.
package lsfmt

import (
	"fmt"
	"regexp"
)

var unprintableRe = regexp.MustCompile("[[:cntrl:]\n]")

func (fi FileInfo) PrintableName() string {
	return unprintableRe.ReplaceAllLiteralString(fi.Name, "?")
}

type Stringer interface {
	FileString(fi FileInfo) string
}

type NameStringer struct{}

func (ns NameStringer) FileString(fi FileInfo) string {
	return fi.PrintableName()
}

type QuotedStringer struct{}

func (qs QuotedStringer) FileString(fi FileInfo) string {
	return fmt.Sprintf("%#v", fi.Name)
}
