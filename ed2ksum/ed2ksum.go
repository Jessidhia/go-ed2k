package main

import (
	"flag"
	"fmt"
	"github.com/Kovensky/go-ed2k"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var useNullChunk = flag.Bool("null-chunk", false,
	`If true, append a null chunk to the end of files with size multiple of the ed2k chunk size.
                     This is used for the primary hash in AniDB, and was used in older versions of ed2k.`)
var checkMode = flag.Bool("c", false,
	`If true, takes a previous output of this program and verifies the hashes.`)
var uriMode = flag.Bool("uri", false,
	`If true, outputs ed2k URIs instead of a verifiable digest.`)

func hashFile(chunkMode bool, path string) (hash string, err error) {
	var fh *os.File
	if path == "-" {
		fh = os.Stdin
	} else {
		fh, err = os.Open(path)
		if err != nil {
			return
		}
		defer fh.Close()
	}

	ed2k := ed2k.New(chunkMode)
	io.Copy(ed2k, fh)
	return ed2k.(fmt.Stringer).String(), err
}

func makeLine(hash string, chunkMode bool, path string) string {
	mode := " "
	if chunkMode {
		mode = "*"
	}
	return fmt.Sprintf("%s %s%s", hash, mode, path)
}

func makeURI(hash string, path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}
	return strings.Join([]string{"ed2k://", "file", path, strconv.FormatInt(fi.Size(), 10), hash, "/"}, "|")
}

func makeDigest(paths ...string) (digest string) {
	lines := make([]string, 0, len(paths))
	for _, path := range paths {
		hash, err := hashFile(*useNullChunk, path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			if *uriMode {
				lines = append(lines, makeURI(hash, path))
			} else {
				lines = append(lines, makeLine(hash, *useNullChunk, path))
			}
		}
	}
	return strings.Join(lines, "\n")
}

var digestRegex = regexp.MustCompile(`(?P<hash>[0-9A-Fa-f]{32}) (?P<mode>[ *])(?P<path>[^\n]+)`)

func checkDigest(digest string) int {
	errCount := 0
	for _, match := range digestRegex.FindAllStringSubmatch(digest, -1) {
		baseHash, modeStr, path := match[1], match[2], match[3]
		mode := false
		if modeStr == "*" {
			mode = true
		}
		hash, err := hashFile(mode, path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			status := "OK"
			if baseHash != hash {
				status = "FAILED"
				errCount++
			}
			fmt.Printf("%s: %s\n", path, status)
		}
	}
	return errCount
}

type Pluralizable string

func (p Pluralizable) DumbPluralize(count int) string {
	if count == 1 {
		return string(p)
	}
	return string(p) + "s"
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"-"}
	}
	if *checkMode {
		errCount := 0
		for _, file := range args {
			var err error
			var fh *os.File
			if file == "-" {
				fh = os.Stdin
			} else {
				fh, err = os.Open(file)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					continue
				}
				defer fh.Close()
			}
			digest, err := ioutil.ReadAll(fh)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else {
				errCount += checkDigest(string(digest))
			}
		}
		if errCount > 0 {
			fmt.Fprintln(os.Stderr, os.Args[0]+": WARNING:", errCount, "computed",
				Pluralizable("checksum").DumbPluralize(errCount), "did NOT match")
			os.Exit(1)
		}
	} else {
		fmt.Println(makeDigest(args...))
	}
}
