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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/k8sutil"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
	"yunion.io/x/onecloud/pkg/apis/monitor"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/auth"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
)

var (
	SessionDebug bool
	SyncUser     bool

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
		if err := makeDomainAdminPublic(s); err != nil {
			return errors.Wrap(err, "always share domainadmin")
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
		commonConfig, err := c.getCommonConfig()
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

func (c keystoneComponent) getWebAccessUrl() (string, error) {
	occtl := c.baseComponent.manager.GetController()
	masterNodeSelector := labels.NewSelector()
	r, err := labels.NewRequirement(
		kubeadmconstants.LabelNodeRoleMaster, selection.Exists, nil)
	if err != nil {
		return "", err
	}
	masterNodeSelector = masterNodeSelector.Add(*r)
	listOpt := metav1.ListOptions{LabelSelector: masterNodeSelector.String()}
	nodes, err := occtl.kubeCli.CoreV1().Nodes().List(listOpt)
	if err != nil {
		return "", err
	}
	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("no master node found")
	}
	var masterAddress string
	for _, node := range nodes.Items {
		if k8sutil.IsNodeReady(node) {
			for _, addr := range node.Status.Addresses {
				if addr.Type == v1.NodeInternalIP {
					masterAddress = addr.Address
					break
				}
			}
		}
		if len(masterAddress) >= 0 {
			break
		}
	}
	if len(masterAddress) == 0 {
		return "", fmt.Errorf("can't find master node internal ip")
	}
	return fmt.Sprintf("https://%s", masterAddress), nil
}

func (c keystoneComponent) getCommonConfig() (map[string]string, error) {
	url, err := c.getWebAccessUrl()
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

func doPolicyRoleInit(s *mcclient.ClientSession) error {
	doInit, err := shouldDoPolicyRoleInit(s)
	if err != nil {
		return errors.Wrap(err, "should do policy init")
	}
	if !doInit {
		return nil
	}
	klog.Infof("Init policy and role...")
	for policyType, content := range DefaultPolicies {
		if _, err := PolicyCreate(s, policyType, content, true); err != nil {
			return errors.Wrapf(err, "create policy %s", policyType)
		}
	}
	for role, desc := range DefaultRoles {
		if _, err := onecloud.EnsureRole(s, role, desc); err != nil {
			return errors.Wrapf(err, "create role %s", role)
		}
	}
	if err := RolesPublic(s, constants.PublicRoles); err != nil {
		return errors.Wrap(err, "public roles")
	}
	if err := PoliciesPublic(s, constants.PublicPolicies); err != nil {
		return errors.Wrap(err, "public policies")
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

func makeDomainAdminPublic(s *mcclient.ClientSession) error {
	if err := RolesPublic(s, []string{constants.RoleDomainAdmin}); err != nil {
		return err
	}
	if err := PoliciesPublic(s, []string{constants.PolicyTypeDomainAdmin}); err != nil {
		return err
	}
	return nil
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
		regionZone := fmt.Sprintf("%s-%s", region, zone)
		wire := v1alpha1.DefaultOnecloudWire
		{ // ensure region-zone created
			if len(oc.Status.RegionServer.RegionZoneId) > 0 {
				regionZone = oc.Status.RegionServer.RegionZoneId
			}
			if regionId, err := ensureRegionZone(s, regionZone, ""); err != nil {
				return errors.Wrapf(err, "create region-zone %s-%s", region, zone)
			} else {
				oc.Status.RegionServer.RegionZoneId = regionId
			}
		}
		{ // ensure zone created
			if len(oc.Status.RegionServer.ZoneId) > 0 {
				zone = oc.Status.RegionServer.ZoneId
			}
			if zoneId, err := ensureZone(s, zone); err != nil {
				return errors.Wrapf(err, "create zone %s", zone)
			} else {
				oc.Status.RegionServer.ZoneId = zoneId
			}
		}
		{ // ensure wire created
			if len(oc.Status.RegionServer.WireId) > 0 {
				wire = oc.Status.RegionServer.WireId
			}
			if wireId, err := ensureWire(s, zone, wire, 1000); err != nil {
				return errors.Wrapf(err, "create default wire")
			} else {
				oc.Status.RegionServer.WireId = wireId
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
	}
}

func (c *registerEndpointComponent) Setup() error {
	return c.RegisterCloudServiceEndpoint(c.cType, c.serviceName, c.serviceType, c.port, c.prefix, true)
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
	return c.RegisterCloudServiceEndpoint(c.cType, c.serviceName, c.serviceType, c.port, c.prefix, false)
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
	return c.RunWithSession(func(s *mcclient.ClientSession) error {
		ret, err := modules.Notice.List(s, nil)
		if err != nil {
			return err
		}
		if ret.Total > 0 {
			return nil
		}
		params := jsonutils.NewDict()
		params.Add(jsonutils.NewString("欢迎使用云管平台"), "title")
		params.Add(jsonutils.NewString("欢迎使用OneCloud多云云管平台。这是公告栏，您可以在这里发布需要告知所有用户的消息。"), "content")

		_, err = modules.Notice.Create(s, params)
		return err
	})
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
	for metric, tem := range alertInfo {
		match := false
	search:
		for _, alert := range rtnAlert {
			metricDs, err := alert.(*jsonutils.JSONDict).GetArray("common_alert_metric_details")
			if err != nil {
				log.Errorln(err)
				return err
			}
			for _, metricD := range metricDs {
				measurement, err := metricD.GetString("measurement")
				if err != nil {
					log.Errorln("get measurement", err)
					return err
				}
				field, err := metricD.GetString("field")
				if err != nil {
					log.Errorln("get field", err)
					return err
				}
				if metric == fmt.Sprintf("%s.%s", measurement, field) {
					match = true
					break search
				}
			}
		}
		if match {
			continue
		}
		_, err = onecloud.CreateCommonAlert(session, tem)
		if err != nil {
			log.Errorln("CreateCommonAlert err:", err)
		}
	}
	return nil
}

func (c monitorComponent) getInitInfo() map[string]onecloud.CommonAlertTem {
	cpuTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "cpu",
		Field:       []string{"usage_active"},
		Comparator:  ">=",
		Threshold:   90,
		Name:        "cpu.usage_active",
	}
	memTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "mem",
		Field:       []string{"free"},
		Comparator:  "<=",
		Threshold:   524288000,
		Name:        "mem.free",
	}
	diskAvaTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "disk",
		Operator:    "",
		Field:       []string{"free", "total"},
		FieldOpt:    "/",
		Comparator:  "<=",
		Threshold:   0.2,
		Filters: []monitor.MetricQueryTag{
			monitor.MetricQueryTag{
				Key:       "path",
				Operator:  "=",
				Value:     "/",
				Condition: "OR",
			},
			monitor.MetricQueryTag{
				Key:       "path",
				Operator:  "=",
				Value:     "/opt",
				Condition: "OR",
			},
		},
		Name: "disk.free/total",
	}
	diskNodeAvaTem := onecloud.CommonAlertTem{
		Database:    "telegraf",
		Measurement: "disk",
		Operator:    "",
		Field:       []string{"inodes_free", "inodes_total"},
		FieldOpt:    "/",
		Comparator:  "<=",
		Threshold:   0.15,
		Filters: []monitor.MetricQueryTag{
			monitor.MetricQueryTag{
				Key:       "path",
				Operator:  "=",
				Value:     "/",
				Condition: "AND",
			},
		},
		Name: "disk.inodes_free/inodes_total",
	}
	speAlert := map[string]onecloud.CommonAlertTem{
		cpuTem.Name:         cpuTem,
		memTem.Name:         memTem,
		diskAvaTem.Name:     diskAvaTem,
		diskNodeAvaTem.Name: diskNodeAvaTem,
	}
	return speAlert
}
