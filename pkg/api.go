package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"

	metav1 "github.com/EmilyShepherd/k8s-client-go/types/meta/v1"
)

type ResponseDecoderFunc func(r io.Reader) ResponseDecoder

type ObjectAPIOption func(opts *objectAPIOptions)
type objectAPIOptions struct {
	log                Logger
	responseDecodeFunc ResponseDecoderFunc
}

func WithLogger(log Logger) ObjectAPIOption {
	return func(opts *objectAPIOptions) {
		opts.log = log
	}
}

func WithResponseDecoder(decoderFunc ResponseDecoderFunc) ObjectAPIOption {
	return func(opts *objectAPIOptions) {
		opts.responseDecodeFunc = decoderFunc
	}
}

func NewObjectAPI[T interface{}](kc Interface, gvr metav1.GroupVersionResource, opt ...ObjectAPIOption) ObjectAPI[T] {
	opts := objectAPIOptions{
		log: &DefaultLogger{},
		responseDecodeFunc: func(r io.Reader) ResponseDecoder {
			return json.NewDecoder(r)
		},
	}
	for _, o := range opt {
		o(&opts)
	}

	return &objectAPI[T]{
		kc:   kc,
		opts: opts,
		gvr:  gvr,
	}
}

type objectAPI[T interface{}] struct {
	kc   Interface
	opts objectAPIOptions
	gvr  metav1.GroupVersionResource
}

func (o *objectAPI[T]) buildRequestURL(namespace, name string) string {
	var gvrPath string
	if o.gvr.Group == "" {
		gvrPath = path.Join("api", o.gvr.Version)
	} else {
		gvrPath = path.Join("apis", o.gvr.Group, o.gvr.Version)
	}
	var nsPath string
	if namespace != "" {
		nsPath = path.Join("namespaces", namespace)
	}
	return o.kc.APIServerURL() + "/" + path.Join(gvrPath, nsPath, o.gvr.Resource, name)
}

func (o *objectAPI[T]) Get(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*T, error) {
	var t T
	reqURL := o.buildRequestURL(namespace, name)
	req, err := o.getRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	resp, err := o.kc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errmsg, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("invalid response code %d for request url %q: %s", resp.StatusCode, reqURL, errmsg)
	}
	if err := o.opts.responseDecodeFunc(resp.Body).Decode(&t); err != nil {
		return nil, err
	}
	return &t, err
}

func (o *objectAPI[T]) Watch(ctx context.Context, namespace, name string, opts metav1.ListOptions) (WatchInterface[T], error) {
	reqURL := o.buildRequestURL(namespace, name) + "?watch"
	req, err := o.getRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	resp, err := o.kc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errmsg, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("invalid response code %d for request url %q: %s", resp.StatusCode, reqURL, errmsg)
	}
	return newStreamWatcher[T](resp.Body, o.opts.log, o.opts.responseDecodeFunc(resp.Body)), nil
}

func (o *objectAPI[T]) getRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if token := o.kc.Token(); len(token) > 0 {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}
