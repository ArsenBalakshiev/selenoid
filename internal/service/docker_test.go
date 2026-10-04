package service

import (
	"testing"

	"github.com/aerokube/selenoid/internal/session"
	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
	assert "github.com/stretchr/testify/require"
)

func newTestStat(ip string, ports nat.PortMap) types.ContainerJSON {
	return types.ContainerJSON{
		NetworkSettings: &types.NetworkSettings{
			NetworkSettingsBase: types.NetworkSettingsBase{Ports: ports},
			DefaultNetworkSettings: types.DefaultNetworkSettings{
				IPAddress: ip,
			},
		},
	}
}

func seleniumPort(t *testing.T) nat.Port {
	port, err := nat.NewPort("tcp", "4444")
	assert.NoError(t, err)
	return port
}

func hostBinding(t *testing.T, hostPort string) nat.PortMap {
	port, err := nat.NewPort("tcp", "4444")
	assert.NoError(t, err)
	return nat.PortMap{port: []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostPort}}}
}

func TestGetHostPortInsideDockerEmptyBindings(t *testing.T) {
	port := seleniumPort(t)
	stat := newTestStat("172.17.0.2", nat.PortMap{})
	env := Environment{InDocker: true}
	hp := getHostPort(env, "4444", session.Caps{}, stat, map[string]nat.Port{"4444": port})
	assert.Equal(t, "172.17.0.2:4444", hp.Selenium)
}

func TestGetHostPortLocalhostEmptyBindings(t *testing.T) {
	port := seleniumPort(t)
	stat := newTestStat("", nat.PortMap{})
	env := Environment{InDocker: false}
	hp := getHostPort(env, "4444", session.Caps{}, stat, map[string]nat.Port{"4444": port})
	assert.Equal(t, "", hp.Selenium)
}

func TestGetHostPortRemoteEmptyBindings(t *testing.T) {
	port := seleniumPort(t)
	stat := newTestStat("", nat.PortMap{})
	env := Environment{IP: "172.17.0.1"}
	hp := getHostPort(env, "4444", session.Caps{}, stat, map[string]nat.Port{"4444": port})
	assert.Equal(t, "", hp.Selenium)
}

func TestGetHostPortRemoteWithBindings(t *testing.T) {
	port := seleniumPort(t)
	stat := newTestStat("", hostBinding(t, "32768"))
	env := Environment{IP: "172.17.0.1"}
	hp := getHostPort(env, "4444", session.Caps{}, stat, map[string]nat.Port{"4444": port})
	assert.Equal(t, "172.17.0.1:32768", hp.Selenium)
}

func TestGetContainerPortsEmptyBindings(t *testing.T) {
	port := seleniumPort(t)
	stat := newTestStat("", nat.PortMap{port: nil})
	ports := getContainerPorts(stat)
	assert.Empty(t, ports)
}
