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
	batchv1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/klog"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	deploymentutil "k8s.io/kubernetes/pkg/kubectl/util/deployment"

	"yunion.io/x/pkg/util/reflectutils"
	"yunion.io/x/structarg"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/mysql"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
	"yunion.io/x/onecloud/pkg/cloudcommon/options"
	"yunion.io/x/onecloud/pkg/mcclient"
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

func SetDaemonSetLastAppliedConfigAnnotation(ds *apps.DaemonSet) error {
	dsApply, err := encode(ds.Spec)
	if err != nil {
		return err
	}
	if ds.Annotations == nil {
		ds.Annotations = map[string]string{}
	}
	ds.Annotations[LastAppliedConfigAnnotation] = dsApply
	templateApply, err := encode(ds.Spec.Template.Spec)
	if err != nil {
		return err
	}
	if ds.Spec.Template.Annotations == nil {
		ds.Spec.Template.Annotations = map[string]string{}
	}
	ds.Spec.Template.Annotations[LastAppliedConfigAnnotation] = templateApply
	return nil
}

func SetCronJobLastAppliedConfigAnnotation(cronJob *batchv1.CronJob) error {
	cronApply, err := encode(cronJob.Spec)
	if err != nil {
		return err
	}
	if cronJob.Annotations == nil {
		cronJob.Annotations = map[string]string{}
	}
	cronJob.Annotations[LastAppliedConfigAnnotation] = cronApply
	templateApply, err := encode(cronJob.Spec.JobTemplate.Spec)
	if err != nil {
		return err
	}
	if cronJob.Spec.JobTemplate.Annotations == nil {
		cronJob.Spec.JobTemplate.Annotations = map[string]string{}
	}
	cronJob.Spec.JobTemplate.Annotations[LastAppliedConfigAnnotation] = templateApply
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
			klog.Errorf("unmarshal Deployment: [%s/%s]'s applied config failed, error: %v",
				old.GetNamespace(), old.GetName(), err)
			return false
		}
		return apiequality.Semantic.DeepEqual(oldConfig.Replicas, new.Spec.Replicas) &&
			apiequality.Semantic.DeepEqual(oldConfig.Template, new.Spec.Template) &&
			apiequality.Semantic.DeepEqual(oldConfig.Strategy, new.Spec.Strategy)
	}
	return false
}

func daemonSetEqual(new, old *apps.DaemonSet) bool {
	oldConfig := apps.DaemonSetSpec{}
	if lastAppliedConfig, ok := old.Annotations[LastAppliedConfigAnnotation]; ok {
		err := json.Unmarshal([]byte(lastAppliedConfig), &oldConfig)
		if err != nil {
			klog.Errorf("unmarshal DaemonSet: [%s/%s]'s applied config failed, error: %v",
				old.GetNamespace(), old.GetName(), err)
			return false
		}
		return apiequality.Semantic.DeepEqual(oldConfig.Template, new.Spec.Template) &&
			apiequality.Semantic.DeepEqual(oldConfig.UpdateStrategy, new.Spec.UpdateStrategy)
	}
	return false
}

