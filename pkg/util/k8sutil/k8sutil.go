package k8sutil

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/pborman/uuid"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	api "yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/util/etcdutil"
)

const (
	// EtcdClientPort is the client port on client service and etcd nodes.
	EtcdClientPort                     = 2379
	TolerateUnreadyEndpointsAnnotation = "service.alpha.kubernetes.io/tolerate-unready-endpoints"

	etcdVolumeMountDir       = "/var/etcd"
	dataDir                  = etcdVolumeMountDir + "/data"
	backupFile               = "/var/etcd/latest.backup"
	etcdVersionAnnotationKey = "etcd.version"
	peerTLSDir               = "/etc/etcdtls/member/peer-tls"
	peerTLSVolume            = "member-peer-tls"
	serverTLSDir             = "/etc/etcdtls/member/server-tls"
	serverTLSVolume          = "member-server-tls"
	operatorEtcdTLSDir       = "/etc/etcdtls/operator/etcd-tls"
	operatorEtcdTLSVolume    = "etcd-client-tls"

	randomSuffixLength = 10
	// k8s object name has a maximum length
	MaxNameLength = 63 - randomSuffixLength - 1

	defaultBusyboxImage = "busybox:1.28.0-glibc"

	// AnnotationScope annotation name for defining instance scope. Used for specifying cluster wide clusters.
	AnnotationScope = "etcd.database.coreos.com/scope"
	//AnnotationClusterWide annotation value for cluster wide clusters.
	AnnotationClusterWide = "clusterwide"

	// defaultDNSTimeout is the default maximum allowed time for the init container of the etcd pod
	// to reverse DNS lookup its IP. The default behavior is to wait forever and has a value of 0.
	defaultDNSTimeout = int64(0)
)

func UniqueMemberName(clusterName string) string {
	suffix := utilrand.String(randomSuffixLength)
	if len(clusterName) > MaxNameLength {
		clusterName = clusterName[:MaxNameLength]
	}
	return clusterName + "-" + suffix
}

func IsKubernetesResourceNotFoundError(err error) bool {
	return apierrors.IsNotFound(err)
}

func GetEtcdVersion(pod *v1.Pod) string {
	return pod.Annotations[etcdVersionAnnotationKey]
}

func ImageName(repo, version string) string {
	return fmt.Sprintf("%s:v%v", repo, version)
}

func SetEtcdVersion(pod *v1.Pod, version string) {
	pod.Annotations[etcdVersionAnnotationKey] = version
}

func GetPodNames(pods []*v1.Pod) []string {
	if len(pods) == 0 {
		return nil
	}
	res := []string{}
	for _, p := range pods {
		res = append(res, p.Name)
	}
	return res
}

func CreatePatch(o, n, datastruct interface{}) ([]byte, error) {
	oldData, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	newData, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return strategicpatch.CreateTwoWayMergePatch(oldData, newData, datastruct)
}

func NewEtcdPod(m *etcdutil.Member, initialCluster []string, clusterName, state, token string, cs api.EtcdClusterSpec, owner metav1.OwnerReference) *v1.Pod {
	pod := newEtcdPod(m, initialCluster, clusterName, state, token, cs)
	applyPodPolicy(clusterName, pod, cs.Pod)
	addOwnerRefToObject(pod.GetObjectMeta(), owner)
	return pod
}

func addOwnerRefToObject(o metav1.Object, r metav1.OwnerReference) {
	o.SetOwnerReferences(append(o.GetOwnerReferences(), r))
}

// AddEtcdVolumeToPod abstract the process of appending volume spec to pod spec
func AddEtcdVolumeToPod(pod *v1.Pod, pvc *v1.PersistentVolumeClaim) {
	vol := v1.Volume{Name: etcdVolumeName}
	if pvc != nil {
		vol.VolumeSource = v1.VolumeSource{
			PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name},
		}
	} else {
		vol.VolumeSource = v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}}
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, vol)
}

