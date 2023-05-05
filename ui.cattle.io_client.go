package main

import (
	"net/http"

	v1 "github.com/rancher/rancher/pkg/apis/ui.cattle.io/v1"
	rest "k8s.io/client-go/rest"
)

type UiV1Interface interface {
	RESTClient() rest.Interface
	NavLinksGetter
}

type NavlinkExpansion interface{}

// UiV1Client is used to interact with features provided by the ui.cattle.io group.
type UiV1Client struct {
	restClient rest.Interface
}

func (c *UiV1Client) Navlinks(namespace string) NavLinkInterface {
	return newNavLinks(c, namespace)
}

// NewForConfig creates a new UiV1Client for the given config.
// NewForConfig is equivalent to NewForConfigAndClient(c, httpClient),
// where httpClient was generated with rest.HTTPClientFor(c).
func NewForConfig(c *rest.Config) (*UiV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	httpClient, err := rest.HTTPClientFor(&config)
	if err != nil {
		return nil, err
	}
	return NewForConfigAndClient(&config, httpClient)
}

// NewForConfigAndClient creates a new UiV1Client for the given config and http client.
// Note the http client provided takes precedence over the configured transport values.
func NewForConfigAndClient(c *rest.Config, h *http.Client) (*UiV1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientForConfigAndClient(&config, h)
	if err != nil {
		return nil, err
	}
	return &UiV1Client{client}, nil
}

// NewForConfigOrDie creates a new UiV1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *UiV1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new UiV1Client for the given RESTClient.
func New(c rest.Interface) *UiV1Client {
	return &UiV1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *UiV1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}