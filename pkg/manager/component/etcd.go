package component

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"path"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/pborman/uuid"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"yunion.io/x/jsonutils"
	"yunion.io/x/log"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/manager"
	"yunion.io/x/onecloud-operator/pkg/util/etcdutil"
	"yunion.io/x/onecloud-operator/pkg/util/k8sutil"
	"yunion.io/x/onecloud-operator/pkg/util/retryutil"
)

type etcdManager struct {
	*ComponentManager

	lock          sync.Mutex
	syncing       bool
	reconcileLock sync.Mutex

	oc     *v1alpha1.OnecloudCluster
	status v1alpha1.EctdStatus

	// members repsersents the members in the etcd cluster.
	// the name of the member is the the name of the pod the member
	// process runs in.
	members etcdutil.MemberSet

	tlsConfig *tls.Config
	defraging bool
}

const (
	peerTLSDir         = "/etc/etcdtls/member/peer-tls"
	serverTLSDir       = "/etc/etcdtls/member/server-tls"
	operatorEtcdTLSDir = "/etc/etcdtls/operator/etcd-tls"

	etcdVolumeMountDir = "/var/etcd"
	dataDir            = etcdVolumeMountDir + "/data"
	etcdVolumeName     = "etcd-data"

	etcdBackendQuotaSize        = 128 * 1024 * 1024 // 128M
	etcdAutoCompactionRetention = 1                 // 1 hour
	etcdMaxWALFileCount         = 1
)

var (
	etcdMan *etcdManager

	reconcileInterval         = 8 * time.Second
	podTerminationGracePeriod = int64(5)
	ErrLostQuorum             = errors.Error("lost quorum")
	errCreatedCluster         = errors.Error("etcd cluster failed to be created")
)

func newEtcdComponentManager(baseMan *ComponentManager) manager.Manager {
	if etcdMan == nil {
		etcdMan = &etcdManager{
			ComponentManager: baseMan,
		}
		go etcdMan.defrag()
	}
	return etcdMan
}

func (m *etcdManager) getProductVersions() []v1alpha1.ProductVersion {
	return []v1alpha1.ProductVersion{
		v1alpha1.ProductVersionFullStack,
		v1alpha1.ProductVersionCMP,
		v1alpha1.ProductVersionEdge,
		v1alpha1.ProductVersionLightEdge,
	}
}

func (m *etcdManager) isSyncing() bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.syncing
}

func (m *etcdManager) setSyncing() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.syncing = true
}

func (m *etcdManager) setUnsync() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.syncing = false
}

func (m *etcdManager) fixEtcdSize(oc *v1alpha1.OnecloudCluster) (bool, error) {
	// list nodes by master node selector
	masterNodeSelector := labels.NewSelector()
	r, err := labels.NewRequirement(
		constants.LabelNodeRoleMaster, selection.Exists, nil)
	if err != nil {
		return false, err
	}
	masterNodeSelector = masterNodeSelector.Add(*r)

	nodes, err := m.nodeLister.List(masterNodeSelector)
	if err != nil {
		return false, err
	}
	masterCount := len(nodes)
	log.Infof("Master node count %d", masterCount)

	oldSize := oc.Spec.Etcd.Size
	if masterCount < 3 {
		oc.Spec.Etcd.Size = 1
	} else {
		if oc.Spec.Etcd.Size < constants.EtcdDefaultClusterSize {
			oc.Spec.Etcd.Size = constants.EtcdDefaultClusterSize
		}
	}
	return oc.Spec.Etcd.Size != oldSize, nil
}

func (m *etcdManager) GetComponentType() v1alpha1.ComponentType {
	return v1alpha1.EtcdComponentType
}

func (m *etcdManager) IsDisabled(oc *v1alpha1.OnecloudCluster) bool {
	return oc.Spec.Etcd.Disable
}

func (m *etcdManager) Sync(oc *v1alpha1.OnecloudCluster) error {
	err := syncComponent(m, oc, "")
	if err != nil {
		return err
	}
	if oc.Spec.Etcd.Disable {
		return nil
	}
	changed, err := m.fixEtcdSize(oc)
	if err != nil {
		log.Errorf("fix etcd size failed %s", err)
		return nil
	}
	if len(oc.Spec.Etcd.Version) == 0 {
		changed = true
		oc.Spec.Etcd.Version = constants.EtcdImageVersion
	}
	if oc.Spec.Etcd.ClientNodePort == 0 {
		changed = true
		oc.Spec.Etcd.ClientNodePort = constants.EtcdClientNodePort
	}
	if changed {
		oc, err = m.onecloudClusterControl.UpdateCluster(oc, nil, nil, "etcd-sync")
		if err != nil {
			log.Errorf("update oc failed %s", err)
			return nil
		}
	}

	if !m.isSyncing() {
		go m.sync(oc)
	}
	return nil
}

