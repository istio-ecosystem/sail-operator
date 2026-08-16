// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helm

import (
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// restClientGetter is required by helm to instantiate ActionConfig
type restClientGetter struct {
	config             *rest.Config
	getDiscoveryClient func() (discovery.CachedDiscoveryInterface, error)
	getRESTMapper      func() (meta.RESTMapper, error)
}

func NewRESTClientGetter(config *rest.Config) genericclioptions.RESTClientGetter {
	c := &restClientGetter{config: config}
	c.getDiscoveryClient = sync.OnceValues(func() (discovery.CachedDiscoveryInterface, error) {
		cfg := rest.CopyConfig(config)
		cfg.Burst = 0
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
		if err != nil {
			return nil, err
		}
		return memory.NewMemCacheClient(discoveryClient), nil
	})
	c.getRESTMapper = sync.OnceValues(func() (meta.RESTMapper, error) {
		discoveryClient, err := c.getDiscoveryClient()
		if err != nil {
			return nil, err
		}
		mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
		return restmapper.NewShortcutExpander(mapper, discoveryClient, nil), nil
	})
	return c
}

func (c *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return c.config, nil
}

func (c *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return c.getDiscoveryClient()
}

func (c *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	return c.getRESTMapper()
}

func (c *restClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// use the standard defaults for this client command
	// DEPRECATED: remove and replace with something more accurate
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmd.ClusterDefaults}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
}