func cronJobEqual(new, old *batchv1.CronJob) bool {
	oldConfig := batchv1.CronJob{}
	if LastAppliedConfig, ok := old.Annotations[LastAppliedConfigAnnotation]; ok {
		err := json.Unmarshal([]byte(LastAppliedConfig), &oldConfig.Spec)
		if err != nil {
			klog.Errorf("unmarshal CronJob: [%s/%s]'s applied config failed, error: %v",
				old.GetNamespace(), old.GetName(), err)
			return false
		}
		return apiequality.Semantic.DeepEqual(oldConfig.Spec.Schedule, new.Spec.Schedule) &&
			apiequality.Semantic.DeepEqual(oldConfig.Spec.JobTemplate.Spec.Template, new.Spec.JobTemplate.Spec.Template)
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

func deploymentIsRollout(deploy *apps.Deployment) (bool, string, error) {
	if deploy.Generation <= deploy.Status.ObservedGeneration {
		cond := deploymentutil.GetDeploymentCondition(deploy.Status, apps.DeploymentProgressing)
		if cond != nil && cond.Reason == deploymentutil.TimedOutReason {
			return false, "", fmt.Errorf("deployment %q exceeded its progress deadline", deploy.GetName())
		}
		if deploy.Spec.Replicas != nil && deploy.Status.UpdatedReplicas < *deploy.Spec.Replicas {
			return false, fmt.Sprintf("Waiting for deployment %q rollout to finish: %d out of %d new replicas have been updated...", deploy.GetName(), deploy.Status.UpdatedReplicas, *deploy.Spec.Replicas), nil
		}
		if deploy.Status.Replicas > deploy.Status.UpdatedReplicas {
			return false, fmt.Sprintf("Waiting for deployment %q rollout to finish: %d old replicas are pending termination...", deploy.GetName(), deploy.Status.Replicas-deploy.Status.UpdatedReplicas), nil
		}
		if deploy.Status.AvailableReplicas < deploy.Status.UpdatedReplicas {
			return false, fmt.Sprintf("Waiting for deployment %q rollout to finish: %d of %d updated replicas are available...", deploy.GetName(), deploy.Status.AvailableReplicas, deploy.Status.UpdatedReplicas), nil
		}
		return true, fmt.Sprintf("deployment %q successfully rolled out", deploy.GetName()), nil
	}
	return false, fmt.Sprintf("Waiting for deployment spec update to be observed..."), nil
}

func deploymentIsUpgrading(deploy *apps.Deployment) bool {
	//if deploy.Status.ObservedGeneration == 0 {
	//return false
	//}
	//if deploy.Generation > deploy.Status.ObservedGeneration && *deploy.Spec.Replicas == deploy.Status.Replicas {
	//return true
	//}
	rollout, reason, err := deploymentIsRollout(deploy)
	if rollout {
		return false
	}
	klog.Infof("Deployment %s is upgrading, reason %s, error: %v", deploy.GetName(), reason, err)
	return true
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
		if userProjectCnt, err := obj.Int("project_count"); err != nil {
			klog.Errorf("Get user %s project_count: %v", username, err)
		} else {
			if userProjectCnt == 0 {
				userId, _ := obj.GetString("id")
				if err := onecloud.ProjectAddUser(s, constants.SysAdminProject, userId, constants.RoleAdmin); err != nil {
					return errors.Wrapf(err, "add exists user %s to system project", username)
				}
			}
		}
		if !controller.SyncUser {
			return nil
		} else {
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
	h.volumeMounts = append(h.volumeMounts, corev1.VolumeMount{
		Name: constants.VolumeCertsName, ReadOnly: true, MountPath: constants.CertDir})

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

func NewVolumeHelperWithEtcdTLS(
	oc *v1alpha1.OnecloudCluster, optCfgMap string, component v1alpha1.ComponentType,
) *VolumeHelper {
	h := NewVolumeHelper(oc, optCfgMap, component)
	h.addEtcdClientTLSVolumes(oc)
	return h
}

func getVolumeMount(volumeMounts []corev1.VolumeMount, name string) *corev1.VolumeMount {
	for _, vm := range volumeMounts {
		if vm.Name == name {
			return &vm
		}
	}
	return nil
}

func GetConfigVolumeMount(volMounts []corev1.VolumeMount) *corev1.VolumeMount {
	return getVolumeMount(volMounts, constants.VolumeConfigName)
}

func (h *VolumeHelper) GetVolumes() []corev1.Volume {
	return h.volumes
}

func (h *VolumeHelper) GetVolumeMounts() []corev1.VolumeMount {
	return h.volumeMounts
}

func (h *VolumeHelper) addVmwareVolumes() *VolumeHelper {
	var (
		bidirectional = corev1.MountPropagationBidirectional
		volSrcType    = corev1.HostPathDirectoryOrCreate
	)
	h.volumeMounts = append(h.volumeMounts,
		corev1.VolumeMount{
			Name:             "var-run-vmware",
			MountPath:        "/var/run/vmware",
			MountPropagation: &bidirectional,
		},
	)
	h.volumes = append(h.volumes,
		corev1.Volume{
			Name: "var-run-vmware",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/vmware",
					Type: &volSrcType,
				},
			},
		},
	)
	return h
}

func (h *VolumeHelper) addOnecloudVolumes() *VolumeHelper {
	var (
		bidirectional = corev1.MountPropagationBidirectional
		volSrcType    = corev1.HostPathDirectoryOrCreate
	)
	h.volumeMounts = append(h.volumeMounts,
		corev1.VolumeMount{
			Name:             "var-run-onecloud",
			MountPath:        "/var/run/onecloud",
			MountPropagation: &bidirectional,
		},
	)
	h.volumes = append(h.volumes,
		corev1.Volume{
			Name: "var-run-onecloud",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/onecloud",
					Type: &volSrcType,
				},
			},
		},
	)
	return h
}

func (h *VolumeHelper) addEtcdClientTLSVolumes(oc *v1alpha1.OnecloudCluster) *VolumeHelper {
	if !oc.Spec.Etcd.Disable && oc.Spec.Etcd.EnableTls {
		h.volumes = append(h.volumes, corev1.Volume{
			Name: constants.EtcdClientSecret,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: constants.EtcdClientSecret},
			},
		})
		h.volumeMounts = append(h.volumeMounts, corev1.VolumeMount{
			MountPath: constants.EtcdClientTLSDir,
			Name:      constants.EtcdClientSecret,
			ReadOnly:  true,
		})
	}
	return h
}