func (m *etcdManager) sync(oc *v1alpha1.OnecloudCluster) {
	m.setSyncing()
	defer m.setUnsync()

	m.reconcileLock.Lock()
	defer m.reconcileLock.Unlock()

	m.oc = oc
	m.status = *oc.Status.Etcd.DeepCopy()
	if err := m.setup(); err != nil {
		if m.status.Phase != v1alpha1.EtcdClusterPhaseFailed {
			m.status.Phase = v1alpha1.EtcdClusterPhaseFailed
			m.status.Reason = err.Error()
			if err := m.updateEtcdStatus(); err != nil {
				log.Errorf("failed to update etcd status %s", err)
			}
		}
		log.Errorf("setup etcd cluster failed: %s", err)
		return
	}
	m.run()
}

func (m *etcdManager) defrag() {
	if m.defraging {
		return
	}
	m.defraging = true
	for {
		select {
		case <-time.After(time.Minute * 3):
			m.membersDefrag()
		}
	}
}

func (m *etcdManager) membersDefrag() {
	cfg := clientv3.Config{
		Endpoints:   m.members.ClientURLs(),
		DialTimeout: constants.EtcdDefaultDialTimeout,
		TLS:         m.tlsConfig,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		log.Errorf("members defrag failed: creating etcd client failed %v", err)
		return
	}
	defer etcdcli.Close()

	for _, mx := range m.members {
		ctx, cancel := context.WithTimeout(context.Background(), constants.EtcdDefaultRequestTimeout)
		defer cancel()
		status, err := etcdcli.Status(ctx, mx.ClientURL())
		if err != nil {
			log.Errorf("fetch etcd status failed: %s", err)
			return
		}
		if status.DbSize > etcdBackendQuotaSize/2 {
			ctx, cancel = context.WithTimeout(context.Background(), constants.EtcdDefaultRequestTimeout)
			defer cancel()
			_, err = etcdcli.Compact(ctx, status.Header.Revision)
			if err != nil {
				log.Errorf("etcd cluster compact failed: %s", err)
				return
			}
			for _, m := range m.members {
				ctx, cancel = context.WithTimeout(context.Background(), constants.EtcdDefaultRequestTimeout)
				defer cancel()
				_, err := etcdcli.Defragment(ctx, m.ClientURL())
				if err != nil {
					log.Errorf("member %s defrag failed: %s", m.Name, err)
				}
			}
		}
		break
	}

}

func (m *etcdManager) isSecure() bool {
	return m.oc.Spec.Etcd.EnableTls == nil || *m.oc.Spec.Etcd.EnableTls
}

func (m *etcdManager) setup() error {
	var shouldCreateCluster bool
	switch m.status.Phase {
	case v1alpha1.EtcdClusterPhaseNone, v1alpha1.EtcdClusterPhaseFailed:
		shouldCreateCluster = true
	case v1alpha1.EtcdClusterPhaseCreating:
		return errCreatedCluster
	case v1alpha1.EtcdClusterPhaseRunning:
		shouldCreateCluster = false
	default:
		return fmt.Errorf("unexpected cluster phase: %s", m.status.Phase)
	}

	log.Infof("start etcd setup ......")

	if m.isSecure() {
		d, err := k8sutil.GetTLSDataFromSecret(m.kubeCli, m.oc.GetNamespace(), constants.EtcdClientSecret)
		if err != nil {
			return err
		}
		m.tlsConfig, err = etcdutil.NewTLSConfig(d.CertData, d.KeyData, d.CAData)
		if err != nil {
			return err
		}
	}

	if shouldCreateCluster {
		return m.create()
	}
	return nil
}

func (m *etcdManager) create() error {
	m.status.Phase = v1alpha1.EtcdClusterPhaseCreating
	if err := m.updateEtcdStatus(); err != nil {
		return fmt.Errorf("etcd cluster create: failed to update cluster phase %v, %v", v1alpha1.EtcdClusterPhaseCreating, err)
	}
	log.Infof("Start create cluster %v", m.oc.Spec.Etcd)
	return m.prepareSeedMember()
}

func (m *etcdManager) prepareSeedMember() error {
	err := m.bootstrap()
	if err != nil {
		return err
	}
	m.status.Size = 1
	return nil
}

func (m *etcdManager) bootstrap() error {
	return m.startSeedMember()
}

func (m *etcdManager) startSeedMember() error {
	mb := &etcdutil.Member{
		Name:         k8sutil.UniqueMemberName(m.getEtcdClusterPrefix()),
		Namespace:    m.oc.GetNamespace(),
		SecurePeer:   m.isSecure(),
		SecureClient: m.isSecure(),
		IPv6Cluster:  m.oc.Spec.IPv6Cluster,
	}
	ms := etcdutil.NewMemberSet(mb)
	if err := m.createPod(ms, mb, "new"); err != nil {
		return fmt.Errorf("failed to create seed member (%s): %v", mb.Name, err)
	}
	m.members = ms
	log.Infof("cluster created with seed member (%s)", mb.Name)
	return nil
}

func (m *etcdManager) isPodPVEnabled() bool {
	if podPolicy := m.oc.Spec.Etcd.Pod; podPolicy != nil {
		return podPolicy.PersistentVolumeClaimSpec != nil
	}
	return false
}

