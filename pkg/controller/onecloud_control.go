// Copyright 2019 Yunion
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

package controller

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud/pkg/apis/monitor"
	"yunion.io/x/onecloud/pkg/httperrors"
	"yunion.io/x/onecloud/pkg/keystone/locale"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/k8sutil"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

var (
	SessionDebug       bool
	SyncUser           bool
	EtcdKeepFailedPods bool

	sessionLock sync.Mutex
)

func GetAuthURL(oc *v1alpha1.OnecloudCluster) string {
	keystoneSvcName := KeystoneComponentName(oc.GetName())
	return fmt.Sprintf("https://%s:%d/v3", keystoneSvcName, constants.KeystoneAdminPort)
}

type OnecloudRCAdminConfig struct {
	AuthURL       string
	Region        string
	Username      string
	Password      string
	DomainName    string
	ProjectName   string
	ProjectDomain string
	Insecure      bool
	Debug         bool
	Timeout       int
	CertFile      string
	KeyFile       string
}

func NewOnecloudRCAdminConfig(oc *v1alpha1.OnecloudCluster, debug bool) *OnecloudRCAdminConfig {
	return &OnecloudRCAdminConfig{
		AuthURL:       GetAuthURL(oc),
		Region:        oc.Spec.Region,
		Username:      constants.SysAdminUsername,
		Password:      oc.Spec.Keystone.BootstrapPassword,
		DomainName:    constants.DefaultDomain,
		ProjectName:   constants.SysAdminProject,
		ProjectDomain: "",
		Insecure:      true,
		Debug:         debug,
		Timeout:       600,
		CertFile:      "",
		KeyFile:       "",
	}
}

func (config *OnecloudRCAdminConfig) ToAuthInfo() *auth.AuthInfo {
	return &auth.AuthInfo{
		AuthUrl:       config.AuthURL,
		Domain:        config.DomainName,
		Username:      config.Username,
		Passwd:        config.Password,
		Project:       config.ProjectName,
		ProjectDomain: config.ProjectDomain,
	}
}

func newOnecloudClientToken(oc *v1alpha1.OnecloudCluster) (*mcclient.Client, mcclient.TokenCredential, error) {
	config := NewOnecloudRCAdminConfig(oc, SessionDebug)
	cli := mcclient.NewClient(
		config.AuthURL,
		config.Timeout,
		config.Debug,
		config.Insecure,
		config.CertFile,
		config.KeyFile,
	)
	token, err := cli.AuthenticateWithSource(
		config.Username,
		config.Password,
		config.DomainName,
		config.ProjectName,
		config.ProjectDomain,
		"operator",
	)
	return cli, token, err
}

func NewOnecloudSessionByToken(cli *mcclient.Client, region string, token mcclient.TokenCredential) (*mcclient.ClientSession, error) {
	session := cli.NewSession(
		context.Background(),
		region,
		"",
		constants.EndpointTypeInternal,
		token,
		"",
	)
	return session, nil
}

func NewOnecloudSimpleClientSession(oc *v1alpha1.OnecloudCluster) (*mcclient.ClientSession, error) {
	cli, token, err := newOnecloudClientToken(oc)
	if err != nil {
		return nil, err
	}
	token = mcclient.SimplifyToken(token)
	cli.SetServiceCatalog(nil)
	return NewOnecloudSessionByToken(cli, oc.Spec.Region, token)
}

func NewOnecloudClientSession(oc *v1alpha1.OnecloudCluster) (*mcclient.ClientSession, error) {
	cli, token, err := newOnecloudClientToken(oc)
	if err != nil {
		return nil, err
	}
	return NewOnecloudSessionByToken(cli, oc.Spec.Region, token)
}

type OnecloudControl struct {
	kubeCli clientset.Interface
}

func NewOnecloudControl(kubeCli clientset.Interface) *OnecloudControl {
	return &OnecloudControl{kubeCli}
}

func (w *OnecloudControl) NewWaiter(oc *v1alpha1.OnecloudCluster) onecloud.Waiter {
	sessionFactory := func() (*mcclient.ClientSession, error) {
		return NewOnecloudClientSession(oc)
	}
	return onecloud.NewOCWaiter(w.kubeCli, sessionFactory, 5*time.Minute, os.Stdout)
}

func (w *OnecloudControl) RunWithSession(oc *v1alpha1.OnecloudCluster, f func(s *mcclient.ClientSession) error) error {
	sessionLock.Lock()
	defer sessionLock.Unlock()

	config := NewOnecloudRCAdminConfig(oc, false)
	var s *mcclient.ClientSession
	var err error
	if !auth.IsAuthed() {
		s, err = w.getSession(oc)
		if err != nil {
			return err
		}
		auth.Init(config.ToAuthInfo(), false, true, "", "")
	} else {
		s = auth.GetAdminSession(context.Background(), oc.Spec.Region, "")
	}
	s.SetServiceUrl("identity", GetAuthURL(oc))
	if err := f(s); err != nil {
		auth.Init(config.ToAuthInfo(), false, true, "", "")
		newSession := auth.GetAdminSession(context.Background(), oc.Spec.Region, "")
		return f(newSession)
	}
	return nil
}

/*func (w *OnecloudControl) RunWithSessionNoEndpoints(oc *v1alpha1.OnecloudCluster, f func(s *mcclient.ClientSession) error) error {
	if !auth.IsAuthed() {
		s, err := w.getSessionNoEndpoints(oc)
		if err != nil {
			return errors.Wrap(err, "init admin session no endpoints")
		}
		auth.InitFromClientSession(s)
	}
	s := auth.GetAdminSession(context.Background(), oc.Spec.Region, "")
	if err := f(s); err != nil {
		newSession, sErr := w.getSessionNoEndpoints(oc)
		if err != nil {
			return errors.NewAggregate([]error{err, sErr})
		}
		return f(newSession)
	}
	return nil
}*/

