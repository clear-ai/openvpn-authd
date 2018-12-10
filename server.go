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
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"os"
	"text/template"
	"time"
)

type openvpnAuthd struct {
	config *AuthConfig
	// the gin service
	router *gin.Engine
	// the vault api client
	vault VaultService
}

// NewOpenVPNAuthd creates a new service for shipping out the openvpn config
func NewOpenVPNAuthd(cfg *AuthConfig) (OpenVPNAuthd, error) {
	//var err error

	glog.Infof("creating a new openvpn authd service, config: %s", cfg)

	service := new(openvpnAuthd)
	service.config = cfg

	// step: create the vault client
	glog.V(3).Infof("creating the vault client, address: %s, username: %s", cfg.VaultURL, cfg.VaultUsername)
	client, err := NewVaultClient(cfg.VaultURL, cfg.VaultCaFile, cfg.VaultTLSVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create a vault client, error: %s", err)
	}
	service.vault = client

	// step: attempt to authenticate to vault
	err = client.Authenticate(cfg.VaultUsername, cfg.VaultPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate to vault, error: %s", err)
	}

	// step: create the gin router
	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.LoadHTMLGlob("templates/*")
	router.GET("/health", service.healthHandler)
	router.GET("/", service.openVPNHandler)

	service.router = router

	return service, nil
}

// Run start the service
func (r *openvpnAuthd) Run() error {
	go func() {
		listener := fmt.Sprintf("%s:%d", r.config.ServiceBind, r.config.ServicePort)
		err := r.router.Run(listener)
		if err != nil {
			glog.Fatalf("failed to start the service, error: %s", err)
		}
	}()

	return nil
}

// openVPNHandler generate the openvpn config
func (r *openvpnAuthd) openVPNHandler(cx *gin.Context) {
	// step: grab the authentication headers
	emailAddress := cx.Request.Header.Get(r.config.AuthHeader)
	if emailAddress == "" {
		cx.AbortWithStatus(http.StatusForbidden)
		return
	}

	// step: generate a certificate for them
	cert, err := r.vault.GetCertificate(emailAddress, r.config.VaultPath, r.config.SessionDuration)
	if err != nil {
		glog.Errorf("failed to generate the certificate for openvpn account, reason: %s", err)
		cx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// step: are we loading a openvpn tls auth file
	if r.config.OpenVPNtlsAuth != "" {
		tlsAuth, err := ioutil.ReadFile(r.config.OpenVPNtlsAuth)
		if err != nil {
			glog.Errorf("unable to read in the tlsAuth file: %s, reason: %s", r.config.OpenVPNtlsAuth, err)
			return
		}

		file := gin.H{
			"openvpn_servers": r.config.servers,
			"ttl":             cert.TTL,
			"expires_in":      time.Now().Add(cert.TTL),
			"certificate":     cert.Certificate,
			"private_key":     cert.PrivateKey,
			"issuing_ca":      cert.IssuingCA,
			"email":           emailAddress,
			"tlsauth":         string(tlsAuth),
		}

		templateFile(file, cx)

	} else {
		file := gin.H{
			"openvpn_servers": r.config.servers,
			"ttl":             cert.TTL,
			"expires_in":      time.Now().Add(cert.TTL),
			"certificate":     cert.Certificate,
			"private_key":     cert.PrivateKey,
			"issuing_ca":      cert.IssuingCA,
			"email":           emailAddress,
		}

		templateFile(file, cx)

	}
}

func (r *openvpnAuthd) healthHandler(cx *gin.Context) {
	res, err := GetVaultHealth(r)
	if err != nil {
		cx.String(http.StatusInternalServerError, err.Error())
		return
	}
	res.AppVersion = Version
	cx.JSON(http.StatusOK, res)
}

func templateFile(d gin.H, cx *gin.Context) {
	t, err := template.ParseFiles("templates/openvpn.tmpl")
	if err != nil {
		glog.Error(err)
		cx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	var fi bytes.Buffer
	t.Execute(&fi, d)

	writeVPNFile(fi.String(), cx)

}
func writeVPNFile(d string, cx *gin.Context) {

	dir, err := ioutil.TempDir("./static/", "")
	if err != nil {
		glog.Error(err)
		cx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	tmpfile, err := ioutil.TempFile(dir, "certfile")
	if err != nil {
		glog.Error(err)
		cx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	defer os.Remove(dir)            // Clean up
	defer os.Remove(tmpfile.Name()) // Clean up

	_, err = tmpfile.WriteString(d)

	if err != nil {
		glog.Error(err)
		cx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if err := tmpfile.Close(); err != nil {
		glog.Error(err)
		cx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	cx.File(tmpfile.Name())

}
