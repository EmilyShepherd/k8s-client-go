package apis

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/EmilyShepherd/k8s-client-go/pkg/client"
	"github.com/EmilyShepherd/k8s-client-go/pkg/stream"
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

func NewObjectAPI[T any, PT types.Object[T]](kc *client.Client, gvr types.GroupVersionResource) types.ObjectAPI[T, PT] {
	return &objectAPI[T, PT]{
		kc:  kc,
		gvr: gvr,
		responseDecodeFunc: func(r io.Reader) ResponseDecoder {
			return json.NewDecoder(r)
		},
	}
}

type objectAPI[T any, PT types.Object[T]] struct {
	kc                 *client.Client
	responseDecodeFunc ResponseDecoderFunc
	gvr                types.GroupVersionResource
	subresource        string
}

func (o *objectAPI[T, PT]) Subresource(subresource string) types.ObjectAPI[T, PT] {
	newO := *o
	newO.subresource = subresource

	return &newO
}

func (o *objectAPI[T, PT]) Status() types.ObjectAPI[T, PT] {
	return o.Subresource("status")
}

func (o *objectAPI[T, PT]) doAndUnmarshal(item interface{}, req client.ResourceRequest, headers ...Header) (*http.Response, error) {
	req.GVR = o.gvr
	req.Subresource = o.subresource
	resp, err := o.kc.Do(req)
	if err != nil {
		return resp, err
	}

	defer resp.Body.Close()

	err = o.responseDecodeFunc(resp.Body).Decode(item)

	return resp, err
}

func (o *objectAPI[T, PT]) doAndUnmarshalItem(req client.ResourceRequest) (T, error) {
	var t T
	_, err := o.doAndUnmarshal(&t, req)
	return t, err
}

func (o *objectAPI[T, PT]) Get(namespace, name string, opts types.GetOptions) (T, error) {
	return o.doAndUnmarshalItem(client.ResourceRequest{
		Namespace: namespace,
		Name:      name,
	})
}

func (o *objectAPI[T, PT]) List(namespace string, opts types.ListOptions) (*types.List[T, PT], error) {
	q := url.Values{}
	for _, label := range opts.LabelSelector {
		if label.Operator == types.Exists {
			q.Add("labelSelector=", label.Label)
		} else {
			q.Add("labelSelector=", label.Label+label.Operator+label.Value)
		}
	}

	var t types.List[T, PT]
	_, err := o.doAndUnmarshal(&t, client.ResourceRequest{
		Namespace: namespace,
		Values:    q,
	})
	return &t, err
}

func (o *objectAPI[T, PT]) Create(namespace string, item T) (T, error) {
	s, _ := json.Marshal(item)
	return o.doAndUnmarshalItem(client.ResourceRequest{
		Verb:      "POST",
		Namespace: namespace,
		Body:      bytes.NewReader(s),
	})
}

func (o *objectAPI[T, PT]) patch(namespace, name, fieldManager string, force bool, h Header, item T) (T, *http.Response, error) {
	s, _ := json.Marshal(item)

	q := url.Values{}
	q.Set("fieldManager", fieldManager)
	if force {
		q.Set("force", "1")
	}

	var t T

	err, resp := o.doAndUnmarshal(&t, client.ResourceRequest{
		Verb:      "PATCH",
		Namespace: namespace,
		Name:      name,
		Values:    q,
		Body:      bytes.NewReader(s),
	}, h)

	return t, err, resp
}

func (o *objectAPI[T, PT]) Delete(namespace, name string, force bool) (T, error) {
	q := url.Values{}
	if force {
		q.Set("force", "1")
	}

	return o.doAndUnmarshalItem(client.ResourceRequest{
		Verb:      "DELETE",
		Namespace: namespace,
		Name:      name,
		Values:    q,
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
	req := client.ResourceRequest{
		Namespace: namespace,
		Values:    make(url.Values, len(opts.LabelSelector)+1),
		GVR:       o.gvr,
	}
	req.Values.Set("watch", "1")
	for _, label := range opts.LabelSelector {
		if label.Operator == types.Exists {
			req.Values.Add("labelSelector", label.Label)
		} else {
			req.Values.Add("labelSelector", label.Label+label.Operator+label.Value)
		}
	}

	// Watching in kubernetes is a collection-level operation so it's not
	// possible to watch a single resource via its URL. However we can do
	// it via a fieldSelector on the resource name.
	if name != "" {
		req.Values.Add("fieldSelector", "metadata.name"+types.Equals+name)
	}

	watch := &Watcher[T, PT]{
		req:             req,
		api:             o.kc,
		resourceVersion: opts.ResourceVersion,
	}
	if err := watch.doWatch(); err != nil {
		return nil, err
	}

	return stream.NewAsyncStream[types.Event[T, PT]](watch), nil
}
