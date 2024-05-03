// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Unicode is a command-line tool for studying Unicode characters.

usage: unicode [-c] [-d] [-n] [-t]

	-c: args are hex; output characters (xyz)
	-n: args are characters; output hex (23 or 23-44)
	-g: args are regular expressions for matching names (see note below)
	-d: output textual description
	-t: output plain text, not one char per line
	-U: output full Unicode description
	-s: sort before output (only useful with -g and multiple regexps)

Default behavior sniffs the arguments to select -c vs. -n.

For -g, regexps are matched against a search-string composed of
the Name and Unicode 1.0 Name fields. The search-string has the
form '{Name};{Unicode 1.0 Name}', and has only one semicolon
between Name and Unicode 1.0 Name. Even if the search-string does
not have a Unicode 1.0 Name, it will have the semicolon followed
by the empty string (as a placeholder). This allows for querys
to single-out Name or 1.0 Name, e.g., '^regexp1;' to fully match
Name, or ';regexp2' to match just the start of a 1.0 Name.
*/
package main // import "robpike.io/cmd/unicode"

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	doNum  = flag.Bool("n", false, "output numeric values")
	doChar = flag.Bool("c", false, "output characters")
	doText = flag.Bool("t", false, "output plain text")
	doDesc = flag.Bool("d", false, "describe the characters from the Unicode database, in simple form")
	doUnic = flag.Bool("u", false, "describe the characters from the Unicode database, in Unicode form")
	doUNIC = flag.Bool("U", false, "describe the characters from the Unicode database, in glorious detail")
	doGrep = flag.Bool("g", false, "grep for argument string in data")
	doSort = flag.Bool("s", false, "sort characters before outputting/describing")
)

var printRange = false

const delim = ";"

// See <https://www.unicode.org/reports/tr44/#Data_Fields> for
// the broader spec for this file.
//
//go:generate sh -c "curl http://ftp.unicode.org/Public/UNIDATA/UnicodeData.txt >UnicodeData.txt"
var (
	//go:embed UnicodeData.txt
	unicodeDataTxt string

	// unicodeLines is a slice of strings of lines from UnicodeData.txt.
	// Each line contains 15 fields separated by delim. See
	// <https://www.unicode.org/reports/tr44/#UnicodeData.txt> for
	// field definitions.
	unicodeLines = splitLines(unicodeDataTxt)
)

func main() {
	flag.Usage = usage
	flag.Parse()
	mode()
	var codes []rune
	switch {
	case *doGrep:
		codes = argsAreRegexps()
		codes = dedupe(codes)
		if *doSort {
			slices.Sort(codes)
		}
	case *doChar:
		codes = argsAreNumbers()
	case *doNum:
		codes = argsAreChars()
	}
	if *doUnic || *doUNIC || *doDesc {
		desc(codes)
		return
	}
	if *doText {
		fmt.Printf("%s\n", string(codes))
		return
	}
	b := new(bytes.Buffer)
	for i, c := range codes {
		switch {
		case printRange:
			fmt.Fprintf(b, "%.4x %c", c, c)
			if i%4 == 3 {
				fmt.Fprint(b, "\n")
			} else {
				fmt.Fprint(b, "\t")
			}
		case *doChar:
			fmt.Fprintf(b, "%c\n", c)
		case *doNum:
			fmt.Fprintf(b, "%.4x\n", c)
		}
	}
	if b.Len() > 0 && b.Bytes()[b.Len()-1] != '\n' {
		fmt.Fprint(b, "\n")
	}
	fmt.Print(b)
}

func fatalf(format string, args ...interface{}) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(2)
}

const usageText = `usage: unicode [-c] [-d] [-n] [-t]

-c: args are hex; output characters (xyz)
-n: args are characters; output hex (23 or 23-44)
-g: args are regular expressions for matching names (see note below)
-d: output textual description
-t: output plain text, not one char per line
-U: output full Unicode description
-s: sort before output (only useful with -g and multiple regexps)

Default behavior sniffs the arguments to select -c vs. -n.

For -g, regexps are matched against a search-string composed of
the Name and Unicode 1.0 Name fields. The search-string has the
form '{Name};{Unicode 1.0 Name}', and has only one semicolon
between Name and Unicode 1.0 Name. Even if the search-string does
not have a Unicode 1.0 Name, it will have the semicolon followed
by the empty string (as a placeholder). This allows for querys
to single-out Name or 1.0 Name, e.g., '^regexp1;' to fully match
Name, or ';regexp2' to match just the start of a 1.0 Name.
`

func usage() {
	fatalf("%s", usageText)
}

// Mode determines whether we have numeric or character input.
// If there are no flags, we sniff the first argument.
func mode() {
	if len(flag.Args()) == 0 {
		usage()
	}
	// If grepping names, we need an output format defined; default is numeric.
	if *doGrep && !(*doNum || *doChar || *doDesc || *doUnic || *doUNIC) {
		*doNum = true
	}
	if *doNum || *doChar {
		return
	}
	alldigits := true
	numDash := 0
	for _, r := range strings.Join(flag.Args(), "") {
		if !strings.ContainsRune("0123456789abcdefABCDEF-", r) {
			alldigits = false
		}
		if r == '-' {
			numDash++
		}
	}
	// If there is one '-' it's a range; if zero it's just a hex number.
	if alldigits && numDash <= 1 {
		*doChar = true
		return
	}
	*doNum = true
}

