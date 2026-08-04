package docker

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
)

// Client wraps the Docker Engine API client.
type Client struct {
	cli *client.Client
}

// Container is a simplified view of a Docker container for the API.
type Container struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// CreateOptions describes a container to create.
type CreateOptions struct {
	Name   string
	Image  string
	Ports  []int
	Labels map[string]string
}

// ContainerRuntime is the live state of a container in Docker.
type ContainerRuntime struct {
	Exists  bool
	Running bool
	Labels  map[string]string
}

// New connects to the local Docker daemon using environment defaults
// (typically /var/run/docker.sock).
func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// Close releases the Docker client connection.
func (c *Client) Close() error {
	return c.cli.Close()
}

// ListContainers returns containers known to Docker.
func (c *Client) ListContainers(ctx context.Context) ([]Container, error) {
	result, err := c.cli.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	containers := make([]Container, 0, len(result.Items))
	for _, ctr := range result.Items {
		name := ""
		if len(ctr.Names) > 0 {
			name = strings.TrimPrefix(ctr.Names[0], "/")
		}
		containers = append(containers, Container{
			ID:     ctr.ID,
			Name:   name,
			Status: string(ctr.State),
		})
	}
	return containers, nil
}

// CreateContainer pulls the image if needed, creates the container, and starts it.
func (c *Client) CreateContainer(ctx context.Context, opts CreateOptions) (Container, error) {
	if err := c.pullImage(ctx, opts.Image); err != nil {
		return Container{}, fmt.Errorf("pull image %s: %w", opts.Image, err)
	}

	exposedPorts, portBindings, err := buildPortMappings(opts.Ports)
	if err != nil {
		return Container{}, err
	}

	createResult, err := c.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Image: opts.Image,
		Config: &container.Config{
			ExposedPorts: exposedPorts,
			Labels:       opts.Labels,
		},
		HostConfig: &container.HostConfig{
			PortBindings: portBindings,
		},
		NetworkingConfig: &network.NetworkingConfig{},
		Name:             opts.Name,
	})
	if err != nil {
		return Container{}, fmt.Errorf("create container: %w", err)
	}

	if _, err := c.cli.ContainerStart(ctx, createResult.ID, client.ContainerStartOptions{}); err != nil {
		return Container{}, fmt.Errorf("start container: %w", err)
	}

	return Container{
		ID:     createResult.ID,
		Name:   opts.Name,
		Status: "running",
	}, nil
}

// DeleteContainer stops a running container (if needed) and removes it.
func (c *Client) DeleteContainer(ctx context.Context, id string) error {
	if _, err := c.cli.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	return nil
}

// StartContainer starts an existing container.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	if _, err := c.cli.ContainerStart(ctx, id, client.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("start container: %w", err)
	}
	return nil
}

// StopContainer stops a running container without removing it.
func (c *Client) StopContainer(ctx context.Context, id string) error {
	timeout := 10
	if _, err := c.cli.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("stop container: %w", err)
	}
	return nil
}

func (c *Client) pullImage(ctx context.Context, imageName string) error {
	pullResp, err := c.cli.ImagePull(ctx, imageName, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer pullResp.Close()
	return pullResp.Wait(ctx)
}

// GetContainerRuntime returns whether a container exists, is running, and its labels.
func (c *Client) GetContainerRuntime(ctx context.Context, id string) (ContainerRuntime, error) {
	info, err := c.cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		if cerrdefs.IsNotFound(err) {
			return ContainerRuntime{Exists: false}, nil
		}
		return ContainerRuntime{}, fmt.Errorf("inspect container: %w", err)
	}

	labels := map[string]string{}
	if info.Container.Config != nil && info.Container.Config.Labels != nil {
		labels = info.Container.Config.Labels
	}

	return ContainerRuntime{
		Exists:  true,
		Running: info.Container.State != nil && info.Container.State.Running,
		Labels:  labels,
	}, nil
}

// Ping verifies the Docker daemon is reachable.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx, client.PingOptions{})
	return err
}

// Does not allow for horizontinal scaling, however I guess its good for prototyping
// No bridge networks or anything, just maps one container port to host port
func buildPortMappings(ports []int) (network.PortSet, network.PortMap, error) {
	exposedPorts := network.PortSet{}
	portBindings := network.PortMap{}

	for _, p := range ports {
		port, err := network.ParsePort(fmt.Sprintf("%d/tcp", p))
		if err != nil {
			return nil, nil, fmt.Errorf("parse port %d: %w", p, err)
		}
		exposedPorts[port] = struct{}{}
		portBindings[port] = []network.PortBinding{{HostPort: strconv.Itoa(p)}}
	}

	return exposedPorts, portBindings, nil
}
