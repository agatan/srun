package srun

import (
	"archive/tar"
	"bytes"
	"runtime"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type Language interface {
	CreateContainer(ctx context.Context, cli *client.Client, code string) (string, error)
	BaseImage() string
}

var Go Language = golang{}

type golang struct{}

func (golang) CreateContainer(ctx context.Context, cli *client.Client, code string) (string, error) {
	cmd := []string{
		"sh", "-c", "go build main.go && ./main",
	}
	hostcfg := defaultHostConfig()
	body, err := cli.ContainerCreate(ctx, &container.Config{
		Image:           "golang:1.8-alpine",
		Cmd:             cmd,
		WorkingDir:      "/go/src/app",
		NetworkDisabled: true,
		OpenStdin:       true,
		StdinOnce:       true,
	}, hostcfg, &network.NetworkingConfig{}, "")
	if err != nil {
		return "", errors.Wrap(err, "failed to create container")
	}

	if err := copyToContainer(ctx, cli, body.ID, "/go/src/app", "main.go", code); err != nil {
		return "", err
	}

	return body.ID, nil
}

func (golang) BaseImage() string {
	return "golang:1.8-alpine"
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

func defaultHostConfig() *container.HostConfig {
	cfg := new(container.HostConfig)
	cfg.DiskQuota = 1024 * 64
	cfg.PidsLimit = 128
	cfg.CPUPeriod = 100000
	cfg.CPUQuota = 100000 / (int64(runtime.NumCPU()) - 1)
	return cfg
}
