// Copyright 2014 Google Inc. All Rights Reserved.
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

package integration

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	kclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/v1"
	_ "k8s.io/client-go/pkg/apis/rbac/install"
	rbacv1beta1 "k8s.io/client-go/pkg/apis/rbac/v1beta1"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type kubeFramework interface {
	// Kube client
	Client() *kclient.Clientset

	// Parses and Returns a replication Controller object contained in 'filePath'
	ParseRC(filePath string) (*v1.ReplicationController, error)

	// Parses and Returns a service object contained in 'filePath'
	ParseService(filePath string) (*v1.Service, error)

	// Parses and Returns a RBAC object contained in 'filePath'
	ParseRBAC(filePath string) (*rbacv1beta1.ClusterRoleBinding, error)

	// Parses and Returns a ServiceAccount object contained in 'filePath'
	ParseServiceAccount(filePath string) (*v1.ServiceAccount, error)

	// Creates a kube service.
	CreateService(ns string, service *v1.Service) (*v1.Service, error)

	// Creates a namespace.
	CreateNs(ns *v1.Namespace) (*v1.Namespace, error)

	// Creates a RBAC.
	CreateRBAC(crb *rbacv1beta1.ClusterRoleBinding) error

	// Creates a ServiceAccount.
	CreateServiceAccount(sa *v1.ServiceAccount) error

	// Creates a kube replication controller.
	CreateRC(ns string, rc *v1.ReplicationController) (*v1.ReplicationController, error)

	// Deletes a namespace
	DeleteNs(ns string) error

	// Destroy cluster
	DestroyCluster()

	// Returns a url that provides access to a kubernetes service via the proxy on the apiserver.
	// This url requires master auth.
	GetProxyUrlForService(service *v1.Service) string

	// Returns the node hostnames.
	GetNodeNames() ([]string, error)

	// Returns the nodes.
	GetNodes() (*v1.NodeList, error)

	// Returns pod names in the cluster.
	// TODO: Remove, or mix with namespace
	GetRunningPodNames() ([]string, error)

	// Returns pods in the cluster running outside kubernetes-master.
	GetPodsRunningOnNodes() ([]v1.Pod, error)

	// Returns pods in the cluster.
	GetAllRunningPods() ([]v1.Pod, error)

	WaitUntilPodRunning(ns string, podLabels map[string]string, timeout time.Duration) error
	WaitUntilServiceActive(svc *v1.Service, timeout time.Duration) error
}

type realKubeFramework struct {
	// Kube client.
	kubeClient *kclient.Clientset

	// The version of the kube cluster
	version string

	// Master IP for this framework
	masterIP string

	// The base directory of current kubernetes release.
	baseDir string
}

const imageUrlTemplate = "https://github.com/kubernetes/kubernetes/releases/download/v%s/kubernetes.tar.gz"

var (
	kubeConfig = flag.String("kube_config", os.Getenv("HOME")+"/.kube/config", "Path to cluster info file.")
	workDir    = flag.String("work_dir", "/tmp/heapster_test", "Filesystem path where test files will be stored. Files will persist across runs to speed up tests.")
)

func exists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		glog.V(2).Infof("%q does not exist", path)
		return false
	}
	return true
}

const pathToGCEConfig = "cluster/gce/config-default.sh"

func disableClusterMonitoring(kubeBaseDir string) error {
	kubeConfigFilePath := filepath.Join(kubeBaseDir, pathToGCEConfig)
	input, err := ioutil.ReadFile(kubeConfigFilePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(input), "\n")

	for i, line := range lines {
		if strings.Contains(line, "ENABLE_CLUSTER_MONITORING") {
			lines[i] = "ENABLE_CLUSTER_MONITORING=false"
		} else if strings.Contains(line, "NUM_MINIONS=") {
			lines[i] = "NUM_MINIONS=2"
		} else if strings.Contains(line, "MASTER_SIZE=") {
			// TODO(piosz): remove this once everything fits onto master
			lines[i] = "MASTER_SIZE=n1-standard-2"
		}
	}
	output := strings.Join(lines, "\n")
	return ioutil.WriteFile(kubeConfigFilePath, []byte(output), 0644)
}

