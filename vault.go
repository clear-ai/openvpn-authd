/*

Copyright 2015 All rights reserved.
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

package main

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/hashicorp/vault/api"
	"net/http"
	"io/ioutil"
	"encoding/json"
)

type vault struct {
	// the vault client
	client *api.Client
}

// NewVaultClient creates a new vault client
func NewVaultClient(vaultURL, caFile string, skipTLSVerify bool) (VaultService, error) {
	var err error

	config := api.DefaultConfig()
	config.Address = vaultURL
	config.HttpClient.Transport, err = buildTransport(skipTLSVerify, caFile)
	if err != nil {
		return nil, err
	}

	client, err := api.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &vault{
		client: client,
	}, nil
}

// Authenticate retrieves a client token for the user and set the client
func (r *vault) Authenticate(username, password string) error {
	glog.V(3).Infof("authentication the client to vault service, username: %s", username)

	// step: create the payload
	var userPassLogin struct {
		// the password for the account
		Password string `json:"password,omitempty"`
	}
	userPassLogin.Password = password

	// step: create the token request
	request := r.client.NewRequest("POST", fmt.Sprintf("/v1/auth/userpass/login/%s", username))
	if err := request.SetJSONBody(&userPassLogin); err != nil {
		return err
	}

	// step: make the request
	resp, err := r.client.RawRequest(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// step: parse and return auth
	secret, err := api.ParseSecret(resp.Body)
	if err != nil {
		return err
	}

	glog.V(10).Infof("setting the vault token to: %s", secret.Auth.ClientToken)
	r.client.SetToken(secret.Auth.ClientToken)

	return nil
}

// GetCertificate retrieves a certificate from vault
func (r *vault) GetCertificate(subject, path string, ttl time.Duration) (*Certificate, error) {
	glog.Infof("attempting to issue a certificate for subject: %s, path: %s, ttl: %s", subject, path, ttl)
	// step: issue the
	secret, err := r.client.Logical().Write(fmt.Sprintf(path),
		map[string]interface{}{
			"common_name": subject,
			"ttl":         ttl.String(),
		})
	if err != nil {
		return nil, err
	}

	// add the full ca chain
	var caChain = secret.Data["ca_chain"].([]interface{})[0].(string) + "\n" + secret.Data["ca_chain"].([]interface{})[1].(string)

	// step: package the certificate
	return &Certificate{
		Certificate: secret.Data["certificate"].(string),
		PrivateKey:  secret.Data["private_key"].(string),
		IssuingCA:   caChain,
		TTL:         ttl,
		Subject:     subject,
	}, nil
}

func GetVaultHealth(r *openvpnAuthd) (*HealthResponse,error){
	glog.Info("Getting health for vault")

	path := r.config.VaultURL + "/v1/sys/health"

	res, err := http.Get(path)
	var s = new(HealthResponse)

	if err != nil {
		return s, err
	}

	body, err := ioutil.ReadAll(res.Body)
	err = json.Unmarshal(body, s)

	if err != nil {
		return s, err
	}

	defer res.Body.Close()

	return s, nil
}
type HealthResponse struct {
	VaultInitialized bool `json:"initialized"`
	VaultSealed bool `json:"sealed"`
	VaultStandby bool `json:"standby"`
	VaultReplicationPerformanceMode string `json:"replication_performance_mode"`
	VaultReplicationDrMode string `json:"replication_dr_mode"`
	VaultServerTimeUtc int `json:"server_time_utc"`
	VaultVersion string `json:"version"`
	VaultClusterId string `json:"cluster_id"`
	VaultClusterName string `json:"cluster_name"`
	AppVersion string `json:"app_version"`
}