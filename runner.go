package srun

import (
	"encoding/binary"
	"io"
	"runtime"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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
	ctx := context.Background()
	containerID, err := (Go{}).CreateContainer(ctx, cli, source)

	defer func() {
		rmerr := cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
		if rmerr != nil && err == nil {
			err = errors.Wrap(rmerr, "failed to remove container")
		}
	}()

	since := time.Now().Add(-1 * time.Second)

	if err := cli.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return nil, errors.Wrap(err, "failed to start container")
	}

	res.Stdout, res.Stderr, err = readLogs(ctx, cli, containerID, since)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read logs")
	}

	exitCh := make(chan int64)
	errCh := make(chan error)

	go func() {
		exit, err := cli.ContainerWait(ctx, containerID)
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