func argsAreChars() []rune {
	var codes []rune
	for i, a := range flag.Args() {
		for _, r := range a {
			codes = append(codes, r)
		}
		// Add space between arguments if output is plain text.
		if *doText && i < len(flag.Args())-1 {
			codes = append(codes, ' ')
		}
	}
	return codes
}

func parseRune(s string) rune {
	r, err := strconv.ParseInt(s, 16, 22)
	if err != nil {
		fatalf("%s", err)
	}
	return rune(r)
}

func argsAreNumbers() []rune {
	var codes []rune
	for _, a := range flag.Args() {
		if s := strings.Split(a, "-"); len(s) == 2 {
			printRange = true
			r1 := parseRune(s[0])
			r2 := parseRune(s[1])
			if r2 < r1 {
				usage()
			}
			for ; r1 <= r2; r1++ {
				codes = append(codes, r1)
			}
			continue
		}
		codes = append(codes, parseRune(a))
	}
	return codes
}

// argsAreRegexps returns runes for search-strings that match some
// number of user-supplied regexps.
//
// A search-string is composed of a Name and (optionally) Unicode
// 1.0 Name, in the form, '{Name};{Unicode 1.0 Name}'; it has only
// one semicolon, between Name and Unicode 1.0 Name. Even if a
// search-string doesn't have a 1.0 Name, it will have the semicolon
// followed by the empty string, '{Name};'.
func argsAreRegexps() []rune {
	var codes []rune
	for _, a := range flag.Args() {
		re, err := regexp.Compile(a)
		if err != nil {
			fatalf("%s", err)
		}
		for _, line := range unicodeLines {
			fields := strings.Split(strings.ToLower(line), ";")
			line = fields[1] + ";" + fields[10]
			if re.MatchString(line) {
				codes = append(codes, parseRune(fields[0]))
			}
		}
	}
	return codes
}

func splitLines(text string) []string {
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if len(lines[i]) == 0 {
			lines = slices.Delete(lines, i, i+1)
			continue
		}
		if strings.Index(lines[i], delim) < 0 {
			fatalf("malformed database: line %d", i+1)
		}
	}
	return lines
}

// runeOfLine returns the parsed rune and the index of its
// trailing delimiter.
func runeOfLine(line string) (r rune, i int) {
	i = strings.Index(line, delim)
	return parseRune(line[0:i]), i
}

func desc(codes []rune) {
	runeData := make(map[rune]string)
	for _, l := range unicodeLines {
		r, i := runeOfLine(l)
		runeData[r] = l[i+1:]
	}
	if *doUNIC {
		for _, r := range codes {
			fmt.Printf("%#U %s", r, dumpUnicode(runeData[r]))
		}
	} else if *doUnic {
		for _, r := range codes {
			fmt.Printf("%#U %s\n", r, runeData[r])
		}
	} else {
		for _, r := range codes {
			fields := strings.Split(strings.ToLower(runeData[r]), delim)
			desc := fields[0]
			if len(desc) >= 9 && fields[9] != "" {
				desc += "; " + fields[9]
			}
			fmt.Printf("%#U %s\n", r, desc)
		}
	}
}

// dedupe returns codes with duplicate runes removed. It preserves
// the original order of the remaining runes.
func dedupe(codes []rune) []rune {
	var (
		deduped = make([]rune, 0)
		m       = make(map[rune]bool)
	)
	for _, r := range codes {
		if !m[r] {
			deduped = append(deduped, r)
			m[r] = true
		}
	}
	return deduped
}

var prop = [...]string{
	"",
	"category: ",
	"canonical combining classes: ",
	"bidirectional category: ",
	"character decomposition mapping: ",
	"decimal digit value: ",
	"digit value: ",
	"numeric value: ",
	"mirrored: ",
	"Unicode 1.0 name: ",
	"10646 comment field: ",
	"uppercase mapping: ",
	"lowercase mapping: ",
	"titlecase mapping: ",
}

// dumpUnicode prints the unicodeLine line, one printed line
// per non-empty field in line.
func dumpUnicode(line string) []byte {
	fields := strings.Split(line, delim)
	if len(fields) == 0 {
		return []byte{'\n'}
	}
	b := new(bytes.Buffer)
	if len(fields) != len(prop) {
		fmt.Fprintf(b, "%s: can't print: expected %d fields, got %d\n", line, len(prop), len(fields))
		return b.Bytes()
	}
	for i, f := range fields {
		if f == "" {
			continue
		}
		if i > 0 {
			b.WriteByte('\t')
		}
		fmt.Fprintf(b, "%s%s\n", prop[i], f)
	}
	return b.Bytes()
}