func runKubeClusterCommand(kubeBaseDir, command string) ([]byte, error) {
	cmd := exec.Command(filepath.Join(kubeBaseDir, "cluster", command))
	glog.V(2).Infof("about to run %v", cmd)
	return cmd.CombinedOutput()
}

func setupNewCluster(kubeBaseDir string) error {
	cmd := "kube-up.sh"
	destroyCluster(kubeBaseDir)
	out, err := runKubeClusterCommand(kubeBaseDir, cmd)
	if err != nil {
		glog.Errorf("failed to bring up cluster - %q\n%s", err, out)
		return fmt.Errorf("failed to bring up cluster - %q", err)
	}
	glog.V(2).Info(string(out))
	glog.V(2).Infof("Giving the cluster 30 sec to stabilize")
	time.Sleep(30 * time.Second)
	return nil
}

func destroyCluster(kubeBaseDir string) error {
	if kubeBaseDir == "" {
		glog.Infof("Skipping cluster tear down since kubernetes repo base path is not set.")
		return nil
	}
	glog.V(1).Info("Bringing down any existing kube cluster")
	out, err := runKubeClusterCommand(kubeBaseDir, "kube-down.sh")
	if err != nil {
		glog.Errorf("failed to tear down cluster - %q\n%s", err, out)
		return fmt.Errorf("failed to tear down kube cluster - %q", err)
	}

	return nil
}

func downloadRelease(workDir, version string) error {
	// Temporary download path.
	downloadPath := filepath.Join(workDir, "kube")
	// Format url.
	downloadUrl := fmt.Sprintf(imageUrlTemplate, version)
	glog.V(1).Infof("About to download kube release using url: %q", downloadUrl)

	// Download kube code and store it in a temp dir.
	if err := exec.Command("wget", downloadUrl, "-O", downloadPath).Run(); err != nil {
		return fmt.Errorf("failed to wget kubernetes release @ %q - %v", downloadUrl, err)
	}

	// Un-tar kube release.
	if err := exec.Command("tar", "-xf", downloadPath, "-C", workDir).Run(); err != nil {
		return fmt.Errorf("failed to un-tar kubernetes release at %q - %v", downloadPath, err)
	}
	return nil
}

func getKubeClient() (string, *kclient.Clientset, error) {
	c, err := kclientcmd.LoadFromFile(*kubeConfig)
	if err != nil {
		return "", nil, fmt.Errorf("error loading kubeConfig: %v", err.Error())
	}
	if c.CurrentContext == "" || len(c.Clusters) == 0 {
		return "", nil, fmt.Errorf("invalid kubeConfig: %+v", *c)
	}
	config, err := kclientcmd.NewDefaultClientConfig(
		*c,
		&kclientcmd.ConfigOverrides{
			ClusterInfo: kclientcmdapi.Cluster{
				APIVersion: "v1",
			},
		}).ClientConfig()
	if err != nil {
		return "", nil, fmt.Errorf("error parsing kubeConfig: %v", err.Error())
	}
	kubeClient, err := kclient.NewForConfig(config)
	if err != nil {
		return "", nil, fmt.Errorf("error creating client - %q", err)
	}

	return c.Clusters[c.CurrentContext].Server, kubeClient, nil
}

func validateCluster(baseDir string) bool {
	glog.V(1).Info("validating existing cluster")
	out, err := runKubeClusterCommand(baseDir, "validate-cluster.sh")
	if err != nil {
		glog.V(1).Infof("cluster validation failed - %q\n %s", err, out)
		return false
	}
	return true
}

