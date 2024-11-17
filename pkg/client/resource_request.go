package client

import (
	"io"
	"net/url"
	"path"

	"github.com/EmilyShepherd/k8s-client-go/types"
)

type ContentType string

const (
	ApplyPatchContentType ContentType = "application/apply-patch+yaml"
	MergePatchContentType             = "application/merge-patch+json"
	JSONContentType                   = "application/json"
)

type ResourceRequest struct {
	GVR         types.GroupVersionResource
	Subresource string
	Verb        string
	Namespace   string
	Name        string
	ContentType ContentType
	Values      url.Values
	Body        io.Reader
}

func (r ResourceRequest) URL() string {
	var gvrPath string
	if r.GVR.Group == "" {
		gvrPath = path.Join("api", r.GVR.Version)
	} else {
		gvrPath = path.Join("apis", r.GVR.Group, r.GVR.Version)
	}
	var nsPath string
	if r.Namespace != "" {
		nsPath = path.Join("namespaces", r.Namespace)
	}
	url := "/" + path.Join(gvrPath, nsPath, r.GVR.Resource, r.Name, r.Subresource)

	if queryString := r.Values.Encode(); queryString != "" {
		url += "?" + queryString
	}

	return url
}