func (m *etcdManager) getEtcdClusterPrefix() string {
	return fmt.Sprintf("%s-etcd", m.oc.GetName())
}

func (m *etcdManager) createPod(
	members etcdutil.MemberSet, mb *etcdutil.Member, state string,
) error {
	token := uuid.New()
	initCluster := members.PeerURLPairs()
	pod := k8sutil.NewEtcdPod(mb, initCluster, m.getEtcdClusterPrefix(), state,
		token, m.customEtcdSpec(), controller.GetOwnerRef(m.oc))
	m.customPodSpec(pod, mb, state, token, initCluster)
	if m.isPodPVEnabled() {
		pvc := k8sutil.NewEtcdPodPVC(mb, *m.oc.Spec.Etcd.Pod.PersistentVolumeClaimSpec,
			m.oc.GetName(), m.oc.GetNamespace(), controller.GetOwnerRef(m.oc))
		_, err := m.kubeCli.CoreV1().PersistentVolumeClaims(m.oc.GetNamespace()).Create(context.Background(), pvc, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create PVC for member (%s): %v", mb.Name, err)
		}
		addEtcdVolumeToPod(pod, pvc)
	} else {
		addEtcdVolumeToPod(pod, nil)
	}
	_, err := m.kubeCli.CoreV1().Pods(m.oc.GetNamespace()).Create(context.Background(), pod, metav1.CreateOptions{})
	return err
}

func (m *etcdManager) customPodSpec(pod *corev1.Pod, mb *etcdutil.Member, state, token string, initialCluster []string) {
	var imageRepository, version = m.oc.Spec.ImageRepository, constants.EtcdImageVersion
	if len(m.oc.Spec.Etcd.Repository) > 0 {
		imageRepository = m.oc.Spec.Etcd.Repository
	}
	if len(m.oc.Spec.Etcd.Version) > 0 {
		version = m.oc.Spec.Etcd.Version
	}
	pod.Spec.Containers[0].Image = fmt.Sprintf("%s:%s", path.Join(imageRepository, constants.EtcdImageName), version)
	pod.Spec.Containers[0].Command = m.newEtcdCommand(mb, state, token, initialCluster)
	pod.Spec.Containers[0].LivenessProbe = m.newLivenessProbe(m.isSecure())
	pod.Spec.Containers[0].ReadinessProbe = m.newReadinessProbe(m.isSecure())
	shareProcessNamespace := true
	pod.Spec.ShareProcessNamespace = &shareProcessNamespace

	pod.Spec.InitContainers[0].Image = fmt.Sprintf("%s:%s",
		path.Join(imageRepository, constants.BusyboxImageName), constants.BusyboxImageVersion)

	pod.Spec.DNSPolicy = corev1.DNSClusterFirst
	if pod.Spec.Tolerations == nil {
		pod.Spec.Tolerations = []corev1.Toleration{}
	}
	pod.Spec.Tolerations = append(pod.Spec.Tolerations, []corev1.Toleration{
		{
			Key:    "node-role.kubernetes.io/master",
			Effect: corev1.TaintEffectNoSchedule,
		},
		{
			Key:    "node-role.kubernetes.io/controlplane",
			Effect: corev1.TaintEffectNoSchedule,
		},
	}...)
	if pod.Spec.NodeSelector == nil {
		pod.Spec.NodeSelector = make(map[string]string)
	}
	if !controller.DisableNodeSelectorController {
		pod.Spec.NodeSelector[constants.OnecloudControllerLabelKey] = "enable"
	}
	if pod.Spec.Affinity == nil {
		pod.Spec.Affinity = new(corev1.Affinity)
	}
	pod.Spec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "etcd"},
				},
				TopologyKey: "kubernetes.io/hostname",
			},
		},
	}
}

func (m *etcdManager) newLivenessProbe(isSecure bool) *corev1.Probe {
	tlsFlags := fmt.Sprintf("--cert=%[1]s/%[2]s --key=%[1]s/%[3]s --cacert=%[1]s/%[4]s",
		operatorEtcdTLSDir, etcdutil.CliCertFile, etcdutil.CliKeyFile, etcdutil.CliCAFile)
	cmd := "ETCDCTL_API=3 etcdctl endpoint status"
	if isSecure {
		cmd = fmt.Sprintf("ETCDCTL_API=3 etcdctl --endpoints=https://localhost:%d %s endpoint status",
			constants.EtcdClientPort, tlsFlags)
	}
	return &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{"/bin/sh", "-ec", cmd},
			},
		},
		InitialDelaySeconds: 10,
		TimeoutSeconds:      10,
		PeriodSeconds:       60,
		FailureThreshold:    3,
	}
}

