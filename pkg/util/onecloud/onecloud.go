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

package onecloud

import (
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"yunion.io/x/jsonutils"
	ansibleapi "yunion.io/x/onecloud/pkg/apis/ansible"
	monitorapi "yunion.io/x/onecloud/pkg/apis/monitor"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modulebase"
	"yunion.io/x/onecloud/pkg/mcclient/modules"
	"yunion.io/x/onecloud/pkg/util/ansible"
	"yunion.io/x/onecloud/pkg/util/httputils"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
)

const (
	NotFoundMsg = "NotFoundError"
)

func IsNotFoundError(err error) bool {
	if httpErr, ok := err.(*httputils.JSONClientError); ok {
		if httpErr.Code == http.StatusNotFound {
			return true
		}
	}
	if strings.Contains(err.Error(), NotFoundMsg) {
		return true
	}
	return false
}

func IsResourceExists(s *mcclient.ClientSession, manager modulebase.Manager, name string) (jsonutils.JSONObject, bool, error) {
	obj, err := manager.Get(s, name, nil)
	if err == nil {
		return obj, true, nil
	}
	if IsNotFoundError(err) {
		return nil, false, nil
	}
	return nil, false, err
}

func EnsureResource(
	s *mcclient.ClientSession,
	man modulebase.Manager,
	name string,
	createFunc func() (jsonutils.JSONObject, error),
) (jsonutils.JSONObject, error) {
	obj, exists, err := IsResourceExists(s, man, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return obj, nil
	}
	return createFunc()
}

func DeleteResource(
	s *mcclient.ClientSession,
	man modulebase.Manager,
	name string,
) error {
	obj, exists, err := IsResourceExists(s, man, name)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	id, _ := obj.GetString("id")
	_, err = man.Delete(s, id, nil)
	return err
}

func IsRoleExists(s *mcclient.ClientSession, roleName string) (jsonutils.JSONObject, bool, error) {
	return IsResourceExists(s, &modules.RolesV3, roleName)
}

func CreateRole(s *mcclient.ClientSession, roleName, description string) (jsonutils.JSONObject, error) {
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(roleName), "name")
	if description != "" {
		params.Add(jsonutils.NewString(description), "description")
	}
	return modules.RolesV3.Create(s, params)
}

func EnsureRole(s *mcclient.ClientSession, roleName, description string) (jsonutils.JSONObject, error) {
	return EnsureResource(s, &modules.RolesV3, roleName, func() (jsonutils.JSONObject, error) {
		return CreateRole(s, roleName, description)
	})
}

func IsServiceExists(s *mcclient.ClientSession, svcName string) (jsonutils.JSONObject, bool, error) {
	return IsResourceExists(s, &modules.ServicesV3, svcName)
}

func EnsureService(s *mcclient.ClientSession, svcName, svcType string) (jsonutils.JSONObject, error) {
	return EnsureResource(s, &modules.ServicesV3, svcName, func() (jsonutils.JSONObject, error) {
		return CreateService(s, svcName, svcType)
	})
}

func EnsureServiceCertificate(s *mcclient.ClientSession, certName string, certDetails *jsonutils.JSONDict) (jsonutils.JSONObject, error) {
	return EnsureResource(s, &modules.ServiceCertificatesV3, certName, func() (jsonutils.JSONObject, error) {
		return CreateServiceCertificate(s, certName, certDetails)
	})
}

func CreateServiceCertificate(s *mcclient.ClientSession, certName string, certDetails *jsonutils.JSONDict) (jsonutils.JSONObject, error) {
	certDetails.Add(jsonutils.NewString(certName), "name")
	return modules.ServiceCertificatesV3.Create(s, certDetails)
}

func CreateService(s *mcclient.ClientSession, svcName, svcType string) (jsonutils.JSONObject, error) {
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(svcType), "type")
	params.Add(jsonutils.NewString(svcName), "name")
	params.Add(jsonutils.JSONTrue, "enabled")
	return modules.ServicesV3.Create(s, params)
}

