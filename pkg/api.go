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
var MergePatchHeader = Header{
	Name:  "content-type",
	Value: "application/merge-patch+json",
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
	kc          Interface
	opts        objectAPIOptions
	gvr         types.GroupVersionResource
	subresource string
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
	url := o.kc.APIServerURL() + "/" + path.Join(gvrPath, nsPath, o.gvr.Resource, r.Name, o.subresource)

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

func (o *objectAPI[T]) Subresource(subresource string) types.ObjectAPI[T] {
	newO := *o
	newO.subresource = subresource

	return &newO
}

func (o *objectAPI[T]) Status() types.ObjectAPI[T] {
	return o.Subresource("status")
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
	if resp.StatusCode < 200 || resp.StatusCode > 226 {
		defer resp.Body.Close()
		errmsg, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("invalid response code %d for request url %q: %s", resp.StatusCode, reqURL, errmsg)
	}

	return resp, nil
}

func (o *objectAPI[T]) doAndUnmarshal(item interface{}, req ResourceRequest, headers ...Header) error {
	resp, err := o.do(req, headers...)
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
	return &t, o.doAndUnmarshal(&t, ResourceRequest{
		Namespace: namespace,
		Name:      name,
	})
}

func (o *objectAPI[T]) List(namespace string, opts types.ListOptions) (*types.List[T], error) {
	extra := make([]string, len(opts.LabelSelector))
	for _, label := range opts.LabelSelector {
		if label.Operator == types.Exists {
			extra = append(extra, "labelSelector="+label.Label)
		} else {
			extra = append(extra, "labelSelector="+label.Label+label.Operator+label.Value)
		}
	}

	var t types.List[T]
	return &t, o.doAndUnmarshal(&t, ResourceRequest{
		Namespace: namespace,
		Extra:     extra,
	})
}

func (o *objectAPI[T]) Create(namespace string, item T) (*T, error) {
	s, _ := json.Marshal(item)
	var t T
	return &t, o.doAndUnmarshal(&t, ResourceRequest{
		Verb:      "POST",
		Namespace: namespace,
		Body:      bytes.NewReader(s),
	})
}

func (o *objectAPI[T]) patch(namespace, name, fieldManager string, force bool, h Header, item T) (*T, error) {
	s, _ := json.Marshal(item)

	extra := []string{"fieldManager=" + fieldManager}
	if force {
		extra = append(extra, "force")
	}

	var t T
	return &t, o.doAndUnmarshal(&t, ResourceRequest{
		Verb:      "PATCH",
		Namespace: namespace,
		Name:      name,
		Extra:     extra,
		Body:      bytes.NewReader(s),
	}, h)
}

func (o *objectAPI[T]) Delete(namespace, name string, force bool) error {
	extra := []string{}
	if force {
		extra = append(extra, "force")
	}

	_, err := o.do(ResourceRequest{
		Verb:      "DELETE",
		Namespace: namespace,
		Name:      name,
		Extra:     extra,
	})

	return err
}

func (o *objectAPI[T]) Apply(namespace, name, fieldManager string, force bool, item T) (*T, error) {
	return o.patch(namespace, name, fieldManager, force, ApplyPatchHeader, item)
}

func (o *objectAPI[T]) Patch(namespace, name, fieldManager string, item T) (*T, error) {
	return o.patch(namespace, name, fieldManager, false, MergePatchHeader, item)
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