func (w *OnecloudControl) getSession(oc *v1alpha1.OnecloudCluster) (*mcclient.ClientSession, error) {
	return NewOnecloudClientSession(oc)
}

func (w *OnecloudControl) getSessionNoEndpoints(oc *v1alpha1.OnecloudCluster) (*mcclient.ClientSession, error) {
	return NewOnecloudSimpleClientSession(oc)
}

type PhaseControl interface {
	Setup() error
	SystemInit(oc *v1alpha1.OnecloudCluster) error
}

type ComponentManager interface {
	RunWithSession(oc *v1alpha1.OnecloudCluster, f func(s *mcclient.ClientSession) error) error
	// RunWithSessionNoEndpoints(oc *v1alpha1.OnecloudCluster, f func(s *mcclient.ClientSession) error) error
	GetController() *OnecloudControl
	GetCluster() *v1alpha1.OnecloudCluster
	Keystone() PhaseControl
	KubeServer(nodeLister corelisters.NodeLister) PhaseControl
	Region() PhaseControl
	Glance() PhaseControl
	YunionAgent() PhaseControl
	Devtool() PhaseControl
	Monitor() PhaseControl
}

func (w *OnecloudControl) Components(oc *v1alpha1.OnecloudCluster) ComponentManager {
	return newComponents(w, oc)
}

type realComponent struct {
	controller *OnecloudControl
	oc         *v1alpha1.OnecloudCluster
}

func newComponents(controller *OnecloudControl, oc *v1alpha1.OnecloudCluster) ComponentManager {
	return &realComponent{
		controller: controller,
		oc:         oc,
	}
}

func (c *realComponent) GetController() *OnecloudControl {
	return c.controller
}

func (c *realComponent) GetCluster() *v1alpha1.OnecloudCluster {
	return c.oc
}

func (c *realComponent) RunWithSession(oc *v1alpha1.OnecloudCluster, f func(s *mcclient.ClientSession) error) error {
	return c.controller.RunWithSession(oc, f)
}

/*func (c *realComponent) RunWithSessionNoEndpoints(oc *v1alpha1.OnecloudCluster, f func(s *mcclient.ClientSession) error) error {
	return c.controller.RunWithSessionNoEndpoints(oc, f)
}*/

func (c *realComponent) Keystone() PhaseControl {
	return &keystoneComponent{newBaseComponent(c)}
}

func (c *realComponent) KubeServer(nodeLister corelisters.NodeLister) PhaseControl {
	return &kubeServerComponent{
		baseComponent: newBaseComponent(c),
		nodeLister:    nodeLister,
	}
}

func (c *realComponent) Region() PhaseControl {
	return &regionComponent{newBaseComponent(c)}
}

func (c *realComponent) Glance() PhaseControl {
	return &glanceComponent{newBaseComponent(c)}
}

func (c *realComponent) YunionAgent() PhaseControl {
	return &yunionagentComponent{newBaseComponent(c)}
}

func (c *realComponent) Devtool() PhaseControl {
	return &devtoolComponent{newBaseComponent(c)}
}

func (c *realComponent) Monitor() PhaseControl {
	return &monitorComponent{newBaseComponent(c)}
}

type baseComponent struct {
	manager ComponentManager
}

func newBaseComponent(manager ComponentManager) *baseComponent {
	return &baseComponent{manager: manager}
}

func (c *baseComponent) RunWithSession(f func(s *mcclient.ClientSession) error) error {
	return c.manager.RunWithSession(c.GetCluster(), f)
}

/*func (c *baseComponent) RunWithSessionNoEndpoints(f func(s *mcclient.ClientSession) error) error {
	return c.manager.RunWithSessionNoEndpoints(c.GetCluster(), f)
}*/

func (c *baseComponent) GetCluster() *v1alpha1.OnecloudCluster {
	return c.manager.GetCluster()
}

func (c *baseComponent) SystemInit(oc *v1alpha1.OnecloudCluster) error {
	return nil
}

type endpoint struct {
	Host      string
	Port      int
	Path      string
	Interface string
}

func newEndpointByInterfaceType(host string, port int, path string, infType string) *endpoint {
	return &endpoint{
		Host:      host,
		Port:      port,
		Path:      path,
		Interface: infType,
	}
}

func newPublicEndpoint(host string, port int, path string) *endpoint {
	return newEndpointByInterfaceType(host, port, path, constants.EndpointTypePublic)
}

func newInternalEndpoint(host string, port int, path string) *endpoint {
	return newEndpointByInterfaceType(host, port, path, constants.EndpointTypeInternal)
}

func (e endpoint) GetProtocolUrl(proto string) string {
	url := fmt.Sprintf("%s://%s:%d", proto, e.Host, e.Port)
	if e.Path != "" {
		url = fmt.Sprintf("%s/%s", url, e.Path)
	}
	return url
}

func (e endpoint) GetUrl(enableSSL bool) string {
	proto := "http"
	if enableSSL {
		proto = "https"
	}
	return e.GetProtocolUrl(proto)
}

func (c *baseComponent) RegisterCloudServiceEndpoint(
	cType v1alpha1.ComponentType,
	serviceName, serviceType string,
	port int, prefix string, enableSsl bool) error {
	oc := c.GetCluster()
	internalAddress := NewClusterComponentName(oc.GetName(), cType)
	publicAddress := oc.Spec.LoadBalancerEndpoint
	if publicAddress == "" {
		publicAddress = internalAddress
	}
	eps := []*endpoint{
		newPublicEndpoint(publicAddress, port, prefix),
		newInternalEndpoint(internalAddress, port, prefix),
	}

	return c.RegisterServiceEndpoints(serviceName, serviceType, eps, enableSsl)
}