func (m *etcdManager) newReadinessProbe(isSecure bool) *corev1.Probe {
	cmd := "ETCDCTL_API=3 etcdctl endpoint status"
	if isSecure {
		tlsFlags := fmt.Sprintf("--cert=%[1]s/%[2]s --key=%[1]s/%[3]s --cacert=%[1]s/%[4]s", operatorEtcdTLSDir, etcdutil.CliCertFile, etcdutil.CliKeyFile, etcdutil.CliCAFile)
		cmd = fmt.Sprintf("ETCDCTL_API=3 etcdctl --endpoints=https://localhost:%d %s endpoint status", constants.EtcdClientPort, tlsFlags)
	}
	return &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{"/bin/sh", "-ec", cmd},
			},
		},
		InitialDelaySeconds: 1,
		TimeoutSeconds:      5,
		PeriodSeconds:       5,
		FailureThreshold:    3,
	}
}

func (m *etcdManager) newEtcdCommand(mb *etcdutil.Member, state, token string, initialCluster []string) []string {
	commands := fmt.Sprintf("/usr/local/bin/etcd --data-dir=%s --name=%s --initial-advertise-peer-urls=%s "+
		"--listen-peer-urls=%s --listen-client-urls=%s --advertise-client-urls=%s "+
		"--initial-cluster=%s --initial-cluster-state=%s "+
		"--quota-backend-bytes %d --auto-compaction-retention %d "+
		"--max-wals %d",
		dataDir, mb.Name, mb.PeerURL(), mb.ListenPeerURL(), mb.ListenClientURL(),
		mb.ClientURL(), strings.Join(initialCluster, ","), state,
		etcdBackendQuotaSize, etcdAutoCompactionRetention, etcdMaxWALFileCount)
	if mb.SecurePeer {
		commands += fmt.Sprintf(" --peer-client-cert-auth=true --peer-trusted-ca-file=%[1]s/peer-ca.crt --peer-cert-file=%[1]s/peer.crt --peer-key-file=%[1]s/peer.key", peerTLSDir)
	}
	if mb.SecureClient {
		commands += fmt.Sprintf(" --client-cert-auth=true --trusted-ca-file=%[1]s/server-ca.crt --cert-file=%[1]s/server.crt --key-file=%[1]s/server.key --cipher-suites=TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305", serverTLSDir)
	}
	if state == "new" {
		commands = fmt.Sprintf("%s --initial-cluster-token=%s", commands, token)
	}
	return strings.Split(commands, " ")
}

func (m *etcdManager) removePod(name string) error {
	ns := m.oc.GetNamespace()
	opts := metav1.NewDeleteOptions(podTerminationGracePeriod)
	err := m.kubeCli.CoreV1().Pods(ns).Delete(context.Background(), name, *opts)
	if err != nil {
		if !k8sutil.IsKubernetesResourceNotFoundError(err) {
			return err
		}
	}
	return nil
}

func (m *etcdManager) pollPods() (running, pending, failed []*corev1.Pod, err error) {
	podList, err := m.kubeCli.CoreV1().Pods(m.oc.GetNamespace()).List(context.Background(), k8sutil.ClusterListOpt(m.getEtcdClusterPrefix()))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to list running pods: %v", err)
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		// Avoid polling deleted pods. k8s issue where deleted pods would sometimes show the status Pending
		// See https://github.com/coreos/etcd-operator/issues/1693
		if pod.DeletionTimestamp != nil {
			continue
		}
		if len(pod.OwnerReferences) < 1 {
			log.Warningf("pollPods: ignore pod %v: no owner", pod.Name)
			continue
		}
		if pod.OwnerReferences[0].UID != m.oc.UID {
			log.Warningf("pollPods: ignore pod %v: owner (%v) is not %v",
				pod.Name, pod.OwnerReferences[0].UID, m.oc.UID)
			continue
		}
		switch pod.Status.Phase {
		case corev1.PodRunning:
			running = append(running, pod)
		case corev1.PodPending:
			pending = append(pending, pod)
		case corev1.PodFailed, corev1.PodUnknown:
			failed = append(failed, pod)
		}
	}

	return running, pending, failed, nil
}

func (m *etcdManager) customEtcdSpec() v1alpha1.EtcdClusterSpec {
	spec := m.oc.Spec.Etcd.DeepCopy()
	if len(spec.Repository) == 0 {
		spec.Repository = path.Join("quay.io/coreos/etcd")
		//spec.Repository = path.Join(m.oc.Spec.ImageRepository, constants.EtcdImageName)
	}
	if len(spec.Version) == 0 {
		spec.Version = constants.EtcdImageVersion
	}
	if spec.Size == 0 {
		spec.Size = constants.EtcdDefaultClusterSize
	}
	if m.isSecure() {
		//certSecretName := controller.ClustercertSecretName(m.oc)
		spec.TLS = new(v1alpha1.TLSPolicy)
		spec.TLS.Static = new(v1alpha1.StaticTLS)
		spec.TLS.Static.OperatorSecret = constants.EtcdClientSecret
		spec.TLS.Static.Member = &v1alpha1.MemberSecret{
			PeerSecret:   constants.EtcdPeerSecret,
			ServerSecret: constants.EtcdServerSecret,
		}
	}
	return spec.EtcdClusterSpec
}

