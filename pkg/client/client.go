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

type Client struct {
	HttpClient   *http.Client
	apiServerURL string

	token token.TokenProvider
}

const (
	serviceAccountToken  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	serviceAccountCACert = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// NewInCluster creates Client if it is inside Kubernetes.
func NewInCluster() (*Client, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, fmt.Errorf("unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
	}
	tp, err := token.NewFileToken(serviceAccountToken)
	if err != nil {
		return nil, err
	}
	ca, err := ioutil.ReadFile(serviceAccountCACert)
	if err != nil {
		return nil, err
	}

	return NewClient("https://"+net.JoinHostPort(host, port), tp, ca)
}

func NewClient(host string, tp token.TokenProvider, ca []byte) (*Client, error) {
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(ca)
	transport := &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    certPool,
	}}

	return &Client{
		apiServerURL: host,
		token:        tp,
		HttpClient: &http.Client{
			Transport: transport,
			Timeout:   time.Nanosecond * 0,
		},
	}, nil
}

func (kc *Client) DoRaw(req *http.Request) (*http.Response, error) {
	if token := kc.token.Token(); len(token) > 0 {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return kc.HttpClient.Do(req)
}

func (kc *Client) Do(r ResourceRequest) (*http.Response, error) {
	req, err := http.NewRequest(r.Verb, kc.apiServerURL+r.URL(), r.Body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json,application/vnd.kubernetes.protobuf")

	if r.ContentType != "" {
		req.Header.Set("Content-Type", string(r.ContentType))
	}

	resp, err := kc.DoRaw(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 226 {
		defer resp.Body.Close()
		errmsg, _ := ioutil.ReadAll(resp.Body)
		return resp, fmt.Errorf("invalid response code %d for request url : %s", resp.StatusCode, errmsg)
	}

	return resp, nil
}
