package srun

import (
	"archive/tar"
	"bytes"
	"encoding/binary"
	"io"
	"runtime"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type Result struct {
	Stdout     []byte
	Stderr     []byte
	ExitStatus int
}

func Run(cli *client.Client, source string) (res *Result, err error) {
	res = new(Result)
	cmd := []string{"sh", "-c", "go build main.go && ./main"}
	ctx := context.Background()

	hostcfg := defaultHostConfig()
	body, err := cli.ContainerCreate(ctx, &container.Config{
		Image:           "golang:1.8-alpine",
		Cmd:             cmd,
		WorkingDir:      "/go/src/app",
		NetworkDisabled: true,
	}, hostcfg, &network.NetworkingConfig{}, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create container")
	}

	if err := copyToContainer(ctx, cli, body.ID, "/go/src/app", "main.go", source); err != nil {
		return nil, err
	}

	defer func() {
		rmerr := cli.ContainerRemove(ctx, body.ID, types.ContainerRemoveOptions{Force: true})
		if rmerr != nil && err == nil {
			err = errors.Wrap(rmerr, "failed to remove container")
		}
	}()

	since := time.Now().Add(-1 * time.Second)

	if err := cli.ContainerStart(ctx, body.ID, types.ContainerStartOptions{}); err != nil {
		return nil, errors.Wrap(err, "failed to start container")
	}

	res.Stdout, res.Stderr, err = readLogs(ctx, cli, body.ID, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read logs")
	}

	exitCh := make(chan int64)
	errCh := make(chan error)

	go func() {
		exit, err := cli.ContainerWait(ctx, body.ID)
		if err != nil {
			errCh <- err
		} else {
			exitCh <- exit
		}
	}()

	select {
	case exit := <-exitCh:
		res.ExitStatus = int(exit)
	case err := <-errCh:
		return nil, errors.Wrap(err, "failed to wait the container")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return res, nil
}

func defaultHostConfig() *container.HostConfig {
	cfg := new(container.HostConfig)
	cfg.DiskQuota = 1024 * 64
	cfg.PidsLimit = 128
	cfg.CPUPeriod = 100000
	cfg.CPUQuota = 100000 / (int64(runtime.NumCPU()) - 1)
	return cfg
}

func copyToContainer(ctx context.Context, cli *client.Client, id string, distdir string, distname string, content string) error {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	hdr := &tar.Header{Name: distname, Mode: 0644, Size: int64(len(content))}
	if err := tw.WriteHeader(hdr); err != nil {
		return errors.Wrap(err, "failed to write a tar header")
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		return errors.Wrap(err, "failed to write a tar body")
	}
	if err := tw.Close(); err != nil {
		return errors.Wrap(err, "failed to close tar archive")
	}

	r := bytes.NewReader(buf.Bytes())
	if err := cli.CopyToContainer(ctx, id, distdir, r, types.CopyToContainerOptions{AllowOverwriteDirWithFile: true}); err != nil {
		return errors.Wrap(err, "failed to copy source code")
	}
	return nil
}

func readLogs(ctx context.Context, cli *client.Client, id string, since time.Time) ([]byte, []byte, error) {
	r, err := cli.ContainerLogs(ctx, id, types.ContainerLogsOptions{
		Since:      time.Now().Sub(since).String(),
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get log")
	}

	const maxLength = 2048
	stdout := make([]byte, 0)
	stderr := make([]byte, 0)
loop:
	for {
		outtype := make([]byte, 4)
		_, err := r.Read(outtype)
		if err != nil && err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, errors.Wrap(err, "faield to read log")
		}
		var size uint32
		if err := binary.Read(r, binary.BigEndian, &size); err != nil {
			return nil, nil, errors.Wrap(err, "failed to read size")
		}
		o := make([]byte, uint(size))
		read := 0
		for uint32(read) < size {
			n, err := r.Read(o[read:])
			if err != nil {
				return nil, nil, errors.Wrap(err, "failed to read body")
			}
			read += n
		}
		switch outtype[0] {
		case 1:
			stdout = append(stdout, o...)
			if len(stdout) > maxLength {
				stdout = stdout[:maxLength]
				break loop
			}
		case 2:
			stderr = append(stderr, o...)
			if len(stderr) > maxLength {
				stderr = stderr[:maxLength]
				break loop
			}
		default:
			return nil, nil, errors.Errorf("unknown chunk type: %v", outtype)
		}
	}
	return stdout, stderr, nil
}
