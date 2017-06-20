package local

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Config struct {
	Dir string
}

type Local struct {
	dir string
}

func New(c *Config) (*Local, error) {
	if c.Dir == "" {
		return nil, fmt.Errorf("empty destination directory")
	}

	dir, _ := filepath.Abs(c.Dir)
	return &Local{
		dir: dir,
	}, nil
}

func (l *Local) Put(f io.ReadSeeker, dst string) (string, error) {
	r := bufio.NewReader(f)

	path := filepath.Join(l.dir, dst)
	df, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer df.Close()

	_, err = r.WriteTo(df)
	if err != nil {
		return "", err
	}

	return path, nil
}
