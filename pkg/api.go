package client

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/EmilyShepherd/k8s-client-go/types"
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

func NewObjectAPI[T interface{}](kc Interface, gvr types.GroupVersionResource, opt ...ObjectAPIOption) types.ObjectAPI[T] {
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
	gvr  types.GroupVersionResource
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

func (o *objectAPI[T]) do(verb, namespace, name, urlExtra string) (*http.Response, error) {
	reqURL := o.buildRequestURL(namespace, name) + urlExtra
	req, err := http.NewRequest(verb, reqURL, nil)
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

	return resp, nil
}

func (o *objectAPI[T]) Get(namespace, name string, opts types.GetOptions) (*T, error) {
	resp, err := o.do("GET", namespace, name, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var t T
	if err := o.opts.responseDecodeFunc(resp.Body).Decode(&t); err != nil {
		return nil, err
	}
	return &t, err
}

func (o *objectAPI[T]) Watch(namespace, name string, opts types.ListOptions) (types.WatchInterface[T], error) {
	resp, err := o.do("GET", namespace, name, "?watch")
	if err != nil {
		return nil, err
	}
	return newStreamWatcher[T](resp.Body, o.opts.log, o.opts.responseDecodeFunc(resp.Body)), nil
}