func IsEndpointExists(s *mcclient.ClientSession, svcId, regionId, interfaceType string) (jsonutils.JSONObject, bool, error) {
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(svcId), "service_id")
	params.Add(jsonutils.NewString(regionId), "region_id")
	params.Add(jsonutils.NewString(interfaceType), "interface")
	eps, err := modules.EndpointsV3.List(s, params)
	if err != nil {
		return nil, false, err
	}
	if len(eps.Data) == 0 {
		return nil, false, nil
	}
	return eps.Data[0], true, nil
}

func EnsureEndpoint(
	s *mcclient.ClientSession, svcId, regionId, interfaceType, url, serviceCert string,
) (jsonutils.JSONObject, error) {
	ep, exists, err := IsEndpointExists(s, svcId, regionId, interfaceType)
	if err != nil {
		return nil, err
	}
	if !exists {
		createParams := jsonutils.NewDict()
		createParams.Add(jsonutils.NewString(svcId), "service_id")
		createParams.Add(jsonutils.NewString(regionId), "region_id")
		createParams.Add(jsonutils.NewString(interfaceType), "interface")
		createParams.Add(jsonutils.NewString(url), "url")
		createParams.Add(jsonutils.JSONTrue, "enabled")
		if len(serviceCert) > 0 {
			createParams.Add(jsonutils.NewString(serviceCert), "service_certificate")
		}
		return modules.EndpointsV3.Create(s, createParams)
	}
	epId, err := ep.GetString("id")
	if err != nil {
		return nil, err
	}
	epUrl, _ := ep.GetString("url")
	enabled, _ := ep.Bool("enabled")
	if epUrl == url && enabled {
		// same endpoint exists and already exists
		return ep, nil
	}
	updateParams := jsonutils.NewDict()
	updateParams.Add(jsonutils.NewString(url), "url")
	updateParams.Add(jsonutils.JSONTrue, "enabled")
	if len(serviceCert) > 0 {
		updateParams.Add(jsonutils.NewString(serviceCert), "service_certificate")
	}
	return modules.EndpointsV3.Update(s, epId, updateParams)
}

func IsUserExists(s *mcclient.ClientSession, username string) (jsonutils.JSONObject, bool, error) {
	return IsResourceExists(s, &modules.UsersV3, username)
}

func CreateUser(s *mcclient.ClientSession, username string, password string) (jsonutils.JSONObject, error) {
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(username), "name")
	params.Add(jsonutils.NewString(password), "password")
	params.Add(jsonutils.JSONTrue, "is_system_account")
	return modules.UsersV3.Create(s, params)
}

func ChangeUserPassword(s *mcclient.ClientSession, username string, password string) (jsonutils.JSONObject, error) {
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(password), "password")
	return modules.UsersV3.Update(s, username, params)
}

func ProjectAddUser(s *mcclient.ClientSession, projectId string, userId string, roleId string) error {
	_, err := modules.RolesV3.PutInContexts(s, roleId, nil,
		[]modulebase.ManagerContext{
			{InstanceManager: &modules.Projects, InstanceId: projectId},
			{InstanceManager: &modules.UsersV3, InstanceId: userId},
		})
	return err
}

func IsZoneExists(s *mcclient.ClientSession, zone string) (jsonutils.JSONObject, bool, error) {
	return IsResourceExists(s, &modules.Zones, zone)
}

func CreateZone(s *mcclient.ClientSession, zone string) (jsonutils.JSONObject, error) {
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(zone), "name")
	return modules.Zones.Create(s, params)
}

func IsWireExists(s *mcclient.ClientSession, wire string) (jsonutils.JSONObject, bool, error) {
	return IsResourceExists(s, &modules.Wires, wire)
}

func CreateWire(s *mcclient.ClientSession, zone string, wire string, bw int, vpc string) (jsonutils.JSONObject, error) {
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(wire), "name")
	params.Add(jsonutils.NewInt(int64(bw)), "bandwidth")
	params.Add(jsonutils.NewString(vpc), "vpc")
	return modules.Wires.CreateInContext(s, params, &modules.Zones, zone)
}

func IsNetworkExists(s *mcclient.ClientSession, net string) (jsonutils.JSONObject, bool, error) {
	return IsResourceExists(s, &modules.Networks, net)
}