// NewSeedMemberPod returns a Pod manifest for a seed member.
// It's special that it has new token, and might need recovery init containers
func NewSeedMemberPod(clusterName string, ms etcdutil.MemberSet, m *etcdutil.Member, cs api.EtcdClusterSpec, owner metav1.OwnerReference, backupURL *url.URL) *v1.Pod {
	token := uuid.New()
	pod := newEtcdPod(m, ms.PeerURLPairs(), clusterName, "new", token, cs)
	// TODO: PVC datadir support for restore process
	AddEtcdVolumeToPod(pod, nil)
	if backupURL != nil {
		addRecoveryToPod(pod, token, m, cs, backupURL)
	}
	applyPodPolicy(clusterName, pod, cs.Pod)
	addOwnerRefToObject(pod.GetObjectMeta(), owner)
	return pod
}

func addRecoveryToPod(pod *v1.Pod, token string, m *etcdutil.Member, cs api.EtcdClusterSpec, backupURL *url.URL) {
	pod.Spec.InitContainers = append(pod.Spec.InitContainers,
		makeRestoreInitContainers(backupURL, token, cs.Repository, cs.Version, m)...)
}

// NewEtcdPodPVC create PVC object from etcd pod's PVC spec
func NewEtcdPodPVC(m *etcdutil.Member, pvcSpec v1.PersistentVolumeClaimSpec, clusterName, namespace string, owner metav1.OwnerReference) *v1.PersistentVolumeClaim {
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PVCNameFromMember(m.Name),
			Namespace: namespace,
			Labels:    LabelsForCluster(clusterName),
		},
		Spec: pvcSpec,
	}
	addOwnerRefToObject(pvc.GetObjectMeta(), owner)
	return pvc
}

// We are using internal api types for cluster related.
func ClusterListOpt(clusterName string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(LabelsForCluster(clusterName)).String(),
	}
}

func LabelsForCluster(clusterName string) map[string]string {
	return map[string]string{
		"etcd_cluster": clusterName,
		"app":          "etcd",
	}
}

// PVCNameFromMember the way we get PVC name from the member name
func PVCNameFromMember(memberName string) string {
	return memberName
}

func makeRestoreInitContainers(backupURL *url.URL, token, repo, version string, m *etcdutil.Member) []v1.Container {
	return []v1.Container{
		{
			Name:  "fetch-backup",
			Image: "tutum/curl",
			Command: []string{
				"/bin/bash", "-ec",
				fmt.Sprintf(`
httpcode=$(curl --write-out %%\{http_code\} --silent --output %[1]s %[2]s)
if [[ "$httpcode" != "200" ]]; then
	echo "http status code: ${httpcode}" >> /dev/termination-log
	cat %[1]s >> /dev/termination-log
	exit 1
fi
					`, backupFile, backupURL.String()),
			},
			VolumeMounts: etcdVolumeMounts(),
		},
		{
			Name:  "restore-datadir",
			Image: ImageName(repo, version),
			Command: []string{
				"/bin/sh", "-ec",
				fmt.Sprintf("ETCDCTL_API=3 etcdctl snapshot restore %[1]s"+
					" --name %[2]s"+
					" --initial-cluster %[2]s=%[3]s"+
					" --initial-cluster-token %[4]s"+
					" --initial-advertise-peer-urls %[3]s"+
					" --data-dir %[5]s 2>/dev/termination-log", backupFile, m.Name, m.PeerURL(), token, dataDir),
			},
			VolumeMounts: etcdVolumeMounts(),
		},
	}
}