func (c *baseComponent) registerServiceEndpointsBySession(s *mcclient.ClientSession, serviceName, serviceType string, eps []*endpoint, enableSSL bool) error {
	urls := map[string]string{}
	for _, ep := range eps {
		urls[ep.Interface] = ep.GetUrl(enableSSL)
	}
	region := c.GetCluster().Spec.Region
	return onecloud.RegisterServiceEndpoints(s, region, serviceName, serviceType, "", urls)
}

func (c *baseComponent) RegisterServiceEndpoints(serviceName, serviceType string, eps []*endpoint, enableSSL bool) error {
	return c.RunWithSession(func(s *mcclient.ClientSession) error {
		return c.registerServiceEndpointsBySession(s, serviceName, serviceType, eps, enableSSL)
	})
}

func (c *baseComponent) registerService(serviceName, serviceType string) error {
	return c.RunWithSession(func(s *mcclient.ClientSession) error {
		_, err := onecloud.EnsureService(s, serviceName, serviceType)
		if err != nil {
			return errors.Wrap(err, "onecloud.EnsureService")
		}
		return nil
	})
}

type keystoneComponent struct {
	*baseComponent
}

func (c keystoneComponent) Setup() error {
	return nil
}

func (c keystoneComponent) SystemInit(oc *v1alpha1.OnecloudCluster) error {
	region := oc.Spec.Region
	if len(oc.Status.RegionServer.RegionId) > 0 {
		region = oc.Status.RegionServer.RegionId
	}
	if err := c.RunWithSession(func(s *mcclient.ClientSession) error {
		if err := doPolicyRoleInit(s); err != nil {
			return errors.Wrap(err, "policy role init")
		}
		if res, err := doCreateRegion(s, region); err != nil {
			return errors.Wrap(err, "create region")
		} else {
			regionId, _ := res.GetString("id")
			oc.Status.RegionServer.RegionId = regionId
		}
		if err := c.doRegisterIdentity(s, region, oc.Spec.LoadBalancerEndpoint, KeystoneComponentName(oc.GetName()),
			constants.KeystoneAdminPort, constants.KeystonePublicPort, true); err != nil {
			return errors.Wrap(err, "register identity endpoint")
		}
		return nil
	}); err != nil {
		return err
	}

	// refresh session when update identity url
	return c.RunWithSession(func(s *mcclient.ClientSession) error {
		if err := doRegisterCloudMeta(s, region); err != nil {
			return errors.Wrap(err, "register cloudmeta endpoint")
		}
		if err := doRegisterTracker(s, region); err != nil {
			return errors.Wrap(err, "register tracker endpoint")
		}
		if err := doCreateExternalService(s); err != nil {
			return errors.Wrap(err, "create external service")
		}
		if err := doRegisterOfflineCloudMeta(s, region); err != nil {
			return errors.Wrap(err, "register offlinecloudmeta endpoint")
		}
		if err := doCreateCommonService(s); err != nil {
			return errors.Wrap(err, "create common service")
		}
		commonConfig, err := c.getCommonConfig(oc)
		if err != nil {
			return errors.Wrap(err, "common config")
		}
		if err := doSyncCommonConfigure(s, commonConfig); err != nil {
			return errors.Wrap(err, "sync common configure")
		}
		if !oc.Spec.Etcd.Disable {
			var certName string
			if oc.Spec.Etcd.EnableTls {
				certConf, err := c.getEtcdCertificate()
				if err != nil {
					return errors.Wrap(err, "get etcd cert")
				}
				if err := doCreateEtcdCertificate(s, certConf); err != nil {
					return errors.Wrap(err, "create etcd certificate")
				}
				certName = constants.ServiceCertEtcdName
			}

			if err := doCreateEtcdServiceEndpoint(s, region, c.getEtcdUrl(), certName); err != nil {
				return errors.Wrap(err, "create etcd endpoint")
			}
		}
		return nil
	})
}

func (c keystoneComponent) getWebAccessUrl(oc *v1alpha1.OnecloudCluster) (string, error) {
	if oc.Spec.LoadBalancerEndpoint == "" {
		return "", errors.Errorf("cluster %s LoadBalancerEndpoint is empty", oc.GetName())
	}
	return fmt.Sprintf("https://%s", oc.Spec.LoadBalancerEndpoint), nil
}

func (c keystoneComponent) getCommonConfig(oc *v1alpha1.OnecloudCluster) (map[string]string, error) {
	url, err := c.getWebAccessUrl(oc)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"api_server": url,
	}, nil
}