func CreateNetwork(
	s *mcclient.ClientSession,
	name string,
	gateway string,
	serverType string,
	wireId string,
	maskLen int,
	startIp string,
	endIp string,
) (jsonutils.JSONObject, error) {
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(name), "name")
	params.Add(jsonutils.NewString(startIp), "guest_ip_start")
	params.Add(jsonutils.NewString(endIp), "guest_ip_end")
	params.Add(jsonutils.NewInt(int64(maskLen)), "guest_ip_mask")
	if gateway != "" {
		params.Add(jsonutils.NewString(gateway), "guest_gateway")
	}
	if serverType != "" {
		params.Add(jsonutils.NewString(serverType), "server_type")
	}
	return modules.Networks.CreateInContext(s, params, &modules.Wires, wireId)
}

func NetworkPrivate(s *mcclient.ClientSession, name string) (jsonutils.JSONObject, error) {
	return modules.Networks.PerformAction(s, "private", name, nil)
}

func CreateRegion(s *mcclient.ClientSession, region, zone string) (jsonutils.JSONObject, error) {
	if zone != "" {
		region = mcclient.RegionID(region, zone)
	}
	obj, err := modules.Regions.Get(s, region, nil)
	if err == nil {
		// region already exists
		return obj, nil
	}
	if !IsNotFoundError(err) {
		return nil, err
	}
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(region), "id")
	return modules.Regions.Create(s, params)
}

func IsSchedtagExists(s *mcclient.ClientSession, name string) (jsonutils.JSONObject, bool, error) {
	return IsResourceExists(s, &modules.Schedtags, name)
}

func CreateSchedtag(s *mcclient.ClientSession, name string, strategy string, description string) (jsonutils.JSONObject, error) {
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(name), "name")
	params.Add(jsonutils.NewString(strategy), "default_strategy")
	params.Add(jsonutils.NewString(description), "description")
	return modules.Schedtags.Create(s, params)
}

func EnsureSchedtag(s *mcclient.ClientSession, name string, strategy string, description string) (jsonutils.JSONObject, error) {
	return EnsureResource(s, &modules.Schedtags, name, func() (jsonutils.JSONObject, error) {
		return CreateSchedtag(s, name, strategy, description)
	})
}

func IsDynamicSchedtagExists(s *mcclient.ClientSession, name string) (jsonutils.JSONObject, bool, error) {
	return IsResourceExists(s, &modules.Dynamicschedtags, name)
}

func CreateDynamicSchedtag(s *mcclient.ClientSession, name, schedtag, condition string) (jsonutils.JSONObject, error) {
	params := jsonutils.NewDict()
	params.Add(jsonutils.NewString(name), "name")
	params.Add(jsonutils.NewString(schedtag), "schedtag")
	params.Add(jsonutils.NewString(condition), "condition")
	params.Add(jsonutils.JSONTrue, "enabled")
	return modules.Dynamicschedtags.Create(s, params)
}

func EnsureDynamicSchedtag(s *mcclient.ClientSession, name, schedtag, condition string) (jsonutils.JSONObject, error) {
	return EnsureResource(s, &modules.Dynamicschedtags, name, func() (jsonutils.JSONObject, error) {
		return CreateDynamicSchedtag(s, name, schedtag, condition)
	})
}

func GetEndpointsByService(s *mcclient.ClientSession, serviceName string) ([]jsonutils.JSONObject, error) {
	obj, err := modules.ServicesV3.Get(s, serviceName, nil)
	if err != nil {
		return nil, err
	}
	svcId, _ := obj.GetString("id")
	searchParams := jsonutils.NewDict()
	searchParams.Add(jsonutils.NewString(svcId), "service_id")
	ret, err := modules.EndpointsV3.List(s, searchParams)
	if err != nil {
		return nil, err
	}
	return ret.Data, nil
}

func DisableService(s *mcclient.ClientSession, id string) error {
	params := jsonutils.NewDict()
	params.Add(jsonutils.JSONFalse, "enabled")
	_, err := modules.ServicesV3.Patch(s, id, params)
	return err
}

