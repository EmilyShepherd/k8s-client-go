package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/EmilyShepherd/k8s-client-go/pkg/token"
)

const (
	serviceAccountToken  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	serviceAccountCACert = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// Interface is minimal kubernetes Client interface.
type Interface interface {
	// Do sends HTTP request to ObjectAPI server.
	Do(req *http.Request) (*http.Response, error)
	// APIServerURL returns API server URL.
	APIServerURL() string
}

// NewInCluster creates Client if it is inside Kubernetes.
func NewInCluster() (*DefaultClient, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, fmt.Errorf("unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
	}
	token, err := token.NewFileToken(serviceAccountToken)
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

	return &DefaultClient{
		apiServerURL: "https://" + net.JoinHostPort(host, port),
		token:        token,
		HttpClient:   httpClient,
	}, nil
}

type DefaultClient struct {
	HttpClient *http.Client
	//Logger     Logger
	apiServerURL string

	token token.TokenProvider
}

func (kc *DefaultClient) Do(req *http.Request) (*http.Response, error) {
	if token := kc.token.Token(); len(token) > 0 {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return kc.HttpClient.Do(req)
}

func (kc *DefaultClient) APIServerURL() string {
	return kc.apiServerURL
}
