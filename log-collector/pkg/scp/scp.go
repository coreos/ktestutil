package scp

import (
	"fmt"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Config struct {
	User            string
	Host            string
	Port            int32
	IdentifyKeyFile string
}

type Scp struct {
	conn   *ssh.Client
	client *sftp.Client
}

func NewClient(config *Config) (*Scp, error) {
	conn, err := newSSHClient(config)
	if err != nil {
		return nil, err
	}

	client, err := sftp.NewClient(conn)
	if err != nil {
		return nil, err
	}

	return &Scp{
		conn:   conn,
		client: client,
	}, nil
}

func (s *Scp) Close() {
	s.client.Close()
	s.conn.Close()
}

func (s *Scp) GetPodsFilePaths(pod string) ([]string, error) {
	pattern := fmt.Sprintf("/var/log/log-collector/container.*%s*.log", pod)
	return s.client.Glob(pattern)
}

func (s *Scp) GetServicesFilePaths(name string) ([]string, error) {
	pattern := fmt.Sprintf("/var/log/log-collector/service.*%s*.log", name)
	return s.client.Glob(pattern)
}

func (s *Scp) Open(path string) (*sftp.File, error) {
	return s.client.Open(path)
}