func DisableEndpoint(s *mcclient.ClientSession, id string) error {
	params := jsonutils.NewDict()
	params.Add(jsonutils.JSONFalse, "enabled")
	_, err := modules.EndpointsV3.Patch(s, id, params)
	return err
}

func DeleteServiceEndpoints(s *mcclient.ClientSession, serviceName string) error {
	endpoints, err := GetEndpointsByService(s, serviceName)
	if err != nil {
		if IsNotFoundError(err) {
			return nil
		}
		return err
	}
	for _, ep := range endpoints {
		id, _ := ep.GetString("id")
		tmpId := id
		if err := DisableEndpoint(s, tmpId); err != nil {
			return err
		}
		if _, err := modules.EndpointsV3.Delete(s, id, nil); err != nil {
			return err
		}
	}
	if err := DisableService(s, serviceName); err != nil {
		return err
	}
	return DeleteResource(s, &modules.ServicesV3, serviceName)
}

func InitServiceAccount(s *mcclient.ClientSession, username string, password string) error {
	obj, exists, err := IsUserExists(s, username)
	if err != nil {
		return err
	}
	if exists {
		id, _ := obj.GetString("id")
		if _, err := ChangeUserPassword(s, id, password); err != nil {
			return errors.Wrapf(err, "user %s already exists, update password", username)
		}
		return nil
	}
	obj, err = CreateUser(s, username, password)
	if err != nil {
		return errors.Wrapf(err, "create user %s", username)
	}
	userId, _ := obj.GetString("id")
	return ProjectAddUser(s, constants.SysAdminProject, userId, constants.RoleAdmin)
}

func RegisterServiceEndpoints(
	s *mcclient.ClientSession,
	regionId string,
	serviceName string,
	serviceType string,
	serviceCert string,
	interfaceUrls map[string]string,
) error {
	svc, err := EnsureService(s, serviceName, serviceType)
	if err != nil {
		return err
	}
	svcId, err := svc.GetString("id")
	if err != nil {
		return err
	}
	errgrp := &errgroup.Group{}
	for inf, endpointUrl := range interfaceUrls {
		tmpInf := inf
		tmpUrl := endpointUrl
		errgrp.Go(func() error {
			_, err = EnsureEndpoint(s, svcId, regionId, tmpInf, tmpUrl, serviceCert)
			if err != nil {
				return err
			}
			return nil
		})
	}
	return errgrp.Wait()
}

func RegisterServiceEndpointByInterfaces(
	s *mcclient.ClientSession,
	regionId string,
	serviceName string,
	serviceType string,
	endpointUrl string,
	serviceCert string,
	interfaces []string,
) error {
	urls := make(map[string]string)
	for _, inf := range interfaces {
		urls[inf] = endpointUrl
	}
	return RegisterServiceEndpoints(s, regionId, serviceName, serviceType, serviceCert, urls)
}

func RegisterServicePublicInternalEndpoint(
	s *mcclient.ClientSession,
	regionId string,
	serviceName string,
	serviceType string,
	serviceCert string,
	endpointUrl string,
) error {
	return RegisterServiceEndpointByInterfaces(s, regionId, serviceName, serviceType,
		endpointUrl, serviceCert, []string{constants.EndpointTypeInternal, constants.EndpointTypePublic})
}

