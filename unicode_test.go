package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

var quoteFlag = flag.Bool("quote", false, "quote got and want in failures")

func TestCLI(t *testing.T) {
	flag.Parse()

	a, err := txtar.ParseFile(filepath.Join("testdata", "cli.txt"))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < len(a.Files); i += 2 {
		if a.Files[i+1].Name != "want" {
			t.Fatalf("file %d wasn't named \"want\"", i+1)
		}
		var (
			tname = a.Files[i].Name
			args  = string(a.Files[i].Data)
			want  = string(a.Files[i+1].Data)
		)

		t.Run(tname, func(t *testing.T) {
			// Reset package main
			*doNum = false
			*doChar = false
			*doText = false
			*doDesc = false
			*doUnic = false
			*doUNIC = false
			*doGrep = false
			printRange = false

			// Backup and restore OS
			var (
				tmpArgs   = os.Args
				tmpStdout = os.Stdout
			)
			defer func() {
				os.Args = tmpArgs
				os.Stdout = tmpStdout
			}()

			r, w, _ := os.Pipe()
			os.Stdout = w
			os.Args = append([]string{"prog"}, strings.Fields(args)...)
			go func() {
				main()
				w.Close()
			}()

			stdout, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			got := string(stdout)

			if got != want {
				if *quoteFlag {
					t.Errorf(" got:\n%q\nwant:\n%q", got, want)
				} else {
					t.Errorf(" got:\n%s\nwant:\n%s", got, want)
				}
			}
		})
	}
}
