package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"

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
	language := flag.String("language", "", "type of source code")
	list := flag.Bool("list", false, "list supported languages")
	pull := flag.Bool("pull", false, "pull target docker image")
	flag.Parse()

	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}
	runner := srun.New(cli)

	if *list {
		for _, lang := range runner.Languages() {
			fmt.Println(lang)
		}
		return nil
	}
	var lang srun.Language

	if *language != "" {
		l, ok := runner.FindLanguageByName(*language)
		if !ok {
			return errors.Errorf("%q is not supported", *language)
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
			l, ok := runner.FindLanguageByExt(args[0])
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

	if *pull {
		if err := runner.Pull(context.Background(), lang); err != nil {
			return err
		}
	}

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