func ToPlaybook(
	hostLines []string,
	mods []string,
	files map[string]string,
) (*ansible.Playbook, error) {
	if len(mods) == 0 {
		return nil, errors.Errorf("Requires at least one mod")
	}
	if len(hostLines) == 0 {
		return nil, errors.Errorf("Requires as least one server/host to operator on")
	}
	pb := ansible.NewPlaybook()
	hosts := []ansible.Host{}
	for _, s := range hostLines {
		host, err := ansible.ParseHostLine(s)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	pb.Inventory = ansible.Inventory{Hosts: hosts}
	for _, s := range mods {
		module, err := ansible.ParseModuleLine(s)
		if err != nil {
			return nil, err
		}
		pb.Modules = append(pb.Modules, module)
	}
	pb.Files = make(map[string][]byte)
	for name, data := range files {
		pb.Files[name] = []byte(data)
	}
	return pb, nil
}

func DevtoolTemplateCreateParams(
	name string,
	hostLines []string,
	mods []string,
	files map[string]string,
	day int64,
	hour int64,
	min int64,
	sec int64,
	interval int64,
	start bool,
	enabled bool,
) (*jsonutils.JSONDict, error) {
	pb, err := ToPlaybook(hostLines, mods, files)
	if err != nil {
		return nil, err
	}
	input := ansibleapi.AnsiblePlaybookCreateInput{
		Name:     name,
		Playbook: *pb,
	}
	params := input.JSON(input)
	params.Add(jsonutils.NewInt(day), "day")
	params.Add(jsonutils.NewInt(hour), "hour")
	params.Add(jsonutils.NewInt(min), "min")
	params.Add(jsonutils.NewInt(sec), "sec")
	params.Add(jsonutils.NewInt(interval), "interval")
	params.Add(jsonutils.NewBool(start), "start")
	params.Add(jsonutils.NewBool(enabled), "enabled")
	return params, nil
}

func CreateDevtoolTemplate(
	s *mcclient.ClientSession,
	name string,
	hosts []string,
	mods []string,
	files map[string]string,
	interval int64,
) (jsonutils.JSONObject, error) {
	params, err := DevtoolTemplateCreateParams(name, hosts, mods, files, 0, 0, 0, 0, interval, false, true)
	if err != nil {
		return nil, errors.Wrapf(err, "get devtool template %s create params", name)
	}
	return modules.DevToolTemplates.Create(s, params)
}

func EnsureDevtoolTemplate(
	s *mcclient.ClientSession,
	name string,
	hosts []string,
	mods []string,
	files map[string]string,
	interval int64,
) (jsonutils.JSONObject, error) {
	return EnsureResource(s, &modules.DevToolTemplates, name, func() (jsonutils.JSONObject, error) {
		return CreateDevtoolTemplate(s, name, hosts, mods, files, interval)
	})
}

func SyncServiceConfig(
	s *mcclient.ClientSession, syncConf map[string]string, serviceName string,
) (jsonutils.JSONObject, error) {
	iconf, err := modules.ServicesV3.GetSpecific(s, serviceName, "config", nil)
	if err != nil {
		return nil, err
	}
	conf := iconf.(*jsonutils.JSONDict)
	if !conf.Contains("config") {
		conf.Add(jsonutils.NewDict(), "config")
	}
	if !conf.Contains("config", "default") {
		conf.Add(jsonutils.NewDict(), "config", "default")
	}
	for k, v := range syncConf {
		if _, ok := conf.GetString("config", "default", k); ok == nil {
			continue
		} else {
			conf.Add(jsonutils.NewString(v), "config", "default", k)
		}
	}
	return modules.ServicesV3.PerformAction(s, serviceName, "config", conf)
}

type CommonAlertTem struct {
	Database    string `json:"database"`
	Measurement string `json:"measurement"`
	//rule operator rule [and|or]
	Operator  string   `json:"operator"`
	Field     []string `json:"field"`
	FieldFunc string   `json:"field_func"`

	Reduce        string
	Comparator    string  `json:"comparator"`
	Threshold     float64 `json:"threshold"`
	Filters       []monitorapi.MetricQueryTag
	FieldOpt      string `json:"field_opt"`
	GetPointStr   bool   `json:"get_point_str"`
	Name          string
	ConditionType string `json:"condition_type"`
	From          string `json:"from"`
	Interval      string `json:"interval"`
}

func GetCommonAlertOfSys(session *mcclient.ClientSession) ([]jsonutils.JSONObject, error) {
	param := jsonutils.NewDict()
	param.Add(jsonutils.NewBool(true), "details")
	param.Add(jsonutils.NewString(monitorapi.CommonAlertSystemAlertType), "alert_type")
	param.Add(jsonutils.NewString("system"), "scope")

	rtn, err := modules.CommonAlertManager.List(session, param)
	if err != nil {
		return nil, err
	}
	return rtn.Data, nil
}

func CreateCommonAlert(s *mcclient.ClientSession, tem CommonAlertTem) (jsonutils.JSONObject, error) {
	commonAlert := newCommonalertQuery(tem)
	input := monitorapi.CommonAlertCreateInput{
		CommonMetricInputQuery: monitorapi.CommonMetricInputQuery{
			MetricQuery: []*monitorapi.CommonAlertQuery{&commonAlert},
		},
		AlertCreateInput: monitorapi.AlertCreateInput{
			Name:  tem.Name,
			Level: "important",
		},
		Recipients: []string{monitorapi.CommonAlertDefaultRecipient},
		AlertType:  monitorapi.CommonAlertSystemAlertType,
		Scope:      "system",
	}
	if len(tem.From) != 0 {
		input.From = tem.From
	}
	if len(tem.Interval) != 0 {
		input.Interval = tem.Interval
	}
	param := jsonutils.Marshal(&input)
	if tem.GetPointStr {
		param.(*jsonutils.JSONDict).Set("get_point_str", jsonutils.JSONTrue)
	}
	param.(*jsonutils.JSONDict).Set("meta_name", jsonutils.NewString(tem.Name))
	return modules.CommonAlertManager.Create(s, param)
}

func UpdateCommonAlert(s *mcclient.ClientSession, tem CommonAlertTem, id string) (jsonutils.JSONObject, error) {
	commonAlert := newCommonalertQuery(tem)
	input := monitorapi.CommonAlertUpdateInput{
		CommonMetricInputQuery: monitorapi.CommonMetricInputQuery{
			MetricQuery: []*monitorapi.CommonAlertQuery{&commonAlert},
		},
	}
	if len(tem.From) != 0 {
		input.From = tem.From
	}
	if len(tem.Interval) != 0 {
		input.Interval = tem.Interval
	}
	param := jsonutils.Marshal(&input)
	param.(*jsonutils.JSONDict).Set("force_update", jsonutils.JSONTrue)
	if tem.GetPointStr {
		param.(*jsonutils.JSONDict).Set("get_point_str", jsonutils.JSONTrue)
	}
	param.(*jsonutils.JSONDict).Set("meta_name", jsonutils.NewString(tem.Name))
	return modules.CommonAlertManager.Update(s, id, param)
}

func DeleteCommonAlert(s *mcclient.ClientSession, ids []string) {
	modules.CommonAlertManager.BatchDelete(s, ids, jsonutils.NewDict())
}

func newCommonalertQuery(tem CommonAlertTem) monitorapi.CommonAlertQuery {
	metricQ := monitorapi.MetricQuery{
		Alias:        "",
		Tz:           "",
		Database:     tem.Database,
		Measurement:  tem.Measurement,
		Tags:         make([]monitorapi.MetricQueryTag, 0),
		GroupBy:      nil,
		Selects:      nil,
		Interval:     "",
		Policy:       "",
		ResultFormat: "",
	}

	for _, field := range tem.Field {
		sel := monitorapi.MetricQueryPart{
			Type:   "field",
			Params: []string{field},
		}
		selectPart := []monitorapi.MetricQueryPart{sel}
		if len(tem.FieldFunc) != 0 {
			selectPart = append(selectPart, monitorapi.MetricQueryPart{
				Type:   tem.FieldFunc,
				Params: []string{},
			})
			if tem.GetPointStr {
				selectPart = append(selectPart, monitorapi.MetricQueryPart{
					Type:   "alias",
					Params: []string{field},
				})
			}
		}
		metricQ.Selects = append(metricQ.Selects, selectPart)
	}
	if len(tem.Filters) != 0 {
		for _, filter := range tem.Filters {
			metricQ.Tags = append(metricQ.Tags, filter)
		}
	}

	alertQ := new(monitorapi.AlertQuery)
	alertQ.Model = metricQ
	alertQ.From = "60m"

	commonAlert := monitorapi.CommonAlertQuery{
		AlertQuery: alertQ,
		Reduce:     tem.Reduce,
		Comparator: tem.Comparator,
		Threshold:  tem.Threshold,
	}
	if tem.FieldOpt != "" {
		commonAlert.FieldOpt = monitorapi.CommonAlertFieldOpt_Division
	}
	if len(tem.ConditionType) != 0 {
		commonAlert.ConditionType = tem.ConditionType
	}
	return commonAlert
}
