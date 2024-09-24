package apis

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"github.com/EmilyShepherd/k8s-client-go/pkg"
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

func NewObjectAPI[T any, PT types.Object[T]](kc client.Interface, gvr types.GroupVersionResource, opt ...ObjectAPIOption) types.ObjectAPI[T, PT] {
	opts := objectAPIOptions{
		log: &DefaultLogger{},
		responseDecodeFunc: func(r io.Reader) ResponseDecoder {
			return json.NewDecoder(r)
		},
	}
	for _, o := range opt {
		o(&opts)
	}

	return &objectAPI[T, PT]{
		kc:   kc,
		opts: opts,
		gvr:  gvr,
	}
}

// The reason we need to use the truly ugly [T, PT interface{ *T }]
// nonsense is that we want the API to be compatible with k8s' standard
// library resources, which use pointer receivers for their `GetKind()`
// style methods. Types with pointer receivers need to be explicitly
// given as a pointer type for Golang generics, however we _also_ need
// the base type so we can create new variables.
//
// For example, given the setup:
//
// ```golang
//
//	type Object struct {
//	  GetKind()
//	}
//
// type ObjectAPI[T Object] interface { ... }
// ```
//
// If we were to try to create a new API for Pods:
//
// ```golang
// NewObjectAPI[corev1.Pod](...)
// ```
//
// This will fail because Pods pointer receivers for `GetKind()`. We
// could fix this issue by requiring the caller to pass the pointer type
// instead, so the format would be:
//
// ```golang
// NewObjectAPI[*corev1.Pod](...)
// ```
//
// However this now means that we don't have access to the base type
// within the library so statements like this won't work:
//
// ```golang
// var t T // T is a pointer to a type, not a type, so it'll be nil
// ```
//
// Golang does not make it easy to convert between them without _very
// explicitly_ defining an interface that requires itself to use pointer
// receivers for itself:
//
// ```golang
//
//	type Object[T any] struct {
//	  GetKind()
//	  *T
//	}
//
// ```
//
// We then have to do compiler shenanigans to get access to both the
// base type and have the type checker verify that the pointer type has
// the methods we need.
//
// ```golang
// type ObjectAPI[T any, PT Object[T]]
// ```
//
// Finally, this gives us access to the `T` which is the base type and
// `PT` which is the pointer type which can be used to call methods:
//
// ```golang
// var t T
// PT(&t).GetKind()
// ```
//
// Strangely, the go compiler even lets us use a syntactic shorthand and
// still only specify:
//
// ```golang
// NewObjectAPI[corev1.Pod]
// ```
//
// Which it will translate under the hood to:
//
// ```golang
// NewObjectAPI[corev1.Pod, Object[corev1.Pod]]
// ```
//
// More info here: https://go.googlesource.com/proposal/+/refs/heads/master/design/43651-type-parameters.md#pointer-method-example
type objectAPI[T any, PT types.Object[T]] struct {
	kc          client.Interface
	opts        objectAPIOptions
	gvr         types.GroupVersionResource
	subresource string
}

func (o *objectAPI[T, PT]) buildRequestURL(r ResourceRequest) string {
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

func (o *objectAPI[T, PT]) Subresource(subresource string) types.ObjectAPI[T, PT] {
	newO := *o
	newO.subresource = subresource

	return &newO
}

func (o *objectAPI[T, PT]) Status() types.ObjectAPI[T, PT] {
	return o.Subresource("status")
}

func (o *objectAPI[T, PT]) do(r ResourceRequest, headers ...Header) (*http.Response, error) {
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
		return resp, fmt.Errorf("invalid response code %d for request url %q: %s", resp.StatusCode, reqURL, errmsg)
	}

	return resp, nil
}

func (o *objectAPI[T, PT]) doAndUnmarshal(item interface{}, req ResourceRequest, headers ...Header) (*http.Response, error) {
	resp, err := o.do(req, headers...)
	if err != nil {
		return resp, err
	}

	defer resp.Body.Close()

	err = o.opts.responseDecodeFunc(resp.Body).Decode(item)

	return resp, err
}

