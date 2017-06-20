package collector

import (
	"io"
	"log"
	"path/filepath"
	"sync"

	"github.com/coreos/ktestutil/log-collector/pkg/fluentd"
	"github.com/coreos/ktestutil/log-collector/pkg/local"
	"github.com/coreos/ktestutil/log-collector/pkg/s3"
	"github.com/coreos/ktestutil/log-collector/pkg/scp"

	"k8s.io/client-go/kubernetes"
)

type Output interface {
	Put(io.ReadSeeker, string) (string, error)
}

type Collector struct {
	scpConfig *scp.Config
	namespace string
	k8s       kubernetes.Interface

	Output Output
}

type Config struct {
	RemoteUser    string
	RemotePort    int32
	RemoteKeyFile string
	K8sClient     kubernetes.Interface
	Namespace     string
}

func New(c *Config) *Collector {
	return &Collector{
		scpConfig: &scp.Config{
			User:            c.RemoteUser,
			Port:            c.RemotePort,
			IdentifyKeyFile: c.RemoteKeyFile,
		},
		namespace: c.Namespace,
		k8s:       c.K8sClient,
	}
}

func (cr *Collector) WithLocalOuput(dstDir string) error {
	l, err := local.New(&local.Config{
		Dir: dstDir,
	})
	if err != nil {
		return err
	}

	cr.Output = l
	return nil
}

func (cr *Collector) WithS3Output(keyId, keySecret, region, bucketName, bucketPrefix string) error {
	s3, err := s3.New(&s3.Config{
		AccessKeyId:     keyId,
		AccessKeySecret: keySecret,
		BucketName:      bucketName,
		BucketPrefix:    bucketPrefix,
		Region:          region,
	})
	if err != nil {
		return err
	}

	cr.Output = s3
	return nil
}

func (cr *Collector) Start() error {
	if err := fluentd.CreateAssets(cr.k8s, cr.namespace); err != nil {
		return err
	}

	host, err := fluentd.GetNodeAddressWithMaster(cr.k8s, cr.namespace)
	if err != nil {
		return err
	}
	cr.scpConfig.Host = host
	return nil
}

func (cr *Collector) CollectPodLogs(pod string) ([]string, error) {
	scp, err := scp.NewClient(cr.scpConfig)
	if err != nil {
		return nil, err
	}
	defer scp.Close()

	paths, err := scp.GetPodsFilePaths(pod)
	if err != nil {
		return nil, err
	}

	results := cr.upload(scp, paths)
	return results, nil
}

func (cr *Collector) CollectServiceLogs(service string) ([]string, error) {
	scp, err := scp.NewClient(cr.scpConfig)
	if err != nil {
		return nil, err
	}
	defer scp.Close()

	paths, err := scp.GetServicesFilePaths(service)
	if err != nil {
		return nil, err
	}

	results := cr.upload(scp, paths)
	return results, nil
}

func (cr *Collector) upload(scp *scp.Scp, paths []string) []string {
	var results []string
	resp := make(chan string)
	go func() {
		for r := range resp {
			results = append(results, r)
		}
	}()

	maxGoroutines := 10
	var wg sync.WaitGroup
	guard := make(chan struct{}, maxGoroutines)

	for _, path := range paths {
		wg.Add(1)

		guard <- struct{}{}
		go func(fpath string, dst chan<- string) {
			defer wg.Done()
			defer func() {
				<-guard
			}()

			f, err := scp.Open(fpath)
			if err != nil {
				log.Printf("skipping fetch for file %s %v", fpath, err)
				return
			}
			defer f.Close()
			loc, err := cr.Output.Put(f, filepath.Base(f.Name()))
			if err != nil {
				log.Printf("skipping upload for file %s %v", fpath, err)
				return
			}
			dst <- loc
		}(path, resp)
	}

	wg.Wait()
	close(resp)
	close(guard)

	return results
}

func (cr *Collector) Cleanup() error {
	if err := fluentd.DeleteAssets(cr.k8s, cr.namespace); err != nil {
		return err
	}

	return nil
}