func (m *etcdManager) updateEtcdStatus() error {
	// fetch newest onecloud cluster
	oc, err := m.onecloudClusterControl.GetCluster(m.oc.GetNamespace(), m.oc.GetName())
	if err != nil {
		return err
	}
	oc.Status.Etcd = m.status
	newoc, err := m.onecloudClusterControl.UpdateCluster(oc, nil, nil, "etcd-update-status")
	if err != nil {
		return err
	}
	m.oc = newoc
	return nil
}

func (m *etcdManager) fetchCluster() error {
	oc, err := m.onecloudClusterControl.GetCluster(m.oc.GetNamespace(), m.oc.GetName())
	if err != nil {
		return err
	}
	m.oc = oc
	return nil
}

func (m *etcdManager) updateMemberStatus(running []*corev1.Pod) {
	var unready []string
	var ready []string
	for _, pod := range running {
		if k8sutil.IsPodReady(pod) {
			ready = append(ready, pod.Name)
			continue
		}
		unready = append(unready, pod.Name)
	}

	m.status.Members.Ready = ready
	m.status.Members.Unready = unready
}

func (m *etcdManager) reportFailedStatus() {
	log.Infof("cluster failed. Reporting failed reason...")

	retryInterval := 5 * time.Second
	f := func() (bool, error) {
		m.status.Phase = v1alpha1.EtcdClusterPhaseFailed
		err := m.updateEtcdStatus()
		if err == nil || k8sutil.IsKubernetesResourceNotFoundError(err) {
			return true, nil
		}

		if !apierrors.IsConflict(err) {
			log.Warningf("retry report status in %v: fail to update: %v", retryInterval, err)
			return false, nil
		}

		oc, err := m.onecloudClusterControl.GetCluster(m.oc.GetNamespace(), m.oc.GetName())
		if err != nil {
			if k8sutil.IsKubernetesResourceNotFoundError(err) {
				return true, nil
			}
			log.Warningf("retry report status in %v: fail to get latest version: %v", retryInterval, err)
			return false, nil
		}
		m.oc = oc
		return false, nil
	}

	retryutil.Retry(retryInterval, math.MaxInt64, f)
}

func (m *etcdManager) setupServices() error {
	errs := make([]error, 0)
	if err := CreateClientService(m.kubeCli, m.getEtcdClusterPrefix(), m.oc.GetNamespace(), controller.GetOwnerRef(m.oc), m.oc.Spec.Etcd.ClientNodePort); err != nil {
		errs = append(errs, errors.Wrap(err, "create etcd client service"))
	}

	if err := CreatePeerService(m.kubeCli, m.getEtcdClusterPrefix(), m.oc.GetNamespace(), controller.GetOwnerRef(m.oc)); err != nil {
		errs = append(errs, errors.Wrap(err, "create etcd peer service"))
	}
	return errors.NewAggregate(errs)
}

func CreateClientService(kubecli kubernetes.Interface, clusterName, ns string, owner metav1.OwnerReference, nodePort int) error {
	ports := []corev1.ServicePort{{
		Name:       "client",
		Port:       constants.EtcdClientPort,
		TargetPort: intstr.FromInt(constants.EtcdClientPort),
		Protocol:   corev1.ProtocolTCP,
		NodePort:   int32(nodePort),
	}}
	return createService(kubecli, ClientServiceName(clusterName), clusterName, ns, "", ports, owner, false, true)
}

func ClientServiceName(clusterName string) string {
	return clusterName + "-client"
}

func CreatePeerService(kubecli kubernetes.Interface, clusterName, ns string, owner metav1.OwnerReference) error {
	ports := []corev1.ServicePort{{
		Name:       "client",
		Port:       constants.EtcdClientPort,
		TargetPort: intstr.FromInt(constants.EtcdClientPort),
		Protocol:   corev1.ProtocolTCP,
	}, {
		Name:       "peer",
		Port:       2380,
		TargetPort: intstr.FromInt(2380),
		Protocol:   corev1.ProtocolTCP,
	}}

	return createService(kubecli, clusterName, clusterName, ns, corev1.ClusterIPNone, ports, owner, true, false)
}

func createService(
	kubecli kubernetes.Interface, svcName, clusterName, ns, clusterIP string,
	ports []corev1.ServicePort, owner metav1.OwnerReference, tolerateUnreadyEndpoints bool,
	useNodePort bool,
) error {
	svc := newEtcdServiceManifest(svcName, clusterName, clusterIP, ports, tolerateUnreadyEndpoints, useNodePort)
	o := svc.GetObjectMeta()
	o.SetOwnerReferences(append(o.GetOwnerReferences(), owner))

	oldSvc, err := kubecli.CoreV1().Services(ns).Get(context.Background(), svcName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "Get service %q", svcName)
	} else if err == nil {
		if !reflect.DeepEqual(oldSvc.Annotations, svc.Annotations) {
			oldSvc.Annotations = svc.Annotations
			oldSvc.Spec.Ports = svc.Spec.Ports
			oldSvc.Spec.Type = svc.Spec.Type
			_, err = kubecli.CoreV1().Services(ns).Update(context.Background(), oldSvc, metav1.UpdateOptions{})
			return err
		}
	} else if apierrors.IsNotFound(err) {
		if _, err = kubecli.CoreV1().Services(ns).Create(context.Background(), svc, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "create service %q", svcName)
		}
	}
	return nil
}