func requireNewCluster(baseDir, version string) bool {
	// Setup kube client
	_, kubeClient, err := getKubeClient()
	if err != nil {
		glog.V(1).Infof("kube client creation failed - %q", err)
		return true
	}
	glog.V(1).Infof("checking if existing cluster can be used")
	versionInfo, err := kubeClient.ServerVersion()
	if err != nil {
		glog.V(1).Infof("failed to get kube version info - %q", err)
		return true
	}
	return !strings.Contains(versionInfo.GitVersion, version)
}

func requireDownload(baseDir string) bool {
	// Check that cluster scripts are present.
	return !exists(filepath.Join(baseDir, "cluster", "kube-up.sh")) ||
		!exists(filepath.Join(baseDir, "cluster", "kube-down.sh")) ||
		!exists(filepath.Join(baseDir, "cluster", "validate-cluster.sh"))
}

func downloadAndSetupCluster(version string) (baseDir string, err error) {
	// Create a temp dir to store the kube release files.
	tempDir := filepath.Join(*workDir, version)
	if !exists(tempDir) {
		if err := os.MkdirAll(tempDir, 0700); err != nil {
			return "", fmt.Errorf("failed to create a temp dir at %s - %q", tempDir, err)
		}
		glog.V(1).Infof("Successfully setup work dir at %s", tempDir)
	}

	kubeBaseDir := filepath.Join(tempDir, "kubernetes")

	if requireDownload(kubeBaseDir) {
		if exists(kubeBaseDir) {
			os.RemoveAll(kubeBaseDir)
		}
		if err := downloadRelease(tempDir, version); err != nil {
			return "", err
		}
		glog.V(1).Infof("Successfully downloaded kubernetes release at %s", tempDir)
	}

	// Disable monitoring
	if err := disableClusterMonitoring(kubeBaseDir); err != nil {
		return "", fmt.Errorf("failed to disable cluster monitoring in kube cluster config - %q", err)
	}
	glog.V(1).Info("Disabled cluster monitoring")
	if !requireNewCluster(kubeBaseDir, version) {
		glog.V(1).Infof("skipping cluster setup since a cluster with required version already exists")
		return kubeBaseDir, nil
	}

	// Setup kube cluster
	glog.V(1).Infof("Setting up new kubernetes cluster version: %s", version)
	if err := os.Setenv("KUBERNETES_SKIP_CONFIRM", "y"); err != nil {
		return "", err
	}
	if err := setupNewCluster(kubeBaseDir); err != nil {
		// Cluster setup failed for some reason.
		// Attempting to validate the cluster to see if it failed in the validate phase.
		sleepDuration := 10 * time.Second
		clusterReady := false
		for i := 0; i < int(time.Minute/sleepDuration); i++ {
			if !validateCluster(kubeBaseDir) {
				glog.Infof("Retry validation after %v seconds.", sleepDuration/time.Second)
				time.Sleep(sleepDuration)
			} else {
				clusterReady = true
				break
			}
		}
		if !clusterReady {
			return "", fmt.Errorf("failed to setup cluster - %q", err)
		}
	}
	glog.V(1).Infof("Successfully setup new kubernetes cluster version %s", version)

	return kubeBaseDir, nil
}

func newKubeFramework(version string) (kubeFramework, error) {
	// Install gcloud components.
	// TODO(piosz): move this to the image creation
	cmd := exec.Command("gcloud", "components", "install", "alpha", "beta", "kubectl", "--quiet")
	glog.V(2).Infof("about to install gcloud components")
	if o, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("Error while installing gcloud components: %v\n%s", err, o)
	}

	var err error
	kubeBaseDir := ""
	if version != "" {
		if len(strings.Split(version, ".")) != 3 {
			glog.Warningf("Using not stable version - %q", version)
		}
		kubeBaseDir, err = downloadAndSetupCluster(version)
		if err != nil {
			return nil, err
		}
	}

	// Setup kube client
	masterIP, kubeClient, err := getKubeClient()
	if err != nil {
		return nil, err
	}
	return &realKubeFramework{
		kubeClient: kubeClient,
		baseDir:    kubeBaseDir,
		version:    version,
		masterIP:   masterIP,
	}, nil
}