func (c keystoneComponent) getEtcdCertificate() (*jsonutils.JSONDict, error) {
	oc := c.GetCluster()
	ret := jsonutils.NewDict()
	ctl := c.baseComponent.manager.GetController()
	secret, err := ctl.kubeCli.CoreV1().Secrets(oc.GetNamespace()).
		Get(constants.EtcdClientSecret, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	ret.Set("certificate", jsonutils.NewString(string(secret.Data[constants.EtcdClientCertName])))
	ret.Set("private_key", jsonutils.NewString(string(secret.Data[constants.EtcdClientKeyName])))
	ret.Set("ca_certificate", jsonutils.NewString(string(secret.Data[constants.EtcdClientCACertName+".crt"])))
	return ret, nil
}

func (c keystoneComponent) getEtcdUrl() string {
	oc := c.GetCluster()
	scheme := "http"
	if oc.Spec.Etcd.EnableTls {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s-etcd-client.%s.svc:%d", scheme, oc.Name, oc.Namespace, constants.EtcdClientPort)
}

func shouldDoPolicyRoleInit(s *mcclient.ClientSession) (bool, error) {
	ret, err := modules.Policies.List(s, nil)
	if err != nil {
		return false, errors.Wrap(err, "list policy")
	}
	return ret.Total == 0, nil
}

func ensureKeystoneVersion36(s *mcclient.ClientSession) error {
	ret, err := modules.Policies.List(s, nil)
	if err != nil {
		return errors.Wrap(err, "list policy")
	}
	if ret.Total == 0 {
		// no policy, a new installation
		return nil
	}
	if ret.Data[0].Contains("scope") {
		return nil
	}
	return errors.Wrap(httperrors.ErrInvalidStatus, "not keystone >= 3.6")
}

func doPolicyRoleInit(s *mcclient.ClientSession) error {
	// check keystone version
	// make sure policy has scope field
	if err := ensureKeystoneVersion36(s); err != nil {
		return errors.Wrap(err, "ensureKeystoneVersion36")
	}
	// create system policies
	policies := locale.GenerateAllPolicies()
	for i := range policies {
		err := createOrUpdatePolicy(s, policies[i])
		if err != nil {
			log.Warningf("createOrUpdatePolicy %s fail %s", policies[i], err)
		}
	}
	// create system roles
	for i := range locale.RoleDefinitions {
		err := createOrUpdateRole(s, locale.RoleDefinitions[i])
		if err != nil {
			log.Warningf("createOrUpdateRole %s fail %s", locale.RoleDefinitions[i], err)
		}
	}
	// update policy quota
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString("default"), "domain")
	result, err := modules.IdentityQuotas.GetQuota(s, params)
	if err != nil {
		log.Warningf("get IdentityQuotas fail %s", err)
	} else {
		policyCnt, _ := result.Int("policy")
		if policyCnt < 500 {
			params.Add(jsonutils.NewInt(500), "policy")
			_, err := modules.IdentityQuotas.DoQuotaSet(s, params)
			if err != nil {
				// ignore the error
				log.Warningf("update IdentityQuotas fail %s", err)
			}
		}
	}
	return nil
}

func doCreateRegion(s *mcclient.ClientSession, region string) (jsonutils.JSONObject, error) {
	return onecloud.CreateRegion(s, region, "")
}

func doRegisterCloudMeta(s *mcclient.ClientSession, regionId string) error {
	return onecloud.RegisterServicePublicInternalEndpoint(s, regionId,
		constants.ServiceNameCloudmeta,
		constants.ServiceTypeCloudmeta,
		"",
		constants.ServiceURLCloudmeta)
}

func doRegisterTracker(s *mcclient.ClientSession, regionId string) error {
	return onecloud.RegisterServicePublicInternalEndpoint(
		s, regionId,
		constants.ServiceNameTorrentTracker,
		constants.ServiceTypeTorrentTracker,
		"",
		constants.ServiceURLTorrentTracker)
}

func (c *keystoneComponent) doRegisterIdentity(
	s *mcclient.ClientSession,
	regionId string,
	publicAddress string,
	keystoneAddress string,
	adminPort int,
	publicPort int,
	enableSSL bool,
) error {
	if publicAddress == "" {
		publicAddress = keystoneAddress
	}
	eps := make([]*endpoint, 0)
	eps = append(
		eps,
		newPublicEndpoint(publicAddress, publicPort, "v3"),
		newInternalEndpoint(keystoneAddress, publicPort, "v3"),
		newEndpointByInterfaceType(publicAddress, adminPort, "v3", constants.EndpointTypeAdmin),
	)

	return c.registerServiceEndpointsBySession(s, constants.ServiceNameKeystone, constants.ServiceTypeIdentity, eps, true)
}

func doCreateExternalService(s *mcclient.ClientSession) error {
	_, err := onecloud.EnsureService(s, constants.ServiceNameExternal, constants.ServiceTypeExternal)
	return err
}

func doCreateCommonService(s *mcclient.ClientSession) error {
	_, err := onecloud.EnsureService(s, constants.ServiceNameCommon, constants.ServiceTypeCommon)
	return err
}

func doSyncCommonConfigure(s *mcclient.ClientSession, defaultConf map[string]string) error {
	_, err := onecloud.SyncServiceConfig(s, defaultConf, constants.ServiceNameCommon)
	return err
}

func doCreateEtcdServiceEndpoint(s *mcclient.ClientSession, regionId, endpointUrl, certName string) error {
	return onecloud.RegisterServiceEndpointByInterfaces(
		s, regionId, constants.ServiceNameEtcd, constants.ServiceTypeEtcd,
		endpointUrl, certName, []string{constants.EndpointTypeInternal},
	)
}

func doCreateEtcdCertificate(s *mcclient.ClientSession, certDetails *jsonutils.JSONDict) error {
	_, err := onecloud.EnsureServiceCertificate(s, constants.ServiceCertEtcdName, certDetails)
	return err
}

func doRegisterOfflineCloudMeta(s *mcclient.ClientSession, regionId string) error {
	return onecloud.RegisterServicePublicInternalEndpoint(s, regionId,
		constants.ServiceNameOfflineCloudmeta,
		constants.ServiceTypeOfflineCloudmeta,
		"",
		constants.ServiceURLOfflineCloudmeta)
}

type regionComponent struct {
	*baseComponent
}

func (c *regionComponent) Setup() error {
	return c.RegisterCloudServiceEndpoint(
		v1alpha1.RegionComponentType,
		constants.ServiceNameRegionV2, constants.ServiceTypeComputeV2,
		constants.RegionPort, "", true)
}

func (c *regionComponent) SystemInit(oc *v1alpha1.OnecloudCluster) error {
	return c.RunWithSession(func(s *mcclient.ClientSession) error {
		region := oc.Spec.Region
		zone := oc.Spec.Zone
		{
			// ensure region-zone created
			regionZone := fmt.Sprintf("%s-%s", region, zone)
			if len(oc.Status.RegionServer.RegionZoneId) == 0 {
				if regionId, err := ensureRegionZone(s, regionZone, ""); err != nil {
					return errors.Wrapf(err, "create region-zone %s-%s", region, zone)
				} else {
					oc.Status.RegionServer.RegionZoneId = regionId
				}
			}
		}
		{
			// ensure zone created
			if len(oc.Status.RegionServer.ZoneId) == 0 {
				if zoneId, err := ensureZone(s, zone); err != nil {
					return errors.Wrapf(err, "create zone %s", zone)
				} else {
					oc.Status.RegionServer.ZoneId = zoneId
				}
			}
			for _, cZone := range oc.Spec.CustomZones {
				// ensure each customized zone created
				if oc.Status.RegionServer.CustomZones == nil {
					oc.Status.RegionServer.CustomZones = make(map[string]string)
				}
				// if zone created, continue
				if _, ok := oc.Status.RegionServer.CustomZones[cZone]; ok {
					continue
				}
				if zoneId, err := ensureZone(s, cZone); err != nil {
					return errors.Wrapf(err, "create zone %s", cZone)
				} else {
					oc.Status.RegionServer.CustomZones[cZone] = zoneId
				}
			}
		}
		{ // ensure wire created
			if len(oc.Status.RegionServer.WireId) == 0 {
				wire := v1alpha1.DefaultOnecloudWire
				if wireId, err := ensureWire(s, zone, wire, 1000); err != nil {
					return errors.Wrapf(err, "create default wire")
				} else {
					oc.Status.RegionServer.WireId = wireId
				}
			}
		}
		if err := initScheduleData(s); err != nil {
			return errors.Wrap(err, "init sched data")
		}
		// TODO: how to inject AWS instance type json
		return nil
	})
}

func ensureZone(s *mcclient.ClientSession, name string) (string, error) {
	res, exists, err := onecloud.IsZoneExists(s, name)
	if err != nil {
		return "", err
	}
	if exists {
		zoneId, _ := res.GetString("id")
		return zoneId, nil
	}
	if res, err := onecloud.CreateZone(s, name); err != nil {
		return "", err
	} else {
		zoneId, _ := res.GetString("id")
		return zoneId, nil
	}
}

func ensureWire(s *mcclient.ClientSession, zone, name string, bw int) (string, error) {
	res, exists, err := onecloud.IsWireExists(s, name)
	if err != nil {
		return "", err
	}
	if exists {
		wireId, _ := res.GetString("id")
		return wireId, nil
	}
	if res, err := onecloud.CreateWire(s, zone, name, bw, v1alpha1.DefaultVPCId); err != nil {
		return "", err
	} else {
		wireId, _ := res.GetString("id")
		return wireId, nil
	}

}

/*func ensureAdminNetwork(s *mcclient.ClientSession, zone string, iface apiv1.NetInterface) error {
	if err := ensureWire(s, zone, iface.Wire, 1000); err != nil {
		return errors.Wrapf(err, "create wire %s", iface.Wire)
	}
	name := apiv1.DefaultOnecloudAdminNetwork
	startIP := iface.Address.String()
	endIP := iface.Address.String()
	gateway := iface.Gateway.String()
	maskLen := iface.MaskLen
	if _, err := onecloud.CreateNetwork(s, name, gateway, constants.NetworkTypeBaremetal, iface.Wire, maskLen, startIP, endIP); err != nil {
		return errors.Wrapf(err, "name %q, gateway %q, %s-%s, masklen %d", name, gateway, startIP, endIP, maskLen)
	}
	return nil
}*/

func ensureRegionZone(s *mcclient.ClientSession, region, zone string) (string, error) {
	res, err := onecloud.CreateRegion(s, region, zone)
	if err != nil {
		return "", err
	}
	regionId, _ := res.GetString("id")
	return regionId, err
}

func initScheduleData(s *mcclient.ClientSession) error {
	if err := registerSchedSameProjectCloudprovider(s); err != nil {
		return err
	}
	/*
	 *if err := registerSchedAzureClassicHost(s); err != nil {
	 *    return err
	 *}
	 */
	return nil
}

func registerSchedSameProjectCloudprovider(s *mcclient.ClientSession) error {
	obj, err := onecloud.EnsureSchedtag(s, "same_project", "prefer", "Prefer hosts belongs to same project")
	if err != nil {
		return errors.Wrap(err, "create schedtag same_project")
	}
	id, _ := obj.GetString("id")
	if _, err := onecloud.EnsureDynamicSchedtag(s, "same_cloudprovider_project", id, "host.cloudprovider.tenant_id == server.owner_tenant_id"); err != nil {
		return err
	}
	return nil
}

func registerSchedAzureClassicHost(s *mcclient.ClientSession) error {
	obj, err := onecloud.EnsureSchedtag(s, "azure_classic", "exclude", "Do not use azure classic host to create VM")
	if err != nil {
		return errors.Wrap(err, "create schedtag azure_classic")
	}
	id, _ := obj.GetString("id")
	if _, err := onecloud.EnsureDynamicSchedtag(s, "avoid_azure_classic_host", id, `host.name.endswith("-classic") && host.host_type == "azure"`); err != nil {
		return err
	}
	return nil
}

type glanceComponent struct {
	*baseComponent
}

func (c *glanceComponent) Setup() error {
	return c.RegisterCloudServiceEndpoint(
		v1alpha1.GlanceComponentType,
		constants.ServiceNameGlance, constants.ServiceTypeGlance,
		constants.GlanceAPIPort, "v1", true)
}

type registerServiceComponent struct {
	*baseComponent
	cType       v1alpha1.ComponentType
	serviceName string
	serviceType string
}

func NewRegisterServiceComponent(
	man ComponentManager,
	serviceName string,
	serviceType string,
) PhaseControl {
	return &registerServiceComponent{
		baseComponent: newBaseComponent(man),
		serviceName:   serviceName,
		serviceType:   serviceType,
	}
}

func (c *registerServiceComponent) Setup() error {
	return c.registerService(c.serviceName, c.serviceType)
}

type registerEndpointComponent struct {
	*baseComponent
	cType       v1alpha1.ComponentType
	serviceName string
	serviceType string
	port        int
	prefix      string
	enableSSL   bool
}

func NewRegisterEndpointComponent(
	man ComponentManager,
	ctype v1alpha1.ComponentType,
	serviceName string,
	serviceType string,
	port int, prefix string,
) PhaseControl {
	return &registerEndpointComponent{
		baseComponent: newBaseComponent(man),
		cType:         ctype,
		serviceName:   serviceName,
		serviceType:   serviceType,
		port:          port,
		prefix:        prefix,
		enableSSL:     true,
	}
}

func NewRegisterEndpointComponentWithSsl(
	man ComponentManager,
	ctype v1alpha1.ComponentType,
	serviceName string,
	serviceType string,
	port int, prefix string, enableSSL bool,
) PhaseControl {
	return &registerEndpointComponent{
		baseComponent: newBaseComponent(man),
		cType:         ctype,
		serviceName:   serviceName,
		serviceType:   serviceType,
		port:          port,
		prefix:        prefix,
		enableSSL:     enableSSL,
	}
}

func (c *registerEndpointComponent) Setup() error {
	return c.RegisterCloudServiceEndpoint(c.cType, c.serviceName, c.serviceType, c.port, c.prefix, c.enableSSL)
}

type itsmComponent struct {
	*registerEndpointComponent
}

func NewItsmEndpointComponent(man ComponentManager,
	ctype v1alpha1.ComponentType,
	serviceName string,
	serviceType string,
	port int, prefix string) PhaseControl {
	return &itsmComponent{
		registerEndpointComponent: &registerEndpointComponent{
			baseComponent: newBaseComponent(man),
			cType:         ctype,
			serviceName:   serviceName,
			serviceType:   serviceType,
			port:          port,
			prefix:        prefix,
		},
	}
}

func (c *itsmComponent) Setup() error {
	return c.RegisterCloudServiceEndpoint(c.cType, c.serviceName, c.serviceType, c.port, c.prefix, true)
}

type yunionagentComponent struct {
	*baseComponent
}

func (c yunionagentComponent) Setup() error {
	return c.RegisterCloudServiceEndpoint(
		v1alpha1.YunionagentComponentType,
		constants.ServiceNameYunionAgent, constants.ServiceTypeYunionAgent,
		constants.YunionAgentPort, "", true)
}

func (c yunionagentComponent) SystemInit(oc *v1alpha1.OnecloudCluster) error {
	if err := c.addWelcomeNotice(); err != nil {
		klog.Errorf("yunion agent add notices error: %v", err)
	}
	return nil
}

func (c yunionagentComponent) addWelcomeNotice() error {
	/*return c.RunWithSession(func(s *mcclient.ClientSession) error {
		ret, err := modules.Notice.List(s, nil)
		if err != nil {
			return err
		}
		if ret.Total > 0 {
			return nil
		}
		params := jsonutils.NewDict()
		params.Add(jsonutils.NewString("欢迎使用云管平台"), "title")
		params.Add(jsonutils.NewString("欢迎使用云管平台。这是公告栏，您可以在这里发布需要告知所有用户的消息。"), "content")

		_, err = modules.Notice.Create(s, params)
		return err
	})*/
	return nil
}

type devtoolComponent struct {
	*baseComponent
}

func (c devtoolComponent) Setup() error {
	return c.RegisterCloudServiceEndpoint(v1alpha1.DevtoolComponentType,
		constants.ServiceNameDevtool, constants.ServiceTypeDevtool,
		constants.DevtoolPort, "", true)
}

func (c devtoolComponent) SystemInit(oc *v1alpha1.OnecloudCluster) error {
	for _, f := range []func() error{
		c.ensureTemplatePing,
		c.ensureTemplateTelegraf,
		c.ensureTemplateNginx,
		c.ensureMonitorAgentScript,
	} {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}

func (c devtoolComponent) upgradeDir() string {
	return "/opt/yunion/upgrade"
}

func (c devtoolComponent) packagePath(name string) string {
	return fmt.Sprintf("%s/rpms/packages/%s", c.upgradeDir(), name)
}

func (c devtoolComponent) yumRepoUrl() string {
	return "https://iso.yunion.cn/yumrepo-2.13"
}

func (c devtoolComponent) rpmPackageUrl(pkgName string) string {
	return fmt.Sprintf("%s/rpms/packages/%s", c.yumRepoUrl(), pkgName)
}

func (c devtoolComponent) ensureTemplatePing() error {
	return c.RunWithSession(func(s *mcclient.ClientSession) error {
		hosts := []string{"HOSTNAME ansible_become=yes"}
		mods := []string{
			"ping",
		}
		// TODO: fix this bug
		files := map[string]string{
			"conf": "",
		}
		_, err := onecloud.EnsureDevtoolTemplate(s, "ping-host", hosts, mods, files, 86400)
		return err
	})
}

func (c devtoolComponent) ensureTemplateNginx() error {
	return c.RunWithSession(func(s *mcclient.ClientSession) error {
		files := map[string]string{
			"conf": "",
		}
		hosts := []string{"HOSTNAME ansible_become=yes"}
		mods := []string{
			"yum name=epel-release state=present",
			"yum name=nginx state=installed",
			"systemd name=nginx enabled=yes state=started",
		}
		_, err := onecloud.EnsureDevtoolTemplate(s, "install-nginx-on-centos", hosts, mods, files, 86400)
		return err
	})
}

func (c devtoolComponent) ensureTemplateTelegraf() error {
	return c.RunWithSession(func(s *mcclient.ClientSession) error {
		hosts := []string{"HOSTNAME ansible_become=yes influxdb=INFLUXDB"}
		pkgName := "telegraf-1.5.18-1.x86_64.rpm"
		mods := []string{
			"file path=/etc/telegraf state=directory mode=0755",
			"template src=conf dest=/etc/telegraf/telegraf.conf mode=0644",
			fmt.Sprintf("get_url url=%s dest=/tmp/%s", c.rpmPackageUrl(pkgName), pkgName),
			fmt.Sprintf("yum name=/tmp/%s state=installed", pkgName),
			"systemd name=telegraf enabled=yes state=started",
		}
		files := map[string]string{
			"conf": onecloud.DevtoolTelegrafConf,
		}
		_, err := onecloud.EnsureDevtoolTemplate(s, "install-telegraf-on-centos", hosts, mods, files, 86400)
		return err
	})
}

func (c devtoolComponent) ensureMonitorAgentScript() error {
	return c.RunWithSession(onecloud.EnsureAgentAnsiblePlaybookRef)
}

type monitorComponent struct {
	*baseComponent
}

func (c monitorComponent) Setup() error {
	return c.RegisterCloudServiceEndpoint(v1alpha1.MonitorComponentType, constants.ServiceNameMonitor, constants.ServiceTypeMonitor, constants.MonitorPort, "", true)
}

func (c monitorComponent) SystemInit(oc *v1alpha1.OnecloudCluster) error {
	alertInfo := c.getInitInfo()
	//c.manager.GetController().getSession(c.GetCluster())
	session := auth.GetAdminSession(context.Background(), "", "")
	rtnAlert, err := onecloud.GetCommonAlertOfSys(session)
	if err != nil {
		return errors.Wrap(err, "monitorComponent GetCommonAlertOfSys")
	}
	tmpAlert := rtnAlert
	for metric, tem := range alertInfo {
		match, id, alert, deleteAlerts, err := c.matchFromRtnAlerts(metric, tmpAlert)
		if err != nil {
			return err
		}
		if match && alert != nil {
			_, err := onecloud.UpdateCommonAlert(session, tem, id, alert)
			if err != nil {
				log.Errorf("UpdateCommonAlert err:%v", err)
			}
			tmpAlert = deleteAlerts
			continue
		}
		_, err = onecloud.CreateCommonAlert(session, tem)
		if err != nil {
			log.Errorln("CreateCommonAlert err:", err)
		}
		tmpAlert = deleteAlerts
	}
	ids := make([]string, 0)
	for _, alert := range tmpAlert {
		id, _ := alert.GetString("id")
		ids = append(ids, id)
	}
	if len(ids) != 0 {
		onecloud.DeleteCommonAlert(session, ids)
	}
	return nil
}

func (c monitorComponent) matchFromRtnAlerts(metric string, rtnAlert []jsonutils.JSONObject) (bool, string,
	jsonutils.JSONObject,
	[]jsonutils.JSONObject, error) {
	match := false
	id := ""
	deleteAlerts := make([]jsonutils.JSONObject, 0)
	for i, alert := range rtnAlert {
		name, _ := alert.GetString("name")
		metadataObj, err := alert.Get("metadata")
		if err != nil {
			return match, id, nil, nil, errors.Wrap(err, "cannot get metadata")
		}
		metaName, _ := metadataObj.GetString("meta_name")
		if metric == name || metric == metaName {
			match = true
			id, _ = alert.GetString("id")
			for start := i + 1; start < len(rtnAlert); start++ {
				deleteAlerts = append(deleteAlerts, rtnAlert[start])
			}
			return match, id, alert, deleteAlerts, nil
		}
		deleteAlerts = append(deleteAlerts, alert)
	}
	return match, id, nil, deleteAlerts, nil
}

func (c monitorComponent) getInitInfo() map[string]onecloud.CommonAlertTem {
	cpuTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "cpu",
		Field:       []string{"usage_active"},
		Comparator:  ">=",
		Threshold:   90,
		Name:        "cpu.usage_active",
		Reduce:      "avg",
		Description: "监测宿主机CPU使用率",
	}
	memTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "mem",
		Field:       []string{"available"},
		Comparator:  "<=",
		Threshold:   524288000,
		Name:        "mem.available",
		Reduce:      "avg",
		Description: "监测宿主机可用内存",
	}
	diskAvaTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "disk",
		Operator:    "",
		FieldFunc:   "last",
		Field:       []string{"free", "total"},
		FieldOpt:    "/",
		Comparator:  "<=",
		Threshold:   0.2,
		Filters: []monitor.MetricQueryTag{
			{
				Key:       "path",
				Operator:  "=",
				Value:     "/",
				Condition: "OR",
			},
			{
				Key:       "path",
				Operator:  "=",
				Value:     "/opt",
				Condition: "OR",
			},
		},
		Name:        "disk.free/total",
		Reduce:      "last",
		From:        "5m",
		Description: "监测宿主机磁盘容量空闲率",
	}
	diskNodeAvaTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "disk",
		Operator:    "",
		FieldFunc:   "last",
		Field:       []string{"inodes_free", "inodes_total"},
		FieldOpt:    "/",
		Comparator:  "<=",
		Threshold:   0.15,
		Filters: []monitor.MetricQueryTag{
			{
				Key:       "path",
				Operator:  "=",
				Value:     "/",
				Condition: "AND",
			},
		},
		Name:        "disk.inodes_free/inodes_total",
		Reduce:      "last",
		From:        "5m",
		Description: "监测宿主机磁盘inode空闲率",
	}
	smartDevTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "smart_device",
		Field:       []string{"exit_status"},
		FieldFunc:   "last",
		Comparator:  "==",
		Threshold:   0,
		Filters: []monitor.MetricQueryTag{
			{
				Key:       "health_ok",
				Operator:  "=",
				Value:     "false",
				Condition: "AND",
			},
		},
		Name:        "smart_device.exit_status",
		Reduce:      "last",
		Description: "监测磁盘健康状态",
	}
	genHostRaidStatusFilter := func(status ...string) []monitor.MetricQueryTag {
		ret := make([]monitor.MetricQueryTag, 0)
		for _, s := range status {
			filter := monitor.MetricQueryTag{
				Key:       "status",
				Value:     s,
				Operator:  "=",
				Condition: "OR",
			}
			ret = append(ret, filter)
		}
		return ret
	}
	hostRaidTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "host_raid",
		Field:       []string{"adapter", "status", "slot"},
		FieldFunc:   "last",
		Comparator:  ">=",
		Threshold:   0,
		Filters: genHostRaidStatusFilter(
			"offline",
			"failed",
			"degraded",
			"out of sync",
		),
		Name:        "host_raid.adapter",
		GetPointStr: true,
		Reduce:      "last",
		From:        "5m",
		Description: "检查宿主机raid控制器状态",
	}
	cloudaccountTem := onecloud.CommonAlertTem{
		Database:    "meter_db",
		Measurement: "cloudaccount_balance",
		Field:       []string{"balance"},
		Comparator:  "<=",
		Threshold:   100,
		Name:        "cloudaccount_balance.balance",
		Reduce:      "last",
		Description: "监测云账号余额",
	}
	noDataTem := onecloud.CommonAlertTem{
		Database:      "telegraf",
		Measurement:   "system",
		Field:         []string{"uptime"},
		FieldFunc:     "last",
		Comparator:    "==",
		Threshold:     0,
		Name:          "system.uptime",
		Reduce:        "last",
		ConditionType: "nodata_query",
		From:          "3m",
		Interval:      "1m",
		Description:   "监测部署节点是否下线",
	}

	defunctProcessTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "processes",
		Operator:    "",
		FieldFunc:   "last",
		Field:       []string{"zombies"},
		GroupBy:     "host_id",
		Comparator:  ">=",
		Threshold:   10,
		Name:        "process.zombies",
		Reduce:      "last",
		From:        "5m",
		Description: "监测节点僵尸进程数",
	}

	totalProcessTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "processes",
		Operator:    "",
		FieldFunc:   "last",
		Field:       []string{"total"},
		GroupBy:     "host_id",
		Comparator:  ">=",
		Threshold:   20000,
		Name:        "process.total",
		Reduce:      "last",
		From:        "5m",
		Description: "监测节点总进程数",
	}

	speAlert := map[string]onecloud.CommonAlertTem{
		cpuTem.Name:            cpuTem,
		memTem.Name:            memTem,
		diskAvaTem.Name:        diskAvaTem,
		diskNodeAvaTem.Name:    diskNodeAvaTem,
		cloudaccountTem.Name:   cloudaccountTem,
		smartDevTem.Name:       smartDevTem,
		hostRaidTem.Name:       hostRaidTem,
		noDataTem.Name:         noDataTem,
		defunctProcessTem.Name: defunctProcessTem,
		totalProcessTem.Name:   totalProcessTem,
	}
	return speAlert
}

