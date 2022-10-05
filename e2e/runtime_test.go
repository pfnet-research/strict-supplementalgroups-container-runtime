package e2e

import (
	"bufio"
	"bytes"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/pointer"
)

var _ = Describe("strict-supplementalgroups-container-runtime", func() {
	Context("CRI: containerd", func() {
		assertBehaviorOnCRI("containerd")
	})
	Context("CRI: cri-o", func() {
		assertBehaviorOnCRI("cri-o")
	})
})

var assertBehaviorOnCRI = func(criName string) {
	containers := []corev1.Container{{
		Name:            "ctr",
		Image:           testImage,
		ImagePullPolicy: corev1.PullNever,
		Command:         []string{"sh", "-c", `id && sleep 65535`},
	}}

	mustGetLogLastLine := func(name string) string {
		req := kubeClient.CoreV1().Pods(namespace).GetLogs(name, &corev1.PodLogOptions{
			Container: "ctr",
			TailLines: pointer.Int64(1),
		})
		podLogs, err := req.Stream(ctx)
		Expect(err).NotTo(HaveOccurred())
		defer podLogs.Close()
		scanner := bufio.NewScanner(podLogs)
		lastLine := ""
		for scanner.Scan() {
			lastLine = scanner.Text()
		}
		return lastLine
	}

	mustExecIdLastLine := func(name string) string {
		req := kubeClient.CoreV1().RESTClient().Post().Resource("pods").Name(name).Namespace(namespace).SubResource("exec")
		req.VersionedParams(&corev1.PodExecOptions{
			Command: []string{"id"},
			Stdout:  true,
		}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
		Expect(err).NotTo(HaveOccurred())

		var stdout bytes.Buffer
		err = exec.Stream(remotecommand.StreamOptions{
			Stdout: &stdout,
		})
		Expect(err).NotTo(HaveOccurred())

		scanner := bufio.NewScanner(&stdout)
		lastLine := ""
		for scanner.Scan() {
			lastLine = scanner.Text()
		}
		return lastLine
	}

	When("Pod.Spec.SecurityContext.{SupplementalGroups ∪ FSGroup} ⊇ 'Groups defined in the image'", func() {
		It("should satisfy the current user belongs to groups specified in the image (i.e.bypass-supplemental-group)", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("user-belongs-to-bypassed-group-%s", criName),
				},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"cri": criName,
					},
					RuntimeClassName: &runtimeClassName,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:          &uidBelongToAdditionalGroup,
						RunAsGroup:         &uidBelongToAdditionalGroup,
						SupplementalGroups: []int64{additionalGidInImage},
						FSGroup:            pointer.Int64(additionalGidInImage),
					},
					Containers: containers,
				},
			}
			var err error
			_, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				got, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(got.Status.Phase).To(Equal(corev1.PodRunning))
			}, 1*time.Minute).Should(Succeed())

			expectedIdOutput := fmt.Sprintf(
				`uid=%d(alice) gid=%d(alice) groups=%d(alice),%d(bypassed-group)`,
				uidBelongToAdditionalGroup, uidBelongToAdditionalGroup, uidBelongToAdditionalGroup, additionalGidInImage,
			)

			// assert uid/gid/supplementalgroups in container process
			Expect(mustGetLogLastLine(pod.Name)).To(Equal(expectedIdOutput))

			// assert exec 'id' command output
			Expect(mustExecIdLastLine(pod.Name)).To(Equal(expectedIdOutput))
		})
	})

	When("!(Pod.Spec.SecurityContext..{SupplementalGroups ∪ FSGroup} ⊇ 'Groups defined in the image')", func() {
		It("should satisfy the current user does NOT belong to groups specified in the image (i.e.bypassed-group) But belong to SupplementalGroups defined in pod manifest", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("user-can-not-belongs-to-bypassed-group-%s", criName),
				},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"cri": criName,
					},
					RuntimeClassName: &runtimeClassName,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser:          &uidBelongToAdditionalGroup,
						RunAsGroup:         &uidBelongToAdditionalGroup,
						SupplementalGroups: []int64{additionalGidInImage + 1},
						FSGroup:            pointer.Int64(additionalGidInImage + 2),
					},
					Containers: containers,
				},
			}

			var err error
			_, err = kubeClient.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				got, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(got.Status.Phase).To(Equal(corev1.PodRunning))
			}, 1*time.Minute).Should(Succeed())

			// alice does NOT belog to "bypassed-group" group
			// but just belong to additionalGidInImage+1
			expectedIdOutput := fmt.Sprintf(
				`uid=%d(alice) gid=%d(alice) groups=%d(alice),%d,%d`,
				uidBelongToAdditionalGroup, uidBelongToAdditionalGroup, uidBelongToAdditionalGroup, additionalGidInImage+1, additionalGidInImage+2,
			)

			// assert uid/gid/supplementalgroups in container process
			Expect(mustGetLogLastLine(pod.Name)).To(Equal(expectedIdOutput))

			// assert exec 'id' command output
			Expect(mustExecIdLastLine(pod.Name)).To(Equal(expectedIdOutput))
		})
	})
}