func (self *realKubeFramework) Client() *kclient.Clientset {
	return self.kubeClient
}

func (self *realKubeFramework) loadObject(filePath string) (runtime.Object, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %v", err)
	}
	obj, _, err := api.Codecs.UniversalDecoder(v1.SchemeGroupVersion).Decode(data, nil, nil)
	return obj, err
}

func (self *realKubeFramework) ParseRC(filePath string) (*v1.ReplicationController, error) {
	obj, err := self.loadObject(filePath)
	if err != nil {
		return nil, err
	}

	rc, ok := obj.(*v1.ReplicationController)
	if !ok {
		return nil, fmt.Errorf("Failed to cast replicationController: %#v", obj)
	}
	return rc, nil
}

// Parses and Returns a RBAC object contained in 'filePath'
func (self *realKubeFramework) ParseRBAC(filePath string) (*rbacv1beta1.ClusterRoleBinding, error) {
	obj, err := self.loadRBACObject(filePath)
	if err != nil {
		return nil, err
	}
	rbac, ok := obj.(*rbacv1beta1.ClusterRoleBinding)
	if !ok {
		return nil, fmt.Errorf("Failed to cast clusterrolebinding: %v", obj)
	}
	return rbac, nil
}

func (self *realKubeFramework) loadRBACObject(filePath string) (runtime.Object, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %v", err)
	}
	obj, _, err := api.Codecs.UniversalDecoder(rbacv1beta1.SchemeGroupVersion).Decode(data, nil, nil)
	return obj, err
}

// CreateRBAC creates the RBAC object
func (self *realKubeFramework) CreateRBAC(rbac *rbacv1beta1.ClusterRoleBinding) error {
	_, err := self.kubeClient.RbacV1beta1().ClusterRoleBindings().Create(rbac)
	return err
}

// Parses and Returns a ServiceAccount object contained in 'filePath'
func (self *realKubeFramework) ParseServiceAccount(filePath string) (*v1.ServiceAccount, error) {
	obj, err := self.loadObject(filePath)
	if err != nil {
		return nil, err
	}

	sa, ok := obj.(*v1.ServiceAccount)
	if !ok {
		return nil, fmt.Errorf("Failed to cast serviceaccount: %v", obj)
	}
	return sa, nil
}

// CreateServiceAccount creates the ServiceAccount object
func (self *realKubeFramework) CreateServiceAccount(sa *v1.ServiceAccount) error {
	_, err := self.kubeClient.CoreV1().ServiceAccounts(sa.Namespace).Create(sa)
	return err
}

func (self *realKubeFramework) ParseService(filePath string) (*v1.Service, error) {
	obj, err := self.loadObject(filePath)
	if err != nil {
		return nil, err
	}
	service, ok := obj.(*v1.Service)
	if !ok {
		return nil, fmt.Errorf("Failed to cast service: %v", obj)
	}
	return service, nil
}

func (self *realKubeFramework) CreateService(ns string, service *v1.Service) (*v1.Service, error) {
	service.Namespace = ns
	newSvc, err := self.kubeClient.Services(ns).Create(service)
	return newSvc, err
}

func (self *realKubeFramework) DeleteNs(ns string) error {

	_, err := self.kubeClient.Namespaces().Get(ns, metav1.GetOptions{})
	if err != nil {
		glog.V(0).Infof("Cannot get namespace %q. Skipping deletion: %s", ns, err)
		return nil
	}
	glog.V(0).Infof("Deleting namespace %s", ns)
	self.kubeClient.Namespaces().Delete(ns, nil)

	for i := 0; i < 5; i++ {
		glog.V(0).Infof("Checking for namespace %s", ns)
		_, err := self.kubeClient.Namespaces().Get(ns, metav1.GetOptions{})
		if err != nil {
			glog.V(0).Infof("%s doesn't exist", ns)
			return nil
		}
		time.Sleep(10 * time.Second)
	}
	return fmt.Errorf("Namespace %s still exists", ns)
}

