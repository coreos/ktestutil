package scp

import (
	"io/ioutil"
	"log"
	"net"
	"os"

	"fmt"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const defaultSSHPort = 22
const defaultSSHUser = "core"

func newSSHClient(config *Config) (*ssh.Client, error) {
	var authMethod ssh.AuthMethod
	sock := os.Getenv("SSH_AUTH_SOCK")
	if config.IdentifyKeyFile != "" {
		log.Println("Creating ssh client with private key")
		key, err := ioutil.ReadFile(config.IdentifyKeyFile)
		if err != nil {
			return nil, err
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, err
		}

		authMethod = ssh.PublicKeys(signer)
	} else if sock != "" {
		log.Println("Creating ssh client with ssh agent")
		sshAgent, err := net.Dial("unix", sock)
		if err != nil {
			return nil, err
		}

		authMethod = ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	} else {
		return nil, fmt.Errorf("No ssh connection authentication provided")
	}

	if config.User == "" {
		config.User = defaultSSHUser
	}
	sshConfig := &ssh.ClientConfig{
		User:            config.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	sshConfig.SetDefaults()

	if config.Port == 0 {
		config.Port = defaultSSHPort
	}
	endpoint := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client, err := ssh.Dial("tcp", endpoint, sshConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}
