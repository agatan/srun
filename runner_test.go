package srun

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/client"
)

func TestRunGo(t *testing.T) {
	cli, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	runner := New(cli)
	tests := []struct {
		source string
		stdout []byte
		stderr []byte
		exit   int
	}{
		{
			`
			package main
			import "fmt"

			func main() {
				fmt.Println("Hello, world!")
			}
		`, []byte("Hello, world!\n"), nil, 0},
		{
			`
			package main
			import (
				"fmt"
				"os"
			)

			func main() {
				fmt.Fprintln(os.Stderr, "Hello, world!")
			}
		`, nil, []byte("Hello, world!\n"), 0},
		{
			`
			package main
			import (
				"os"
			)

			func main() {
				os.Exit(37)
			}
		`, nil, nil, 37},
	}

	for i, test := range tests {
		test := test // ref: https://golang.org/doc/faq#closures_and_goroutines
		t.Run(fmt.Sprintf("running %d", i), func(st *testing.T) {
			st.Parallel()
			ctx := context.Background()
			res, err := runner.Run(ctx, Go, test.source)
			if err != nil {
				st.Fatalf("should not be error for %v but: %+v", test.source, err)
			}
			if res.ExitStatus != test.exit {
				st.Errorf("exit status should be %d, but got %d", test.exit, res.ExitStatus)
			}
			if !bytes.Equal(res.Stdout, test.stdout) {
				st.Errorf("stdout should be %q, but %q", string(test.stdout), string(res.Stdout))
			}
			if !bytes.Equal(res.Stderr, test.stderr) {
				st.Errorf("stderr should be %q, but %q", string(test.stderr), string(res.Stderr))
			}
		})
	}
}

func TestRunRuby(t *testing.T) {
	cli, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	runner := New(cli)
	tests := []struct {
		source string
		stdout []byte
		stderr []byte
		exit   int
	}{
		{` puts "Hello, world!" `, []byte("Hello, world!\n"), nil, 0},
		{` $stderr.puts "Hello, world!" `, nil, []byte("Hello, world!\n"), 0},
		{` exit 37 `, nil, nil, 37},
	}

	for i, test := range tests {
		test := test // ref: https://golang.org/doc/faq#closures_and_goroutines
		t.Run(fmt.Sprintf("running %d", i), func(st *testing.T) {
			st.Parallel()
			ctx := context.Background()
			res, err := runner.Run(ctx, Ruby, test.source)
			if err != nil {
				st.Fatalf("should not be error for %v but: %+v", test.source, err)
			}
			if res.ExitStatus != test.exit {
				st.Errorf("exit status should be %d, but got %d", test.exit, res.ExitStatus)
			}
			if !bytes.Equal(res.Stdout, test.stdout) {
				st.Errorf("stdout should be %q, but %q", string(test.stdout), string(res.Stdout))
			}
			if !bytes.Equal(res.Stderr, test.stderr) {
				st.Errorf("stderr should be %q, but %q", string(test.stderr), string(res.Stderr))
			}
		})
	}
}