func (h *VolumeHelper) addOvsVolumes() *VolumeHelper {
	volSrcType := corev1.HostPathDirectoryOrCreate
	h.volumes = append(h.volumes,
		corev1.Volume{
			Name: "var-run-openvswitch",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/openvswitch",
					Type: &volSrcType,
				},
			},
		},
		corev1.Volume{
			Name: "var-log-openvswitch",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/log/openvswitch",
					Type: &volSrcType,
				},
			},
		},
	)
	h.volumeMounts = append(h.volumeMounts,
		corev1.VolumeMount{
			Name:      "var-run-openvswitch",
			MountPath: "/var/run/openvswitch",
		},
		corev1.VolumeMount{
			Name:      "var-log-openvswitch",
			MountPath: "/var/log/openvswitch",
		},
	)
	return h
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

func SetOptionsServiceTLS(config *options.BaseOptions, disableTLS bool) {
	enableConfigTLS(disableTLS, config, constants.CertDir, constants.CACertName, constants.ServiceCertName, constants.ServiceKeyName)
}

func enableConfigTLS(disableTLS bool, config *options.BaseOptions, certDir string, ca string, cert string, key string) {
	config.EnableSsl = !disableTLS
	config.SslCaCerts = path.Join(certDir, ca)
	config.SslCertfile = path.Join(certDir, cert)
	config.SslKeyfile = path.Join(certDir, key)
}

func SetServiceBaseOptions(opt *options.BaseOptions, region string, input v1alpha1.ServiceBaseConfig) {
	opt.Region = region
	opt.Port = input.Port
}

func SetServiceCommonOptions(opt *options.CommonOptions, oc *v1alpha1.OnecloudCluster, input v1alpha1.ServiceCommonOptions) {
	SetServiceBaseOptions(&opt.BaseOptions, oc.GetRegion(), input.ServiceBaseConfig)
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

var (
	GetEdition          = v1alpha1.GetEdition
	IsEnterpriseEdition = v1alpha1.IsEnterpriseEdition
)

type PVCVolumePair struct {
	name      string
	mountPath string
	claimName string
	component v1alpha1.ComponentType
}

func NewPVCVolumePair(name, mountPath string, oc *v1alpha1.OnecloudCluster, comp v1alpha1.ComponentType) *PVCVolumePair {
	return &PVCVolumePair{
		name:      name,
		mountPath: mountPath,
		claimName: controller.NewClusterComponentName(oc.GetName(), comp),
		component: comp,
	}
}

func (p PVCVolumePair) GetVolume() corev1.Volume {
	return corev1.Volume{
		Name: p.name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: p.claimName,
				ReadOnly:  false,
			},
		},
	}
}

func (p PVCVolumePair) GetVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      p.name,
		MountPath: p.mountPath,
	}
}

func NewHostVolume(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	configMap string,
) *VolumeHelper {
	h := &VolumeHelper{
		cluster:      oc,
		optionCfgMap: configMap,
		component:    cType,
	}

	// volumes mounts
	var bidirectional = corev1.MountPropagationBidirectional
	h.volumeMounts = []corev1.VolumeMount{
		{
			Name:      "etc-yunion",
			ReadOnly:  false,
			MountPath: "/etc/yunion",
		},
		{
			Name:      constants.VolumeCertsName,
			ReadOnly:  true,
			MountPath: constants.CertDir,
		},
		{
			Name:      constants.VolumeConfigName,
			ReadOnly:  true,
			MountPath: path.Join(constants.ConfigDir, "common"),
		},
		{
			Name:             "cloud",
			ReadOnly:         false,
			MountPath:        "/opt/cloud",
			MountPropagation: &bidirectional,
		},
		{
			Name:      "usr",
			ReadOnly:  false,
			MountPath: "/usr/local",
		},
		/*
		 * {
		 * 	Name:      "proc",
		 * 	ReadOnly:  false,
		 * 	MountPath: "/proc",
		 * },
		 */
		{
			Name:      "dev",
			ReadOnly:  false,
			MountPath: "/dev",
		},
		{
			Name:      "sys",
			ReadOnly:  false,
			MountPath: "/sys",
		},
		{
			Name:             "tmp",
			ReadOnly:         false,
			MountPath:        "/tmp",
			MountPropagation: &bidirectional,
		},
	}

	// volumes
	var hostPathDirectory = corev1.HostPathDirectory
	h.volumes = []corev1.Volume{
		{
			Name: "etc-yunion",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/yunion",
					Type: &hostPathDirectory,
				},
			},
		},
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
		{
			Name: constants.VolumeConfigName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: h.optionCfgMap,
					},
					Items: []corev1.KeyToPath{
						{Key: constants.VolumeConfigName, Path: "common.conf"},
					},
				},
			},
		},
		{
			Name: "cloud",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/opt/cloud",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "usr",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/usr/local",
					Type: &hostPathDirectory,
				},
			},
		},
		/*
		 * {
		 * 	Name: "proc",
		 * 	VolumeSource: corev1.VolumeSource{
		 * 		HostPath: &corev1.HostPathVolumeSource{
		 * 			Path: "/proc",
		 * 			Type: &hostPathDirectory,
		 * 		},
		 * 	},
		 * },
		 */
		{
			Name: "dev",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/dev",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "sys",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/sys",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "tmp",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/tmp",
					Type: &hostPathDirectory,
				},
			},
		},
	}
	h.addOnecloudVolumes()
	h.addVmwareVolumes()
	h.addOvsVolumes()
	h.addEtcdClientTLSVolumes(oc)
	return h
}

