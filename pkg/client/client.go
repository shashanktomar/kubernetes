/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// ClientInterface holds the methods for clients of Kubenetes, an interface to allow mock testing
type ClientInterface interface {
	ListPods(selector map[string]string) (api.PodList, error)
	GetPod(name string) (api.Pod, error)
	DeletePod(name string) error
	CreatePod(api.Pod) (api.Pod, error)
	UpdatePod(api.Pod) (api.Pod, error)

	GetReplicationController(name string) (api.ReplicationController, error)
	CreateReplicationController(api.ReplicationController) (api.ReplicationController, error)
	UpdateReplicationController(api.ReplicationController) (api.ReplicationController, error)
	DeleteReplicationController(string) error

	GetService(name string) (api.Service, error)
	CreateService(api.Service) (api.Service, error)
	UpdateService(api.Service) (api.Service, error)
	DeleteService(string) error
}

// AuthInfo is used to store authorization information
type AuthInfo struct {
	User     string
	Password string
}

// Client is the actual implementation of a Kubernetes client.
// Host is the http://... base for the URL
type Client struct {
	host       string
	auth       *AuthInfo
	httpClient *http.Client
}

// Create a new client object.
func New(host string, auth *AuthInfo) *Client {
	return &Client{
		auth: auth,
		host: host,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		},
	}
}

// Execute a request, adds authentication (if auth != nil), and HTTPS cert ignoring.
func (c *Client) doRequest(request *http.Request) ([]byte, error) {
	if c.auth != nil {
		request.SetBasicAuth(c.auth.User, c.auth.Password)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return []byte{}, err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return body, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode > http.StatusPartialContent {
		return nil, fmt.Errorf("request [%#v] failed (%d) %s: %s", request, response.StatusCode, response.Status, string(body))
	}
	return body, err
}

// Underlying base implementation of performing a request.
// method is the HTTP method (e.g. "GET")
// path is the path on the host to hit
// requestBody is the body of the request. Can be nil.
// target the interface to marshal the JSON response into.  Can be nil.
func (c *Client) rawRequest(method, path string, requestBody io.Reader, target interface{}) ([]byte, error) {
	request, err := http.NewRequest(method, c.makeURL(path), requestBody)
	if err != nil {
		return []byte{}, err
	}
	body, err := c.doRequest(request)
	if err != nil {
		return body, err
	}
	if target != nil {
		err = api.DecodeInto(body, target)
	}
	if err != nil {
		log.Printf("Failed to parse: %s\n", string(body))
		// FIXME: no need to return err here?
	}
	return body, err
}

func (client Client) makeURL(path string) string {
	return client.host + "/api/v1beta1/" + path
}

// EncodeSelector transforms a selector expressed as a key/value map, into a
// comma separated, key=value encoding.
func EncodeSelector(selector map[string]string) string {
	parts := make([]string, 0, len(selector))
	for key, value := range selector {
		parts = append(parts, key+"="+value)
	}
	return url.QueryEscape(strings.Join(parts, ","))
}

// DecodeSelector transforms a selector from a comma separated, key=value format into
// a key/value map.
func DecodeSelector(selector string) map[string]string {
	result := map[string]string{}
	if len(selector) == 0 {
		return result
	}
	parts := strings.Split(selector, ",")
	for _, part := range parts {
		pieces := strings.Split(part, "=")
		if len(pieces) == 2 {
			result[pieces[0]] = pieces[1]
		} else {
			log.Printf("Invalid selector: %s", selector)
		}
	}
	return result
}

// ListPods takes a selector, and returns the list of pods that match that selector
func (client Client) ListPods(selector map[string]string) (api.PodList, error) {
	path := "pods"
	if selector != nil && len(selector) > 0 {
		path += "?labels=" + EncodeSelector(selector)
	}
	var result api.PodList
	_, err := client.rawRequest("GET", path, nil, &result)
	return result, err
}

// GetPod takes the name of the pod, and returns the corresponding Pod object, and an error if it occurs
func (client Client) GetPod(name string) (api.Pod, error) {
	var result api.Pod
	_, err := client.rawRequest("GET", "pods/"+name, nil, &result)
	return result, err
}

// DeletePod takes the name of the pod, and returns an error if one occurs
func (client Client) DeletePod(name string) error {
	_, err := client.rawRequest("DELETE", "pods/"+name, nil, nil)
	return err
}

// CreatePod takes the representation of a pod.  Returns the server's representation of the pod, and an error, if it occurs
func (client Client) CreatePod(pod api.Pod) (api.Pod, error) {
	var result api.Pod
	body, err := json.Marshal(pod)
	if err == nil {
		_, err = client.rawRequest("POST", "pods", bytes.NewBuffer(body), &result)
	}
	return result, err
}

// UpdatePod takes the representation of a pod to update.  Returns the server's representation of the pod, and an error, if it occurs
func (client Client) UpdatePod(pod api.Pod) (api.Pod, error) {
	var result api.Pod
	body, err := json.Marshal(pod)
	if err == nil {
		_, err = client.rawRequest("PUT", "pods/"+pod.ID, bytes.NewBuffer(body), &result)
	}
	return result, err
}

// GetReplicationController returns information about a particular replication controller
func (client Client) GetReplicationController(name string) (api.ReplicationController, error) {
	var result api.ReplicationController
	_, err := client.rawRequest("GET", "replicationControllers/"+name, nil, &result)
	return result, err
}

// CreateReplicationController creates a new replication controller
func (client Client) CreateReplicationController(controller api.ReplicationController) (api.ReplicationController, error) {
	var result api.ReplicationController
	body, err := json.Marshal(controller)
	if err == nil {
		_, err = client.rawRequest("POST", "replicationControllers", bytes.NewBuffer(body), &result)
	}
	return result, err
}

// UpdateReplicationController updates an existing replication controller
func (client Client) UpdateReplicationController(controller api.ReplicationController) (api.ReplicationController, error) {
	var result api.ReplicationController
	body, err := json.Marshal(controller)
	if err == nil {
		_, err = client.rawRequest("PUT", "replicationControllers/"+controller.ID, bytes.NewBuffer(body), &result)
	}
	return result, err
}

func (client Client) DeleteReplicationController(name string) error {
	_, err := client.rawRequest("DELETE", "replicationControllers/"+name, nil, nil)
	return err
}

// GetReplicationController returns information about a particular replication controller
func (client Client) GetService(name string) (api.Service, error) {
	var result api.Service
	_, err := client.rawRequest("GET", "services/"+name, nil, &result)
	return result, err
}

// CreateReplicationController creates a new replication controller
func (client Client) CreateService(svc api.Service) (api.Service, error) {
	var result api.Service
	body, err := json.Marshal(svc)
	if err == nil {
		_, err = client.rawRequest("POST", "services", bytes.NewBuffer(body), &result)
	}
	return result, err
}

// UpdateReplicationController updates an existing replication controller
func (client Client) UpdateService(svc api.Service) (api.Service, error) {
	var result api.Service
	body, err := json.Marshal(svc)
	if err == nil {
		_, err = client.rawRequest("PUT", "services/"+svc.ID, bytes.NewBuffer(body), &result)
	}
	return result, err
}

func (client Client) DeleteService(name string) error {
	_, err := client.rawRequest("DELETE", "services/"+name, nil, nil)
	return err
}