func newEtcdServiceManifest(
	svcName, clusterName, clusterIP string,
	ports []corev1.ServicePort, tolerateUnreadyEndpoints bool,
	useNodePort bool,
) *corev1.Service {
	labels := k8sutil.LabelsForCluster(clusterName)
	annotations := map[string]string{}
	if tolerateUnreadyEndpoints {
		annotations[k8sutil.TolerateUnreadyEndpointsAnnotation] = "true"
	}
	for i, p := range ports {
		annotations[fmt.Sprintf("onecloud.etcd.io/port%d", i)] = jsonutils.Marshal(p).String()
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        svcName,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			PublishNotReadyAddresses: true,
			Ports:                    ports,
			Selector:                 labels,
			ClusterIP:                clusterIP,
		},
	}
	if useNodePort {
		svc.Spec.Type = corev1.ServiceTypeNodePort
	} else {
		svc.Spec.Type = corev1.ServiceTypeClusterIP
	}
	return svc
}

func (m *etcdManager) run() {
	if err := m.setupServices(); err != nil {
		log.Errorf("fail to setup etcd services: %v", err)
	}
	m.status.ServiceName = controller.NewClusterComponentName(m.oc.GetName(), v1alpha1.EtcdClientComponentType)
	m.status.ClientPort = constants.EtcdClientPort
	m.status.ClientNodePort = m.oc.Spec.Etcd.ClientNodePort
	m.status.Phase = v1alpha1.EtcdClusterPhaseRunning
	if err := m.updateEtcdStatus(); err != nil {
		log.Warningf("update initial etcd culster status failed: %s", err)
	}

	log.Infof("start running ......")
	var rerr error
Loop:
	for {
		select {
		case <-time.After(reconcileInterval):
			if err := m.fetchCluster(); err != nil {
				log.Warningf("fetch cluster failed %s", err)
				continue
			}
			running, pending, failed, err := m.pollPods()
			if err != nil {
				log.Warningf("failed poll pods %s", err)
				continue
			}

			if len(pending) > 0 {
				// Pod startup might take long
				// e.g. pulling image. It would deterministically become running or succeeded/failed later.
				log.Infof("skip reconciliation: running (%v), pending (%v)",
					k8sutil.GetPodNames(running), k8sutil.GetPodNames(pending))
				continue
			}
			if len(failed) > 0 && !controller.EtcdKeepFailedPods {
				// Clean etcd pod in failed status
				for i := 0; i < len(failed); i++ {
					log.Infof("remove failed status pod %s", failed[i].GetName())
					if err := m.removePod(failed[i].Name); err != nil {
						log.Errorf("faield remove pod %s", failed[i].GetName())
					}
				}
			}
			if len(running) == 0 {
				log.Warningf("all etcd pods are dead.")
				// Note: we didn't need the data stone in etcd
				// so we can rebuild etcd cluster on all etcd pods are dead
				m.updateMemberStatus(nil)
				m.status.Phase = v1alpha1.EtcdClusterPhaseFailed
				if err := m.updateEtcdStatus(); err != nil {
					log.Warningf("update etcd status failed: %s", err)
				}
				break Loop
			}
			if rerr != nil || m.members == nil {
				rerr = m.updateMembers(m.podsToMemberSet(running))
				if rerr != nil {
					log.Errorf("failed to update members: %v", rerr)
					break
				}
			}
			rerr = m.reconcile(running)
			if errors.Cause(rerr) == ErrLostQuorum {
				// etcd cluster lost quorum, try clean all of members
				log.Errorf("etcd cluster lost quorum, clean all of members")
				if err := m.cleanAllMembers(); err != nil {
					log.Errorf("clean all members failed %s", err)
					continue
				} else {
					go m.sync(m.oc)
					break Loop
				}
			} else if rerr != nil {
				log.Errorf("failed to reconcile %s", rerr)
				break
			}
			m.updateMemberStatus(running)
			if err := m.updateEtcdStatus(); err != nil {
				log.Warningf("periodic update etcd status failed %s", err)
			}

			if isFatalError(rerr) {
				m.status.Reason = rerr.Error()
				log.Errorf("cluster failed: %s", rerr)
				m.reportFailedStatus()
				break Loop
			}
			// TODO handle cluster resize, udpate image version event
		}
	}
}

