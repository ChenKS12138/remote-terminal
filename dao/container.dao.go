package dao

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

const (
	CONTAINER_IMAGE   = "debian:8"
)

type ContainerDao struct {
	ctx context.Context
	cli *client.Client
}

func NewContainerDao() (*ContainerDao, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &ContainerDao{
		ctx: context.Background(),
		cli: cli,
	}, nil
}

func (c *ContainerDao) getContainerName(ID string) string {
	configDao := NewConfigDaoMust()
	prefix := configDao.ContainerPrefix
	if len(prefix) == 0 {
		prefix = "remote_terminal_default"
	}
	return fmt.Sprintf("%s-%s", prefix, ID)
}

func (c *ContainerDao) findByContainerID(containerID string) (*types.Container, error) {
	containers, err := c.cli.ContainerList(c.ctx, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}
	for _, container := range containers {
		if container.ID == containerID {
			return &container, nil
		}
	}
	return nil, nil
}

func (c *ContainerDao) FindByID(ID string) (*types.Container, error) {
	containers, err := c.cli.ContainerList(c.ctx, types.ContainerListOptions{
		All: true,
	})
	containerName := c.getContainerName(ID)
	if err != nil {
		return nil, err
	}
	for _, container := range containers {
		if len(container.Names) != 0 && container.Names[0] == fmt.Sprintf("/%s", containerName) {
			return &container, nil
		}
	}
	return nil, nil
}

func (c *ContainerDao) CreateByID(ID string, out io.Writer) (*types.Container, error) {
	reader, err := c.cli.ImagePull(c.ctx, CONTAINER_IMAGE, types.ImagePullOptions{})
	if err != nil {
		return nil, err
	}
	io.Copy(out, reader)
	resp, err := c.cli.ContainerCreate(c.ctx, &container.Config{
		AttachStderr: true,
		AttachStdin:  true,
		Tty:          true,
		AttachStdout: true,
		OpenStdin:    true,
		Cmd:          []string{"bash","-c","sleep 10;while true;do if [ $( ls /dev/pts | wc -l ) -gt 2 ];then sleep 10;else exit 1;fi ;done"},
		Image:        CONTAINER_IMAGE,
	}, &container.HostConfig{
		ExtraHosts: []string{
			"host.docker.internal:host-gateway",
		},
	}, nil, nil, c.getContainerName(ID))
	if err != nil {
		return nil, err
	}
	container, err := c.findByContainerID(resp.ID)
	return container, err
}

func (c *ContainerDao) AttachAndWait(cont *types.Container, in io.Reader, out io.Writer, wsCloseChan chan interface{}, resizeChan chan [2]float64) error {
	switch strings.ToLower(cont.State) {
	case "paused":
		fallthrough
	case "exited":
		fallthrough
	case "created":
		if err := c.cli.ContainerStart(c.ctx, cont.ID, types.ContainerStartOptions{}); err != nil {
			return err
		}
	case "restarting":
		fallthrough
	case "running":
		// do nothing
	case "removing":
		fallthrough
	case "dead":
		return fmt.Errorf("can not restart %s container",cont.State)
	default:
		return fmt.Errorf("unexpected container state %s",cont.State)
	}

	execId,err := c.cli.ContainerExecCreate(c.ctx,cont.ID,types.ExecConfig{
		Privileged: false,
		Tty: true,
		AttachStdin: true,
		AttachStderr: true,
		AttachStdout: true,
		Detach: false,
		Cmd: []string{"/bin/bash"},
		Env: []string{"TERM=xterm-256color"},
	})
	if err != nil {
		return err
	}

	
	waiter, err := c.cli.ContainerExecAttach(c.ctx, execId.ID, types.ExecStartCheck{
		Detach: false,
		Tty: true,
	})
	if err != nil {
		return err
	}
	go io.Copy(out, waiter.Reader)
	go io.Copy(waiter.Conn, in)

	for {
		select {
		case <- wsCloseChan:
			waiter.Conn.Write([]byte("\u0004"))
			waiter.Close()
			return nil
		case size := <- resizeChan:
			if err = c.cli.ContainerExecResize(c.ctx,execId.ID,types.ResizeOptions{
				Height: uint(size[0]),
				Width: uint(size[1]),
			}) ; err != nil {
				return err
			}
		}
	}

}


// func (c *ContainerDao) Shutdown(cont *types.Container) error {
// 	resp,err := c.cli.ContainerStatsOneShot(c.ctx,cont.ID)
// 	if err != nil {
// 		return err
// 	}
// 	stats := types.Stats{}
// 	if err = json.NewDecoder(resp.Body).Decode(&stats) ;err != nil {
// 		return err
// 	}
// 	if stats.PidsStats.Current > 1 {
// 		return nil
// 	}
// 	timeout := time.Duration(0) * time.Second
// 	return c.cli.ContainerStop(c.ctx, cont.ID, &timeout)
// }
