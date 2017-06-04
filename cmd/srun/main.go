package main

import (
	"context"
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
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}
	runner := srun.New(cli)
	if err := runner.AddLanguage("go", srun.Go{}); err != nil {
		return err
	}
	var f io.Reader
	if len(os.Args) < 2 {
		f = os.Stdin
	} else {
		var err error
		f, err = os.Open(os.Args[1])
		if err != nil {
			return err
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

	res, err := runner.Run(context.Background(), "go", string(source))
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