// reconcile reconciles cluster current state to desired state specified by spec.
// - it tries to reconcile the cluster to desired size.
// - if the cluster needs for upgrade, it tries to upgrade old member one by one.
func (m *etcdManager) reconcile(pods []*corev1.Pod) error {
	log.Infoln("Start reconciling")
	defer log.Infoln("Finish reconciling")

	defer func() {
		m.status.Size = m.members.Size()
	}()

	sp := m.oc.Spec.Etcd
	running := m.podsToMemberSet(pods)
	if !running.IsEqual(m.members) || m.members.Size() != sp.Size {
		return m.reconcileMembers(running)
	}

	if needUpgrade(pods, sp.EtcdClusterSpec) {
		m.status.TargetVersion = sp.Version
		mb := pickOneOldMember(pods, sp.Version)
		return m.upgradeOneMember(mb.Name)
	}

	m.status.TargetVersion = ""
	m.status.CurrentVersion = sp.Version

	return nil
}

// reconcileMembers reconciles
// - running pods on k8s and cluster membership
// - cluster membership and expected size of etcd cluster
// Steps:
// 1. Remove all pods from running set that does not belong to member set.
// 2. L consist of remaining pods of runnings
// 3. If L = members, the current state matches the membership state. END.
// 4. If len(L) < len(members)/2 + 1, return quorum lost error.
// 5. Add one missing member. END.
func (m *etcdManager) reconcileMembers(running etcdutil.MemberSet) error {
	log.Infof("running members: %s", running)
	log.Infof("cluster membership: %s", m.members)

	unknownMembers := running.Diff(m.members)
	if unknownMembers.Size() > 0 {
		log.Infof("removing unexpected pods: %v", unknownMembers)
		for _, mb := range unknownMembers {
			if err := m.removePod(mb.Name); err != nil {
				return err
			}
		}
	}
	L := running.Diff(unknownMembers)

	if L.Size() == m.members.Size() {
		return m.resize()
	}

	if L.Size() < m.members.Size()/2+1 {
		return ErrLostQuorum
	}

	log.Infof("removing one dead member")
	// remove dead members that doesn't have any running pods before doing resizing.
	return m.removeDeadMember(m.members.Diff(L).PickOne())
}

func (m *etcdManager) resize() error {
	if m.members.Size() == m.oc.Spec.Etcd.Size {
		return nil
	}

	if m.members.Size() < m.oc.Spec.Etcd.Size {
		return m.addOneMember()
	}

	return m.removeOneMember()
}

func (m *etcdManager) addOneMember() error {
	cfg := clientv3.Config{
		Endpoints:   m.members.ClientURLs(),
		DialTimeout: constants.EtcdDefaultDialTimeout,
		TLS:         m.tlsConfig,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return fmt.Errorf("add one member failed: creating etcd client failed %v", err)
	}
	defer etcdcli.Close()

	newMember := m.newMember()
	ctx, cancel := context.WithTimeout(context.Background(), constants.EtcdDefaultRequestTimeout)
	resp, err := etcdcli.MemberAdd(ctx, []string{newMember.PeerURL()})
	cancel()
	if err != nil {
		return fmt.Errorf("fail to add new member (%s): %v", newMember.Name, err)
	}
	newMember.ID = resp.Member.ID
	m.members.Add(newMember)

	if err := m.createPod(m.members, newMember, "existing"); err != nil {
		return fmt.Errorf("fail to create member's pod (%s): %v", newMember.Name, err)
	}
	log.Infof("added member (%s)", newMember.Name)
	return nil
}

func (m *etcdManager) removeOneMember() error {
	return m.removeMember(m.members.PickOne())
}

func (m *etcdManager) newMember() *etcdutil.Member {
	name := k8sutil.UniqueMemberName(m.getEtcdClusterPrefix())
	mb := &etcdutil.Member{
		Name:         name,
		Namespace:    m.oc.GetNamespace(),
		SecurePeer:   m.isSecure(),
		SecureClient: m.isSecure(),
		IPv6Cluster:  m.oc.Spec.IPv6Cluster,
	}

	//if m.oc.Spec.Etcd.Pod != nil {
	//	mb.ClusterDomain = m.oc.Spec.Etcd.Pod.ClusterDomain
	//}
	return mb
}

func (m *etcdManager) removeDeadMember(toRemove *etcdutil.Member) error {
	log.Infof("removing dead member %q", toRemove.Name)
	return m.removeMember(toRemove)
}

func (m *etcdManager) removeMember(toRemove *etcdutil.Member) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("remove member (%s) failed: %v", toRemove.Name, err)
		}
	}()

	err = etcdutil.RemoveMember(m.members.ClientURLs(), m.tlsConfig, toRemove.ID)
	if err != nil {
		switch err {
		case rpctypes.ErrMemberNotFound:
			log.Infof("etcd member (%v) has been removed", toRemove.Name)
		default:
			return err
		}
	}
	m.members.Remove(toRemove.Name)
	if err := m.removePod(toRemove.Name); err != nil {
		return err
	}
	if m.isPodPVEnabled() {
		err = m.removePVC(k8sutil.PVCNameFromMember(toRemove.Name))
		if err != nil {
			return err
		}
	}
	log.Infof("removed member (%v) with ID (%d)", toRemove.Name, toRemove.ID)
	return nil
}

