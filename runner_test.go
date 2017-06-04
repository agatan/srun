package srun

import (
	"bytes"
	"context"
	"testing"

	"github.com/docker/docker/client"
)

func TestRunGo(t *testing.T) {
	cli, err := client.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}
	runner := New(cli)
	if err := runner.AddLanguage("go", Go{}); err != nil {
		t.Fatal(err)
	}
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

	for _, test := range tests {
		ctx := context.Background()
		res, err := runner.Run(ctx, "go", test.source)
		if err != nil {
			t.Fatalf("should not be error for %v but: %+v", test.source, err)
		}
		if res.ExitStatus != test.exit {
			t.Errorf("exit status should be %d, but got %d", test.exit, res.ExitStatus)
		}
		if !bytes.Equal(res.Stdout, test.stdout) {
			t.Errorf("stdout should be %q, but %q", string(test.stdout), string(res.Stdout))
		}
		if !bytes.Equal(res.Stderr, test.stderr) {
			t.Errorf("stderr should be %q, but %q", string(test.stderr), string(res.Stderr))
		}
	}
}