type kubeServerComponent struct {
	*baseComponent

	nodeLister corelisters.NodeLister
}

func (c *kubeServerComponent) Setup() error {
	return NewRegisterEndpointComponent(
		c.manager, v1alpha1.KubeServerComponentType,
		constants.ServiceNameKubeServer, constants.ServiceTypeKubeServer,
		constants.KubeServerPort, "api").Setup()
}

func (c *kubeServerComponent) SystemInit(oc *v1alpha1.OnecloudCluster) error {
	if !oc.Spec.Minio.Enable {
		return nil
	}
	masterNodes, err := k8sutil.GetReadyMasterNodes(c.nodeLister)
	if err != nil {
		return errors.Wrap(err, "List k8s ready master node")
	}

	spec := &oc.Spec.Minio
	if len(masterNodes) >= 3 {
		if spec.Mode == "" {
			spec.Mode = v1alpha1.MinioModeDistributed
		}
	} else {
		if spec.Mode == v1alpha1.MinioModeDistributed {
			return errors.Errorf("Master ready node count %d, but mode is %s", len(masterNodes), spec.Mode)
		}

		if spec.Mode == "" {
			spec.Mode = v1alpha1.MinioModeStandalone
		}
	}
	return c.RunWithSession(func(s *mcclient.ClientSession) error {
		if err := c.doEnableMinio(s, spec); err != nil {
			return errors.Wrap(err, "Enable minio")
		}
		return nil
	})
}

func (c *kubeServerComponent) doEnableMinio(
	s *mcclient.ClientSession,
	spec *v1alpha1.Minio,
) error {
	return onecloud.SyncMinio(s, spec)
}
