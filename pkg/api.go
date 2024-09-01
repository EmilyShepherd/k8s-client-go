package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"

	corev1 "github.com/EmilyShepherd/k8s-client-go/types/core/v1"
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

func NewObjectAPI[T corev1.Object](kc Interface, opt ...ObjectAPIOption) ObjectAPI[T] {
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
	}
}

type objectAPI[T corev1.Object] struct {
	kc   Interface
	opts objectAPIOptions
}

func buildRequestURL(apiServerURL string, gvr metav1.GroupVersionResource, namespace, name string) string {
	var gvrPath string
	if gvr.Group == "" {
		gvrPath = path.Join("api", gvr.Version)
	} else {
		gvrPath = path.Join("apis", gvr.Group, gvr.Version)
	}
	var nsPath string
	if namespace != "" {
		nsPath = path.Join("namespaces", namespace)
	}
	return apiServerURL + "/" + path.Join(gvrPath, nsPath, gvr.Resource, name)
}

func (o *objectAPI[T]) Get(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*T, error) {
	var t T
	reqURL := buildRequestURL(o.kc.APIServerURL(), t.GVR(), namespace, name)
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
	var t T
	reqURL := buildRequestURL(o.kc.APIServerURL(), t.GVR(), namespace, name)
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
