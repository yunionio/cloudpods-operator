package component

import (
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
)

const (
	RepoConfigTemplate = `
server {
  listen       {{ .Port }};
  server_name  localhost;

  #charset koi8-r;
  #access_log  /var/log/nginx/host.access.log  main;

  location / {
      root   /usr/share/nginx/html;
      index  index.html index.htm;
      autoindex on;
  }

  #error_page  404              /404.html;

  # redirect server error pages to the static page /50x.html
  #
  error_page   500 502 503 504  /50x.html;
  location = /50x.html {
      root   /usr/share/nginx/html;
  }

  # proxy the PHP scripts to Apache listening on 127.0.0.1:80
  #
  #location ~ \.php$ {
  #    proxy_pass   http://127.0.0.1;
  #}

  # pass the PHP scripts to FastCGI server listening on 127.0.0.1:9000
  #
  #location ~ \.php$ {
  #    root           html;
  #    fastcgi_pass   127.0.0.1:9000;
  #    fastcgi_index  index.php;
  #    fastcgi_param  SCRIPT_FILENAME  /scripts$fastcgi_script_name;
  #    include        fastcgi_params;
  #}

  # deny access to .htaccess files, if Apache's document root
  # concurs with nginx's one
  #
  #location ~ /\.ht {
  #    deny  all;
  #}
}
`
)

type RepoConfig struct {
	Port int
}

func (c RepoConfig) GetContent() (string, error) {
	return CompileTemplateFromMap(RepoConfigTemplate, c)
}

type repoManager struct {
	*ComponentManager
}

func newRepoManager(man *ComponentManager) manager.Manager {
	return &repoManager{man}
}

func (r *repoManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	return syncComponent(r, oc, oc.Spec.Repo.Disable, "")
}

func (r *repoManager) getPhaseControl(man controller.ComponentManager, zone string) controller.PhaseControl {
	return controller.NewRegisterEndpointComponentWithSsl(
		man, v1alpha1.RepoComponentType,

		constants.ServiceNameRepo, constants.ServiceTypeRepo,
		constants.RepoPort, "", false,
	)
}

func (r *repoManager) getService(oc *v1alpha1.OnecloudCluster, zone string) []*corev1.Service {
	return []*corev1.Service{r.newSingleNodePortService(v1alpha1.RepoComponentType, oc, constants.RepoPort)}
}

func (r *repoManager) getConfigMap(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*corev1.ConfigMap, bool, error) {
	config := RepoConfig{
		Port: constants.RepoPort,
	}
	content, err := config.GetContent()
	if err != nil {
		return nil, false, err
	}
	return r.newConfigMap(v1alpha1.RepoComponentType, "", oc, content), false, nil
}

func (r *repoManager) getDeployment(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig, zone string) (*apps.Deployment, error) {
	cfgMapname := controller.ComponentConfigMapName(oc, v1alpha1.RepoComponentType)
	pluginCfgVol := corev1.Volume{
		Name: constants.ServiceNameRepo,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cfgMapname,
				},
				Items: []corev1.KeyToPath{
					{Key: "config", Path: "default.conf"},
				},
			},
		},
	}
	deploy, err := m.newCloudServiceSinglePortDeployment(v1alpha1.RepoComponentType, "", oc, &oc.Spec.Repo, constants.RepoPort, false, false)
	if err != nil {
		return nil, err
	}
	spec := &deploy.Spec.Template.Spec
	spec.Containers[0].Command = nil
	spec.Containers[0].VolumeMounts = append(spec.Containers[0].VolumeMounts,
		corev1.VolumeMount{
			MountPath: "/etc/nginx/conf.d",
			Name:      constants.ServiceNameRepo,
		})
	spec.Volumes = append(spec.Volumes, pluginCfgVol)
	return deploy, nil
}

func (r *repoManager) getDeploymentStatus(oc *v1alpha1.OnecloudCluster, zone string) *v1alpha1.DeploymentStatus {
	return &oc.Status.Repo
}
