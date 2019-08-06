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

package component

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"text/template"

	"github.com/pkg/errors"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"

	"yunion.io/x/onecloud/pkg/cloudcommon/options"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/pkg/util/reflectutils"
	"yunion.io/x/structarg"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/mysql"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

const (
	// LastAppliedConfigAnnotation is annotation key of last applied configuration
	LastAppliedConfigAnnotation = "onecloud.yunion.io/last-applied-configuration"
	// ImagePullBackOff is the pod state of image pull failed
	ImagePullBackOff = "ImagePullBackOff"
	// ErrImagePull is the pod state of image pull failed
	ErrImagePull = "ErrImagePull"
)

// templateEqual compares the new podTemplateSpec's spec with old podTemplateSpec's last applied config
func templateEqual(new corev1.PodTemplateSpec, old corev1.PodTemplateSpec) bool {
	oldConfig := corev1.PodSpec{}
	if lastAppliedConfig, ok := old.Annotations[LastAppliedConfigAnnotation]; ok {
		err := json.Unmarshal([]byte(lastAppliedConfig), &oldConfig)
		if err != nil {
			klog.Errorf("unmarshal PodTemplate: [%s/%s]'s applied config failed,error: %v", old.GetNamespace(), old.GetName(), err)
			return false
		}
		return apiequality.Semantic.DeepEqual(oldConfig, new.Spec)
	}
	return false
}

// SetServiceLastAppliedConfigAnnotation set last applied config info to Service's annotation
func SetServiceLastAppliedConfigAnnotation(svc *corev1.Service) error {
	svcApply, err := encode(svc.Spec)
	if err != nil {
		return err
	}
	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	svc.Annotations[LastAppliedConfigAnnotation] = svcApply
	return nil
}

func SetIngressLastAppliedConfigAnnotation(ing *extensions.Ingress) error {
	ingApply, err := encode(ing.Spec)
	if err != nil {
		return err
	}
	if ing.Annotations == nil {
		ing.Annotations = map[string]string{}
	}
	ing.Annotations[LastAppliedConfigAnnotation] = ingApply
	return nil
}

func SetConfigMapLastAppliedConfigAnnotation(cfg *corev1.ConfigMap) error {
	cfgApply, err := encode(cfg.Data)
	if err != nil {
		return err
	}
	if cfg.Annotations == nil {
		cfg.Annotations = map[string]string{}
	}
	cfg.Annotations[LastAppliedConfigAnnotation] = cfgApply
	return nil
}

func SetDeploymentLastAppliedConfigAnnotation(deploy *apps.Deployment) error {
	deployApply, err := encode(deploy.Spec)
	if err != nil {
		return err
	}
	if deploy.Annotations == nil {
		deploy.Annotations = map[string]string{}
	}
	deploy.Annotations[LastAppliedConfigAnnotation] = deployApply

	templateApply, err := encode(deploy.Spec.Template.Spec)
	if err != nil {
		return err
	}
	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = map[string]string{}
	}
	deploy.Spec.Template.Annotations[LastAppliedConfigAnnotation] = templateApply
	return nil
}

// serviceEqual compares the new Service's spec with old Service's last applied config
func serviceEqual(new, old *corev1.Service) (bool, error) {
	oldSpec := corev1.ServiceSpec{}
	if lastAppliedConfig, ok := old.Annotations[LastAppliedConfigAnnotation]; ok {
		err := json.Unmarshal([]byte(lastAppliedConfig), &oldSpec)
		if err != nil {
			klog.Errorf("unmarshal ServiceSpec: [%s/%s]'s applied config failed,error: %v", old.GetNamespace(), old.GetName(), err)
			return false, err
		}
		return apiequality.Semantic.DeepEqual(oldSpec, new.Spec), nil
	}
	return false, nil
}

func ingressEqual(new, old *extensions.Ingress) (bool, error) {
	oldSpec := extensions.IngressSpec{}
	if lastAppliedConfig, ok := old.Annotations[LastAppliedConfigAnnotation]; ok {
		err := json.Unmarshal([]byte(lastAppliedConfig), &oldSpec)
		if err != nil {
			return false, err
		}
		return apiequality.Semantic.DeepEqual(oldSpec, new.Spec), nil
	}
	return false, nil
}

