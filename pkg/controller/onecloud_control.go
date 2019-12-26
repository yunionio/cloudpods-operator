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
	"time"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"

	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

var (
	SessionDebug bool
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

func NewOnecloudClientToken(oc *v1alpha1.OnecloudCluster) (*mcclient.Client, mcclient.TokenCredential, error) {
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
	cli, token, err := NewOnecloudClientToken(oc)
	if err != nil {
		return nil, err
	}
	token = mcclient.SimplifyToken(token)
	cli.SetServiceCatalog(nil)
	return NewOnecloudSessionByToken(cli, oc.Spec.Region, token)
}

func NewOnecloudClientSession(oc *v1alpha1.OnecloudCluster) (*mcclient.ClientSession, error) {
	cli, token, err := NewOnecloudClientToken(oc)
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

func (w *OnecloudControl) GetSession(oc *v1alpha1.OnecloudCluster) (*mcclient.ClientSession, error) {
	return NewOnecloudClientSession(oc)
}

func (w *OnecloudControl) GetSessionNoEndpoints(oc *v1alpha1.OnecloudCluster) (*mcclient.ClientSession, error) {
	return NewOnecloudSimpleClientSession(oc)
}

type PhaseControl interface {
	Setup() error
	SystemInit() error
}

type ComponentManager interface {
	GetSession() (*mcclient.ClientSession, error)
	GetSessionNoEndpoints() (*mcclient.ClientSession, error)
	GetController() *OnecloudControl
	GetCluster() *v1alpha1.OnecloudCluster
	Keystone() PhaseControl
	Region() PhaseControl
	Glance() PhaseControl
	YunionAgent() PhaseControl
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

func (c *realComponent) GetSession() (*mcclient.ClientSession, error) {
	return c.controller.GetSession(c.oc)
}

func (c *realComponent) GetSessionNoEndpoints() (*mcclient.ClientSession, error) {
	return c.controller.GetSessionNoEndpoints(c.oc)
}

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

type baseComponent struct {
	manager ComponentManager
}

func newBaseComponent(manager ComponentManager) *baseComponent {
	return &baseComponent{manager: manager}
}

func (c *baseComponent) GetSession() (*mcclient.ClientSession, error) {
	return c.manager.GetSession()
}

func (c *baseComponent) GetSessionNoEndpoints() (*mcclient.ClientSession, error) {
	return c.manager.GetSessionNoEndpoints()
}

func (c *baseComponent) GetCluster() *v1alpha1.OnecloudCluster {
	return c.manager.GetCluster()
}

func (c *baseComponent) SystemInit() error {
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
	port int, prefix string,
) error {
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

	return c.RegisterServiceEndpoints(serviceName, serviceType, eps, true)
}

func (c *baseComponent) registerServiceEndpointsBySession(s *mcclient.ClientSession, serviceName, serviceType string, eps []*endpoint, enableSSL bool) error {
	urls := map[string]string{}
	for _, ep := range eps {
		urls[ep.Interface] = ep.GetUrl(enableSSL)
	}
	region := c.GetCluster().Spec.Region
	return onecloud.RegisterServiceEndpoints(s, region, serviceName, serviceType, urls)
}

func (c *baseComponent) RegisterServiceEndpoints(serviceName, serviceType string, eps []*endpoint, enableSSL bool) error {
	s, err := c.GetSession()
	if err != nil {
		return err
	}
	return c.registerServiceEndpointsBySession(s, serviceName, serviceType, eps, enableSSL)
}

func (c *baseComponent) registerService(serviceName, serviceType string) error {
	s, err := c.GetSession()
	if err != nil {
		return errors.Wrap(err, "c.GetSession")
	}
	_, err = onecloud.EnsureService(s, serviceName, serviceType)
	if err != nil {
		return errors.Wrap(err, "onecloud.EnsureService")
	}
	return nil
}

type keystoneComponent struct {
	*baseComponent
}

func (c keystoneComponent) Setup() error {
	return nil
}

func (c keystoneComponent) SystemInit() error {
	s, err := c.GetSessionNoEndpoints()
	if err != nil {
		return err
	}
	oc := c.GetCluster()
	if err := doPolicyRoleInit(s); err != nil {
		return errors.Wrap(err, "policy role init")
	}
	region := oc.Spec.Region
	if err := c.doRegisterIdentity(s, region, oc.Spec.LoadBalancerEndpoint, KeystoneComponentName(oc.GetName()), constants.KeystoneAdminPort, constants.KeystonePublicPort, true); err != nil {
		return errors.Wrap(err, "register identity endpoint")
	}

	// refresh session when update identity url
	s, err = c.GetSession()
	if err != nil {
		return err
	}

	if _, err := doCreateRegion(s, region); err != nil {
		return errors.Wrap(err, "create region")
	}
	if err := doRegisterCloudMeta(s, region); err != nil {
		return errors.Wrap(err, "register cloudmeta endpoint")
	}
	if err := doRegisterTracker(s, region); err != nil {
		return errors.Wrap(err, "register tracker endpoint")
	}
	if err := doRegisterFakeAutoUpdate(s, region); err != nil {
		return errors.Wrap(err, "register fake autoupdate endpoint")
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
	return nil
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
		constants.ServiceURLCloudmeta)
}

func doRegisterTracker(s *mcclient.ClientSession, regionId string) error {
	return onecloud.RegisterServicePublicInternalEndpoint(
		s, regionId,
		constants.ServiceNameTorrentTracker,
		constants.ServiceTypeTorrentTracker,
		constants.ServiceURLTorrentTracker)
}

func doRegisterFakeAutoUpdate(s *mcclient.ClientSession, regionId string) error {
	return onecloud.RegisterServicePublicInternalEndpoint(
		s, regionId,
		constants.ServiceNameAutoUpdate,
		constants.ServiceTypeAutoUpdate,
		constants.ServiceURLFakeAutoUpdate)
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

func doRegisterOfflineCloudMeta(s *mcclient.ClientSession, regionId string) error {
	return onecloud.RegisterServicePublicInternalEndpoint(s, regionId,
		constants.ServiceNameOfflineCloudmeta,
		constants.ServiceTypeOfflineCloudmeta,
		constants.ServiceURLOfflineCloudmeta)
}

type regionComponent struct {
	*baseComponent
}

func (c *regionComponent) Setup() error {
	return c.RegisterCloudServiceEndpoint(
		v1alpha1.RegionComponentType,
		constants.ServiceNameRegionV2, constants.ServiceTypeComputeV2,
		constants.RegionPort, "")
}

func (c *regionComponent) SystemInit() error {
	oc := c.GetCluster()
	s, err := c.GetSession()
	if err != nil {
		return err
	}
	region := oc.Spec.Region
	zone := oc.Spec.Zone
	if err := ensureZone(s, zone); err != nil {
		return errors.Wrapf(err, "create zone %s", zone)
	}
	if err := ensureRegionZone(s, region, zone); err != nil {
		return errors.Wrapf(err, "create region-zone %s-%s", region, zone)
	}
	if err := ensureWire(s, oc.Spec.Zone, v1alpha1.DefaultOnecloudWire, 1000); err != nil {
		return errors.Wrapf(err, "create default wire")
	}
	if err := initScheduleData(s); err != nil {
		return errors.Wrap(err, "init sched data")
	}
	// TODO: how to inject AWS instance type json
	return nil
}

func ensureZone(s *mcclient.ClientSession, name string) error {
	_, exists, err := onecloud.IsZoneExists(s, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err := onecloud.CreateZone(s, name); err != nil {
		return err
	}
	return nil
}

func ensureWire(s *mcclient.ClientSession, zone, name string, bw int) error {
	_, exists, err := onecloud.IsWireExists(s, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err := onecloud.CreateWire(s, zone, name, bw, v1alpha1.DefaultVPCId); err != nil {
		return err
	}
	return nil
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

func ensureRegionZone(s *mcclient.ClientSession, region, zone string) error {
	_, err := onecloud.CreateRegion(s, region, zone)
	return err
}

func initScheduleData(s *mcclient.ClientSession) error {
	if err := registerSchedSameProjectCloudprovider(s); err != nil {
		return err
	}
	if err := registerSchedAzureClassicHost(s); err != nil {
		return err
	}
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
		constants.GlanceAPIPort, "v1")
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
	return c.RegisterCloudServiceEndpoint(c.cType, c.serviceName, c.serviceType, c.port, c.prefix)
}

type yunionagentComponent struct {
	*baseComponent
}

func (c yunionagentComponent) Setup() error {
	return c.RegisterCloudServiceEndpoint(
		v1alpha1.YunionagentComponentType,
		constants.ServiceNameYunionAgent, constants.ServiceTypeYunionAgent,
		constants.YunionAgentPort, "")
}

func (c yunionagentComponent) SystemInit() error {
	if err := c.addWelcomeNotice(); err != nil {
		klog.Errorf("yunion agent add notices error: %v", err)
	}
	return nil
}

func (c yunionagentComponent) addWelcomeNotice() error {
	s, err := c.GetSession()
	if err != nil {
		return err
	}
	ret, err := modules.Notice.List(s, nil)
	if err != nil {
		return err
	}
	if ret.Total > 0 {
		return nil
	}
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString("欢迎使用云管平台"), "title")
	params.Add(jsonutils.NewString("欢迎使用OneCloud多云云管平台。这里告栏。您可以在这里发布需要告知所有用户的消息。"), "content")

	_, err = modules.Notice.Create(s, params)
	return err
}
