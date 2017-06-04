package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/agatan/srun"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (err error) {
	// parse options
	typ := flag.String("type", "", "type of source code")
	list := flag.Bool("list", false, "list supported languages")
	flag.Parse()

	if *list {
		for _, lang := range listLanguages() {
			fmt.Println(lang)
		}
		return nil
	}

	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}
	runner := srun.New(cli)
	var lang srun.Language

	if *typ != "" {
		l, ok := srun.Languages[*typ]
		if !ok {
			return errors.Errorf("%q is not supported", *typ)
		}
		lang = l
	}

	var f io.Reader
	args := flag.Args()
	if len(args) < 1 {
		if lang == nil {
			return errors.New("can't read source code from stdin without -type option")
		}
		f = os.Stdin
	} else {
		var err error
		f, err = os.Open(args[0])
		if err != nil {
			return err
		}
		if lang == nil {
			l, ok := findLanguage(args[0])
			if !ok {
				return errors.Errorf("can't identifier language for %q without -type option", args[0])
			}
			lang = l
		}
	}
	defer func() {
		if c, ok := f.(io.Closer); ok {
			if cerr := c.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}
	}()

	source, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	res, err := runner.Run(context.Background(), lang, string(source))
	if err != nil {
		return err
	}

	if _, err := os.Stdout.Write(res.Stdout); err != nil {
		return err
	}
	if _, err := os.Stderr.Write(res.Stderr); err != nil {
		return err
	}
	if res.ExitStatus != 0 {
		return errors.Errorf("exit status %d", res.ExitStatus)
	}
	return nil
}

func listLanguages() []string {
	langs := make([]string, 0, len(srun.Languages))
	for k, _ := range srun.Languages {
		langs = append(langs, k)
	}
	sort.Strings(langs)
	return langs
}

func findLanguage(filename string) (srun.Language, bool) {
	fileext := filepath.Ext(filename)
	for _, lang := range srun.Languages {
		for _, ext := range lang.Extensions() {
			if fileext == ext {
				return lang, true
			}
		}
	}
	return nil, false
}