func newEtcdPod(m *etcdutil.Member, initialCluster []string, clusterName, state, token string, cs api.EtcdClusterSpec) *v1.Pod {
	commands := fmt.Sprintf("/usr/local/bin/etcd --data-dir=%s --name=%s --initial-advertise-peer-urls=%s "+
		"--listen-peer-urls=%s --listen-client-urls=%s --advertise-client-urls=%s "+
		"--initial-cluster=%s --initial-cluster-state=%s",
		dataDir, m.Name, m.PeerURL(), m.ListenPeerURL(), m.ListenClientURL(), m.ClientURL(), strings.Join(initialCluster, ","), state)
	if m.SecurePeer {
		commands += fmt.Sprintf(" --peer-client-cert-auth=true --peer-trusted-ca-file=%[1]s/peer-ca.crt --peer-cert-file=%[1]s/peer.crt --peer-key-file=%[1]s/peer.key", peerTLSDir)
	}
	if m.SecureClient {
		commands += fmt.Sprintf(" --client-cert-auth=true --trusted-ca-file=%[1]s/server-ca.crt --cert-file=%[1]s/server.crt --key-file=%[1]s/server.key", serverTLSDir)
	}
	if state == "new" {
		commands = fmt.Sprintf("%s --initial-cluster-token=%s", commands, token)
	}

	labels := map[string]string{
		"app":          "etcd",
		"etcd_node":    m.Name,
		"etcd_cluster": clusterName,
	}

	livenessProbe := newEtcdProbe(cs.TLS.IsSecureClient())
	readinessProbe := newEtcdProbe(cs.TLS.IsSecureClient())
	readinessProbe.InitialDelaySeconds = 1
	readinessProbe.TimeoutSeconds = 5
	readinessProbe.PeriodSeconds = 5
	readinessProbe.FailureThreshold = 3

	container := containerWithProbes(
		etcdContainer(strings.Split(commands, " "), cs.Repository, cs.Version),
		livenessProbe,
		readinessProbe)

	volumes := []v1.Volume{}

	if m.SecurePeer {
		container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
			MountPath: peerTLSDir,
			Name:      peerTLSVolume,
		})
		volumes = append(volumes, v1.Volume{Name: peerTLSVolume, VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{SecretName: cs.TLS.Static.Member.PeerSecret},
		}})
	}
	if m.SecureClient {
		container.VolumeMounts = append(container.VolumeMounts, v1.VolumeMount{
			MountPath: serverTLSDir,
			Name:      serverTLSVolume,
		}, v1.VolumeMount{
			MountPath: operatorEtcdTLSDir,
			Name:      operatorEtcdTLSVolume,
		})
		volumes = append(volumes, v1.Volume{Name: serverTLSVolume, VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{SecretName: cs.TLS.Static.Member.ServerSecret},
		}}, v1.Volume{Name: operatorEtcdTLSVolume, VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{SecretName: cs.TLS.Static.OperatorSecret},
		}})
	}

	DNSTimeout := defaultDNSTimeout
	if cs.Pod != nil {
		DNSTimeout = cs.Pod.DNSTimeoutInSecond
	}

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        m.Name,
			Labels:      labels,
			Annotations: map[string]string{},
		},
		Spec: v1.PodSpec{
			InitContainers: []v1.Container{{
				// busybox:latest uses uclibc which contains a bug that sometimes prevents name resolution
				// More info: https://github.com/docker-library/busybox/issues/27
				//Image default: "busybox:1.28.0-glibc",
				Image: imageNameBusybox(cs.Pod),
				Name:  "check-dns",
				// In etcd 3.2, TLS listener will do a reverse-DNS lookup for pod IP -> hostname.
				// If DNS entry is not warmed up, it will return empty result and peer connection will be rejected.
				// In some cases the DNS is not created correctly so we need to time out after a given period.
				Command: []string{"/bin/sh", "-c", fmt.Sprintf(`
TIMEOUT_READY=%d
while ( ! nslookup %s )
do
	# If TIMEOUT_READY is 0 we should never time out and exit
	TIMEOUT_READY=$(( TIMEOUT_READY-1 ))
	if [ $TIMEOUT_READY -eq 0 ];
	then
		echo "Timed out waiting for DNS entry"
		exit 1
	fi
	sleep 1
	done`, DNSTimeout, m.Addr())},
			}},
			Containers:    []v1.Container{container},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes:       volumes,
			// DNS A record: `[m.Name].[clusterName].Namespace.svc`
			// For example, etcd-795649v9kq in default namesapce will have DNS name
			// `etcd-795649v9kq.etcd.default.svc`.
			Hostname:                     m.Name,
			Subdomain:                    clusterName,
			AutomountServiceAccountToken: func(b bool) *bool { return &b }(false),
			SecurityContext:              podSecurityContext(cs.Pod),
		},
	}
	SetEtcdVersion(pod, cs.Version)
	return pod
}

// imageNameBusybox returns the default image for busybox init container, or the image specified in the PodPolicy
func imageNameBusybox(policy *api.PodPolicy) string {
	if policy != nil && len(policy.BusyboxImage) > 0 {
		return policy.BusyboxImage
	}
	return defaultBusyboxImage
}

func podSecurityContext(podPolicy *api.PodPolicy) *v1.PodSecurityContext {
	if podPolicy == nil {
		return nil
	}
	return podPolicy.SecurityContext
}

func PodWithNodeSelector(p *v1.Pod, ns map[string]string) *v1.Pod {
	p.Spec.NodeSelector = ns
	return p
}