func (o *objectAPI[T, PT]) doAndUnmarshalItem(req ResourceRequest) (T, error) {
	var t T
	_, err := o.doAndUnmarshal(&t, req)
	return t, err
}

func (o *objectAPI[T, PT]) Get(namespace, name string, opts types.GetOptions) (T, error) {
	return o.doAndUnmarshalItem(ResourceRequest{
		Namespace: namespace,
		Name:      name,
	})
}

func (o *objectAPI[T, PT]) List(namespace string, opts types.ListOptions) (*types.List[T, PT], error) {
	extra := make([]string, len(opts.LabelSelector))
	for _, label := range opts.LabelSelector {
		if label.Operator == types.Exists {
			extra = append(extra, "labelSelector="+label.Label)
		} else {
			extra = append(extra, "labelSelector="+label.Label+label.Operator+label.Value)
		}
	}

	var t types.List[T, PT]
	_, err := o.doAndUnmarshal(&t, ResourceRequest{
		Namespace: namespace,
		Extra:     extra,
	})
	return &t, err
}

func (o *objectAPI[T, PT]) Create(namespace string, item T) (T, error) {
	s, _ := json.Marshal(item)
	return o.doAndUnmarshalItem(ResourceRequest{
		Verb:      "POST",
		Namespace: namespace,
		Body:      bytes.NewReader(s),
	})
}

func (o *objectAPI[T, PT]) patch(namespace, name, fieldManager string, force bool, h Header, item T) (T, *http.Response, error) {
	s, _ := json.Marshal(item)

	extra := []string{"fieldManager=" + fieldManager}
	if force {
		extra = append(extra, "force")
	}

	var t T

	err, resp := o.doAndUnmarshal(&t, ResourceRequest{
		Verb:      "PATCH",
		Namespace: namespace,
		Name:      name,
		Extra:     extra,
		Body:      bytes.NewReader(s),
	}, h)

	return t, err, resp
}

func (o *objectAPI[T, PT]) Delete(namespace, name string, force bool) (T, error) {
	extra := []string{}
	if force {
		extra = append(extra, "force")
	}

	return o.doAndUnmarshalItem(ResourceRequest{
		Verb:      "DELETE",
		Namespace: namespace,
		Name:      name,
		Extra:     extra,
	})
}

func (o *objectAPI[T, PT]) Apply(namespace, name, fieldManager string, force bool, item T) (T, error, types.EventType) {
	t, resp, err := o.patch(namespace, name, fieldManager, force, ApplyPatchHeader, item)

	var eventType types.EventType
	if resp.StatusCode == 201 {
		eventType = types.EventTypeAdded
	} else {
		eventType = types.EventTypeModified
	}

	return t, err, eventType
}

func (o *objectAPI[T, PT]) Patch(namespace, name, fieldManager string, item T) (T, error) {
	t, _, err := o.patch(namespace, name, fieldManager, false, MergePatchHeader, item)
	return t, err
}

func (o *objectAPI[T, PT]) Watch(namespace, name string, opts types.ListOptions) (types.WatchInterface[T, PT], error) {
	extra := []string{"watch"}
	if opts.ResourceVersion != "" {
		extra = append(extra, "resourceVersion="+opts.ResourceVersion)
	}
	for _, label := range opts.LabelSelector {
		if label.Operator == types.Exists {
			extra = append(extra, "labelSelector="+label.Label)
		} else {
			extra = append(extra, "labelSelector="+label.Label+label.Operator+label.Value)
		}
	}
	resp, err := o.do(ResourceRequest{
		Namespace: namespace,
		Name:      name,
		Extra:     extra,
	})
	if err != nil {
		return nil, err
	}
	return newStreamWatcher[T, PT](resp.Body, o.opts.log, o.opts.responseDecodeFunc(resp.Body)), nil
}