func (self *realKubeFramework) CreateNs(ns *v1.Namespace) (*v1.Namespace, error) {
	return self.kubeClient.Namespaces().Create(ns)
}

func (self *realKubeFramework) CreateRC(ns string, rc *v1.ReplicationController) (*v1.ReplicationController, error) {
	rc.Namespace = ns
	return self.kubeClient.ReplicationControllers(ns).Create(rc)
}

func (self *realKubeFramework) DestroyCluster() {
	destroyCluster(self.baseDir)
}

func (self *realKubeFramework) GetProxyUrlForService(service *v1.Service) string {
	return fmt.Sprintf("%s/api/v1/proxy/namespaces/default/services/%s/", self.masterIP, service.Name)
}

func (self *realKubeFramework) GetNodeNames() ([]string, error) {
	var nodes []string
	nodeList, err := self.GetNodes()
	if err != nil {
		return nodes, err
	}
	for _, node := range nodeList.Items {
		nodes = append(nodes, node.Name)
	}
	return nodes, nil
}

func (self *realKubeFramework) GetNodes() (*v1.NodeList, error) {
	return self.kubeClient.Nodes().List(metav1.ListOptions{})
}

func (self *realKubeFramework) GetAllRunningPods() ([]v1.Pod, error) {
	return getRunningPods(true, self.kubeClient)
}

func (self *realKubeFramework) GetPodsRunningOnNodes() ([]v1.Pod, error) {
	return getRunningPods(false, self.kubeClient)
}

func getRunningPods(includeMaster bool, kubeClient *kclient.Clientset) ([]v1.Pod, error) {
	glog.V(0).Infof("Getting running pods")
	podList, err := kubeClient.Pods(v1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods := []v1.Pod{}
	for _, pod := range podList.Items {
		if pod.Status.Phase == v1.PodRunning {
			if includeMaster || !isMasterNode(pod.Spec.NodeName) {
				pods = append(pods, pod)
			}
		}
	}
	return pods, nil
}

func isMasterNode(nodeName string) bool {
	return strings.Contains(nodeName, "kubernetes-master")
}

func (self *realKubeFramework) GetRunningPodNames() ([]string, error) {
	var pods []string
	podList, err := self.GetAllRunningPods()
	if err != nil {
		return pods, err
	}
	for _, pod := range podList {
		pods = append(pods, string(pod.Name))
	}
	return pods, nil
}

func (rkf *realKubeFramework) WaitUntilPodRunning(ns string, podLabels map[string]string, timeout time.Duration) error {
	glog.V(2).Infof("Waiting for pod %v in %s...", podLabels, ns)
	podsInterface := rkf.Client().Pods(ns)
	for i := 0; i < int(timeout/time.Second); i++ {
		podList, err := podsInterface.List(metav1.ListOptions{
			LabelSelector: labels.Set(podLabels).AsSelector().String(),
		})
		if err != nil {
			glog.V(1).Info(err)
			return err
		}
		if len(podList.Items) > 0 {
			podSpec := podList.Items[0]
			if podSpec.Status.Phase == v1.PodRunning {
				return nil
			}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("pod not in running state after %d", timeout/time.Second)
}

func (rkf *realKubeFramework) WaitUntilServiceActive(svc *v1.Service, timeout time.Duration) error {
	glog.V(2).Infof("Waiting for endpoints in service %s/%s", svc.Namespace, svc.Name)
	for i := 0; i < int(timeout/time.Second); i++ {
		e, err := rkf.Client().Endpoints(svc.Namespace).Get(svc.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if len(e.Subsets) > 0 {
			return nil
		}
		time.Sleep(time.Second)
	}

	return fmt.Errorf("Service %q not active after %d seconds - no endpoints found", svc.Name, timeout/time.Second)

}
