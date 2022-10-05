package e2e

import (
	"context"
	"flag"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	namespace = "default"
)

var (
	ctx        = context.Background()
	kubeClient *kubernetes.Clientset
	restConfig *rest.Config

	// test cli flags
	kubeConfig                 string
	testImage                  string
	additionalGidInImage       = int64(50000)
	uidBelongToAdditionalGroup = int64(1000)
	runtimeClassName           = "strict-supplementalgroups"
)

func TestE2e(t *testing.T) {
	flag.Parse()
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2e Suite")
}

var _ = BeforeSuite(func() {
	Expect(kubeConfig).NotTo(BeEmpty())
	Expect(testImage).NotTo(BeEmpty())

	var err error
	restConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	Expect(err).NotTo(HaveOccurred())
	kubeClient, err = kubernetes.NewForConfig(restConfig)
	Expect(err).NotTo(HaveOccurred())

	// wait for all node is ready and service account created to create test pods
	Eventually(func(g Gomega) {
		nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		for _, node := range nodes.Items {
			g.Expect(node.Status.Conditions).To(ContainElement(SatisfyAll(
				WithTransform(func(c corev1.NodeCondition) corev1.NodeConditionType {
					return c.Type
				}, Equal(corev1.NodeReady)),
				WithTransform(func(c corev1.NodeCondition) corev1.ConditionStatus {
					return c.Status
				}, Equal(corev1.ConditionTrue)),
			)))
		}
	})
})

func init() {
	flag.StringVar(&kubeConfig, "kubeconfig", "", "kubeconfig to use")
	flag.StringVar(&testImage, "test-image", "", "test image")
}
