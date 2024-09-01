package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	serviceAccountToken  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	serviceAccountCACert = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// Interface is minimal kubernetes Client interface.
type Interface interface {
	// Do sends HTTP request to ObjectAPI server.
	Do(req *http.Request) (*http.Response, error)
	// Token returns current access token.
	Token() string
	// APIServerURL returns API server URL.
	APIServerURL() string
}

// NewInCluster creates Client if it is inside Kubernetes.
func NewInCluster() (*DefaultClient, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, fmt.Errorf("unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
	}
	token, err := ioutil.ReadFile(serviceAccountToken)
	if err != nil {
		return nil, err
	}
	ca, err := ioutil.ReadFile(serviceAccountCACert)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(ca)
	transport := &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    certPool,
	}}
	httpClient := &http.Client{Transport: transport, Timeout: time.Nanosecond * 0}

	client := &DefaultClient{
		apiServerURL: "https://" + net.JoinHostPort(host, port),
		token:        string(token),
		HttpClient:   httpClient,
	}

	// Create a new file watcher to listen for new Service Account tokens
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					token, err := ioutil.ReadFile(serviceAccountToken)
					if err == nil {
						client.tokenMu.Lock()
						client.token = string(token)
						client.tokenMu.Unlock()
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	err = watcher.Add(serviceAccountToken)
	if err != nil {
		return nil, err
	}

	return client, nil
}

type DefaultClient struct {
	HttpClient *http.Client
	//Logger     Logger
	apiServerURL string

	tokenMu sync.RWMutex
	token   string
}

func (kc *DefaultClient) Do(req *http.Request) (*http.Response, error) {
	if token := kc.Token(); len(token) > 0 {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return kc.HttpClient.Do(req)
}

func (kc *DefaultClient) Token() string {
	kc.tokenMu.RLock()
	defer kc.tokenMu.RUnlock()

	return kc.token
}

func (kc *DefaultClient) APIServerURL() string {
	return kc.apiServerURL
}
