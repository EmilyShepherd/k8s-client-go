package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"github.com/EmilyShepherd/k8s-client-go/types"
)

type Header struct {
	Name  string
	Value string
}

var ApplyPatchHeader = Header{
	Name:  "content-type",
	Value: "application/apply-patch+yaml",
}

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

func (o *objectAPI[T]) buildRequestURL(r ResourceRequest) string {
	var gvrPath string
	if o.gvr.Group == "" {
		gvrPath = path.Join("api", o.gvr.Version)
	} else {
		gvrPath = path.Join("apis", o.gvr.Group, o.gvr.Version)
	}
	var nsPath string
	if r.Namespace != "" {
		nsPath = path.Join("namespaces", r.Namespace)
	}
	url := o.kc.APIServerURL() + "/" + path.Join(gvrPath, nsPath, o.gvr.Resource, r.Name)

	if len(r.Extra) > 0 {
		url += "?" + strings.Join(r.Extra, "&")
	}

	return url
}

type ResourceRequest struct {
	Verb      string
	Namespace string
	Name      string
	Extra     []string
	Body      io.Reader
}

func (o *objectAPI[T]) do(r ResourceRequest, headers ...Header) (*http.Response, error) {
	reqURL := o.buildRequestURL(r)
	req, err := http.NewRequest(r.Verb, reqURL, r.Body)
	if err != nil {
		return nil, err
	}
	for _, header := range headers {
		req.Header.Set(header.Name, header.Value)
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

func (o *objectAPI[T]) getAndUnmarshal(item interface{}, namespace, name string) error {
	resp, err := o.do(ResourceRequest{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := o.opts.responseDecodeFunc(resp.Body).Decode(item); err != nil {
		return err
	}
	return err
}

func (o *objectAPI[T]) Get(namespace, name string, opts types.GetOptions) (*T, error) {
	var t T
	if err := o.getAndUnmarshal(&t, namespace, name); err != nil {
		return nil, err
	}
	return &t, nil
}

func (o *objectAPI[T]) List(namespace string, opts types.ListOptions) (*types.List[T], error) {
	var t types.List[T]
	if err := o.getAndUnmarshal(&t, namespace, ""); err != nil {
		return nil, err
	}
	return &t, nil
}

func (o *objectAPI[T]) Apply(namespace, name, fieldManager string, force bool, item T) (*T, error) {
	s, _ := json.Marshal(item)

	extra := []string{"fieldManager=" + fieldManager}
	if force {
		extra = append(extra, "force")
	}

	resp, err := o.do(ResourceRequest{
		Verb:      "PATCH",
		Namespace: namespace,
		Name:      name,
		Extra:     extra,
		Body:      bytes.NewReader(s),
	}, ApplyPatchHeader)
	if err != nil {
		return nil, err
	}

	var t T
	if err := o.opts.responseDecodeFunc(resp.Body).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (o *objectAPI[T]) Watch(namespace, name string, opts types.ListOptions) (types.WatchInterface[T], error) {
	resp, err := o.do(ResourceRequest{
		Namespace: namespace,
		Name:      name,
		Extra:     []string{"watch"},
	})
	if err != nil {
		return nil, err
	}
	return newStreamWatcher[T](resp.Body, o.opts.log, o.opts.responseDecodeFunc(resp.Body)), nil
}