func configMapEqual(new, old *corev1.ConfigMap) (bool, error) {
	oldData := map[string]string{}
	if lastAppliedConfig, ok := old.Annotations[LastAppliedConfigAnnotation]; ok {
		err := json.Unmarshal([]byte(lastAppliedConfig), &oldData)
		if err != nil {
			return false, err
		}
		return reflect.DeepEqual(oldData, new.Data), nil
	}
	return false, nil
}

func deploymentEqual(new apps.Deployment, old apps.Deployment) bool {
	oldConfig := apps.DeploymentSpec{}
	if lastAppliedConfig, ok := old.Annotations[LastAppliedConfigAnnotation]; ok {
		err := json.Unmarshal([]byte(lastAppliedConfig), &oldConfig)
		if err != nil {
			klog.Errorf("unmarshal Deployment: [%s/%s]'s applied config failed, error: %v", old.GetNamespace(), old.GetName(), err)
			return false
		}
		return apiequality.Semantic.DeepEqual(oldConfig.Replicas, new.Spec.Replicas) &&
			apiequality.Semantic.DeepEqual(oldConfig.Template, new.Spec.Template) &&
			apiequality.Semantic.DeepEqual(oldConfig.Strategy, new.Spec.Strategy)
	}
	return false
}

func encode(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func deploymentIsUpgrading(deploy *apps.Deployment) bool {
	if deploy.Status.ObservedGeneration == 0 {
		return false
	}
	if deploy.Generation > deploy.Status.ObservedGeneration && *deploy.Spec.Replicas == deploy.Status.Replicas {
		return true
	}
	return false
}

// CombineAnnotations merges two annotations maps
func CombineAnnotations(a, b map[string]string) map[string]string {
	if a == nil {
		a = make(map[string]string)
	}
	for k, v := range b {
		a[k] = v
	}
	return a
}

func CreateOrUpdateConfigMap(client clientset.Interface, cm *corev1.ConfigMap) error {
	return apiclient.CreateOrUpdateConfigMap(client, cm)
}

func GetDBConnectionByCluster(oc *v1alpha1.OnecloudCluster) (*mysql.Connection, error) {
	return mysql.NewConnection(&oc.Spec.Mysql)
}

func EnsureClusterDBUser(oc *v1alpha1.OnecloudCluster, dbConfig v1alpha1.DBConfig) error {
	dbName := dbConfig.Database
	username := dbConfig.Username
	password := dbConfig.Password
	conn, err := GetDBConnectionByCluster(oc)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := EnsureDBUser(conn, dbName, username, password); err != nil {
		return err
	}
	return nil
}

func EnsureDBUser(conn *mysql.Connection, dbName string, username string, password string) error {
	dbExists, err := conn.IsDatabaseExists(dbName)
	if err != nil {
		return errors.Wrap(err, "check db exists")
	}
	if !dbExists {
		if err := conn.CreateDatabase(dbName); err != nil {
			return errors.Wrapf(err, "create database %q", dbName)
		}
	}
	if err := conn.CreateUser(username, password, dbName); err != nil {
		return errors.Wrapf(err, "create user %q for database %q", username, dbName)
	}
	return nil
}

func LoginByServiceAccount(s *mcclient.ClientSession, account v1alpha1.CloudUser) (mcclient.TokenCredential, error) {
	return s.GetClient().AuthenticateWithSource(account.Username, account.Password, constants.DefaultDomain, constants.SysAdminProject, "", "operator")
}

func EnsureServiceAccount(s *mcclient.ClientSession, account v1alpha1.CloudUser) error {
	username := account.Username
	password := account.Password
	obj, exists, err := onecloud.IsUserExists(s, username)
	if err != nil {
		return err
	}
	if exists {
		// password not change
		if _, err := LoginByServiceAccount(s, account); err == nil {
			return nil
		}
		id, _ := obj.GetString("id")
		if _, err := onecloud.ChangeUserPassword(s, id, password); err != nil {
			return errors.Wrapf(err, "user %s already exists, update password", username)
		}
		return nil
	}
	obj, err = onecloud.CreateUser(s, username, password)
	if err != nil {
		return errors.Wrapf(err, "create user %s", username)
	}
	userId, _ := obj.GetString("id")
	return onecloud.ProjectAddUser(s, constants.SysAdminProject, userId, constants.RoleAdmin)
}

func SetOptionsDefault(opt interface{}, serviceType string) error {
	parser, err := structarg.NewArgumentParser(opt, serviceType, "", "")
	if err != nil {
		return err
	}
	parser.SetDefault()

	var optionsRef *options.BaseOptions
	if err := reflectutils.FindAnonymouStructPointer(opt, &optionsRef); err != nil {
		return err
	}
	if len(optionsRef.ApplicationID) == 0 {
		optionsRef.ApplicationID = serviceType
	}
	return nil
}

type VolumeHelper struct {
	cluster      *v1alpha1.OnecloudCluster
	optionCfgMap string
	component    v1alpha1.ComponentType
	volumes      []corev1.Volume
	volumeMounts []corev1.VolumeMount
}

func NewVolumeHelper(oc *v1alpha1.OnecloudCluster, optCfgMap string, component v1alpha1.ComponentType) *VolumeHelper {
	h := &VolumeHelper{
		cluster:      oc,
		optionCfgMap: optCfgMap,
		component:    component,
		volumes:      make([]corev1.Volume, 0),
		volumeMounts: make([]corev1.VolumeMount, 0),
	}
	h.volumes = []corev1.Volume{
		{
			Name: constants.VolumeCertsName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: controller.ClustercertSecretName(h.cluster),
					Items: []corev1.KeyToPath{
						{Key: constants.CACertName, Path: constants.CACertName},
						{Key: constants.ServiceCertName, Path: constants.ServiceCertName},
						{Key: constants.ServiceKeyName, Path: constants.ServiceKeyName},
					},
				},
			},
		},
	}
	h.volumeMounts = append(h.volumeMounts, corev1.VolumeMount{Name: constants.VolumeCertsName, ReadOnly: true, MountPath: constants.CertDir})

	if h.optionCfgMap != "" {
		cfgVol := corev1.Volume{
			Name: constants.VolumeConfigName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: h.optionCfgMap,
					},
					Items: []corev1.KeyToPath{
						{Key: constants.VolumeConfigName, Path: fmt.Sprintf("%s.conf", h.component)},
					},
				},
			},
		}
		h.volumes = append(h.volumes, cfgVol)
		h.volumeMounts = append(h.volumeMounts, corev1.VolumeMount{Name: constants.VolumeConfigName, ReadOnly: true, MountPath: constants.ConfigDir})
	}
	return h
}