func NewOvsVolumeHelper(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	configMap string,
) *VolumeHelper {
	h := &VolumeHelper{
		cluster:      oc,
		optionCfgMap: configMap,
		component:    cType,
	}
	h.addOvsVolumes()
	return h
}

func NewHostImageVolumeHelper(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	configMap string,
) *VolumeHelper {
	h := &VolumeHelper{
		cluster:      oc,
		optionCfgMap: configMap,
		component:    cType,
	}
	var hostPathDirectory = corev1.HostPathDirectory
	h.volumes = []corev1.Volume{
		{
			Name: "host-root",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/",
					Type: &hostPathDirectory,
				},
			},
		},
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
		{
			Name: constants.VolumeConfigName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: h.optionCfgMap,
					},
					Items: []corev1.KeyToPath{
						{Key: constants.VolumeConfigName, Path: "common.conf"},
					},
				},
			},
		},
	}
	h.volumeMounts = []corev1.VolumeMount{
		{
			Name:      "host-root",
			ReadOnly:  false,
			MountPath: YUNION_HOST_ROOT,
		},
		{
			Name:      constants.VolumeCertsName,
			ReadOnly:  true,
			MountPath: constants.CertDir,
		},
		{
			Name:      constants.VolumeConfigName,
			ReadOnly:  true,
			MountPath: path.Join(constants.ConfigDir, "common"),
		},
	}

	return h
}

func NewHostDeployerVolume(
	cType v1alpha1.ComponentType,
	oc *v1alpha1.OnecloudCluster,
	configMap string,
) *VolumeHelper {
	h := &VolumeHelper{
		cluster:      oc,
		optionCfgMap: configMap,
		component:    cType,
	}
	// volumes mounts
	var bidirectional = corev1.MountPropagationBidirectional
	h.volumeMounts = []corev1.VolumeMount{
		{
			Name:      "etc-yunion",
			ReadOnly:  false,
			MountPath: "/etc/yunion",
		},
		{
			Name:      constants.VolumeConfigName,
			ReadOnly:  true,
			MountPath: path.Join(constants.ConfigDir, "common"),
		},
		{
			Name:             "cloud",
			ReadOnly:         false,
			MountPath:        "/opt/cloud",
			MountPropagation: &bidirectional,
		},
		{
			Name:      "usr",
			ReadOnly:  false,
			MountPath: "/usr/local",
		},
		{
			Name:      "dev",
			ReadOnly:  false,
			MountPath: "/dev",
		},
		{
			Name:      "sys",
			ReadOnly:  false,
			MountPath: "/sys",
		},
	}
	// volumes
	var hostPathDirectory = corev1.HostPathDirectory
	var hostPathDirectoryOrCreate = corev1.HostPathDirectoryOrCreate
	h.volumes = []corev1.Volume{
		{
			Name: "etc-yunion",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/yunion",
					Type: &hostPathDirectoryOrCreate,
				},
			},
		},
		{
			Name: constants.VolumeConfigName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: h.optionCfgMap,
					},
					Items: []corev1.KeyToPath{
						{Key: constants.VolumeConfigName, Path: "common.conf"},
					},
				},
			},
		},
		{
			Name: "cloud",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/opt/cloud",
					Type: &hostPathDirectoryOrCreate,
				},
			},
		},
		{
			Name: "dev",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/dev",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "usr",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/usr/local",
					Type: &hostPathDirectory,
				},
			},
		},
		{
			Name: "sys",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/sys",
					Type: &hostPathDirectory,
				},
			},
		},
	}
	h.addOnecloudVolumes()
	h.addVmwareVolumes()
	return h
}