func (m *etcdManager) cleanAllMembers() error {
	for _, member := range m.members {
		if err := m.removePod(member.Name); err != nil {
			return err
		}
		m.members.Remove(member.Name)
		if m.isPodPVEnabled() {
			err := m.removePVC(k8sutil.PVCNameFromMember(member.Name))
			if err != nil {
				return err
			}
		}

	}
	m.status.Phase = v1alpha1.EtcdClusterPhaseFailed
	m.status.Size = 0
	m.status.Members = v1alpha1.EtcdMembersStatus{}
	return m.updateEtcdStatus()
}

func (m *etcdManager) removePVC(pvcName string) error {
	err := m.kubeCli.CoreV1().PersistentVolumeClaims(m.oc.GetNamespace()).Delete(context.Background(), pvcName, metav1.DeleteOptions{})
	if err != nil && !k8sutil.IsKubernetesResourceNotFoundError(err) {
		return fmt.Errorf("remove pvc (%s) failed: %v", pvcName, err)
	}
	return nil
}

func (m *etcdManager) updateMembers(known etcdutil.MemberSet) error {
	resp, err := etcdutil.ListMembers(known.ClientURLs(), m.tlsConfig)
	if err != nil {
		return err
	}
	members := etcdutil.MemberSet{}
	for _, mb := range resp.Members {
		name, err := getMemberName(mb, m.oc.GetName())
		if err != nil {
			return errors.Wrap(err, "get member name failed")
		}

		members[name] = &etcdutil.Member{
			Name:         name,
			Namespace:    m.oc.GetNamespace(),
			ID:           mb.ID,
			SecurePeer:   m.isSecure(),
			SecureClient: m.isSecure(),
			IPv6Cluster:  m.oc.Spec.IPv6Cluster,
		}
	}
	m.members = members
	return nil
}

func (m *etcdManager) upgradeOneMember(memberName string) error {
	ns := m.oc.GetNamespace()
	pod, err := m.kubeCli.CoreV1().Pods(ns).Get(context.Background(), memberName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("fail to get pod (%s): %v", memberName, err)
	}
	oldpod := pod.DeepCopy()

	log.Infof("upgrading the etcd member %v from %s to %s", memberName, k8sutil.GetEtcdVersion(pod), m.oc.Spec.Etcd.Version)
	pod.Spec.Containers[0].Image = k8sutil.ImageName(m.oc.Spec.Etcd.Repository, m.oc.Spec.Etcd.Version)
	k8sutil.SetEtcdVersion(pod, m.oc.Spec.Etcd.Version)

	patchdata, err := k8sutil.CreatePatch(oldpod, pod, corev1.Pod{})
	if err != nil {
		return fmt.Errorf("error creating patch: %v", err)
	}

	_, err = m.kubeCli.CoreV1().Pods(ns).Patch(context.Background(), pod.GetName(), types.StrategicMergePatchType, patchdata, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("fail to update the etcd member (%s): %v", memberName, err)
	}
	log.Infof("finished upgrading the etcd member %v", memberName)
	return nil
}
func needUpgrade(pods []*corev1.Pod, cs v1alpha1.EtcdClusterSpec) bool {
	return len(pods) == cs.Size && pickOneOldMember(pods, cs.Version) != nil
}

func pickOneOldMember(pods []*corev1.Pod, newVersion string) *etcdutil.Member {
	for _, pod := range pods {
		if k8sutil.GetEtcdVersion(pod) == newVersion {
			continue
		}
		return &etcdutil.Member{Name: pod.Name, Namespace: pod.Namespace}
	}
	return nil
}

func (m *etcdManager) podsToMemberSet(pods []*corev1.Pod) etcdutil.MemberSet {
	members := etcdutil.MemberSet{}
	for _, pod := range pods {
		member := &etcdutil.Member{
			Name:         pod.Name,
			Namespace:    pod.Namespace,
			SecureClient: m.isSecure(),
			IPv6Cluster:  m.oc.Spec.IPv6Cluster,
		}
		members.Add(member)
	}
	return members
}

type fatalError struct {
	reason string
}

func (fe *fatalError) Error() string {
	return fe.reason
}

func newFatalError(reason string) *fatalError {
	return &fatalError{reason}
}

func isFatalError(err error) bool {
	switch errors.Cause(err).(type) {
	case *fatalError:
		return true
	default:
		return false
	}
}

func getMemberName(m *etcdserverpb.Member, clusterName string) (string, error) {
	name, err := etcdutil.MemberNameFromPeerURL(m.PeerURLs[0])
	if err != nil {
		return "", newFatalError(fmt.Sprintf("invalid member peerURL (%s): %v", m.PeerURLs[0], err))
	}
	return name, nil
}

// addEtcdVolumeToPod abstract the process of appending volume spec to pod spec
func addEtcdVolumeToPod(pod *corev1.Pod, pvc *corev1.PersistentVolumeClaim) {
	vol := corev1.Volume{Name: etcdVolumeName}
	if pvc != nil {
		vol.VolumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.Name},
		}
	} else {
		// default mount etcd data dir as tmpfs
		vol.VolumeSource = corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, vol)
}