func (h *VolumeHelper) GetVolumes() []corev1.Volume {
	return h.volumes
}

func (h *VolumeHelper) GetVolumeMounts() []corev1.VolumeMount {
	return h.volumeMounts
}

func NewServiceNodePort(name string, port int32) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       name,
		Protocol:   corev1.ProtocolTCP,
		Port:       port,
		TargetPort: intstr.FromInt(int(port)),
		NodePort:   port,
	}
}

func SetOptionsServiceTLS(config *options.BaseOptions) {
	enableConfigTLS(config, constants.CertDir, constants.CACertName, constants.ServiceCertName, constants.ServiceKeyName)
}

func enableConfigTLS(config *options.BaseOptions, certDir string, ca string, cert string, key string) {
	config.EnableSsl = true
	config.SslCaCerts = path.Join(certDir, ca)
	config.SslCertfile = path.Join(certDir, cert)
	config.SslKeyfile = path.Join(certDir, key)
}

func SetServiceBaseOptions(opt *options.BaseOptions, region string, input v1alpha1.ServiceBaseConfig) {
	opt.Region = region
	opt.Port = input.Port
}

func SetServiceCommonOptions(opt *options.CommonOptions, oc *v1alpha1.OnecloudCluster, input v1alpha1.ServiceCommonOptions) {
	SetServiceBaseOptions(&opt.BaseOptions, oc.Spec.Region, input.ServiceBaseConfig)
	opt.AuthURL = controller.GetAuthURL(oc)
	opt.AdminUser = input.CloudUser.Username
	opt.AdminDomain = constants.DefaultDomain
	opt.AdminPassword = input.CloudUser.Password
	opt.AdminProject = constants.SysAdminProject
}

func SetDBOptions(opt *options.DBOptions, mysql v1alpha1.Mysql, input v1alpha1.DBConfig) {
	opt.SqlConnection = fmt.Sprintf("mysql+pymysql://%s:%s@%s:%d/%s?charset=utf8", input.Username, input.Password, mysql.Host, mysql.Port, input.Database)
}

func CompileTemplateFromMap(tmplt string, configMap interface{}) (string, error) {
	out := new(bytes.Buffer)
	t := template.Must(template.New("compiled_template").Parse(tmplt))
	if err := t.Execute(out, configMap); err != nil {
		return "", err
	}
	return out.String(), nil
}
