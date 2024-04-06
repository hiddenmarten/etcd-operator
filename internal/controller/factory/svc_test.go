/*
Copyright 2024 The etcd-operator Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package factory

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	etcdaenixiov1alpha1 "github.com/aenix-io/etcd-operator/api/v1alpha1"
)

var _ = Describe("CreateOrUpdateService handlers", func() {
	Context("When ensuring a cluster services", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		etcdcluster := &etcdaenixiov1alpha1.EtcdCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: "default",
				UID:       "test-uid",
			},
			Spec: etcdaenixiov1alpha1.EtcdClusterSpec{
				Replicas: ptr.To(int32(3)),
			},
		}

		It("should successfully create the cluster service", func() {
			svc := &corev1.Service{}
			err := CreateOrUpdateClusterService(ctx, etcdcluster, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, typeNamespacedName, svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			Expect(svc.Spec.ClusterIP).To(Equal("None"))

			Expect(k8sClient.Delete(ctx, svc)).To(Succeed())
		})

		It("should successfully create the cluster service with custom metadata", func() {
			svc := &corev1.Service{}
			cluster := etcdcluster.DeepCopy()
			cluster.Spec.HeadlessServiceTemplate = &etcdaenixiov1alpha1.HeadlessServiceSpec{
				EmbeddedObjectMetadata: etcdaenixiov1alpha1.EmbeddedObjectMetadata{
					Name:        "headless-name",
					Labels:      map[string]string{"label": "value"},
					Annotations: map[string]string{"annotation": "value"},
				},
			}

			err := CreateOrUpdateClusterService(ctx, cluster, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Spec.HeadlessServiceTemplate.Name,
				Namespace: "default",
			}, svc)
			Expect(err).NotTo(HaveOccurred())

			By("checking the service metadata", func() {
				Expect(svc.Name).To(Equal(cluster.Spec.HeadlessServiceTemplate.Name))
				Expect(svc.Labels).To(Equal(cluster.Spec.HeadlessServiceTemplate.Labels))
				Expect(svc.Annotations).To(Equal(cluster.Spec.HeadlessServiceTemplate.Annotations))
			})

			By("Deleting the service", func() {
				Expect(k8sClient.Delete(ctx, svc)).To(Succeed())
			})
		})

		It("should fail on creating the cluster service with invalid owner reference", func() {
			emptyScheme := runtime.NewScheme()

			err := CreateOrUpdateClusterService(ctx, etcdcluster, k8sClient, emptyScheme)
			Expect(err).To(HaveOccurred())
		})

		It("should successfully create the client service", func() {
			svc := &corev1.Service{}
			err := CreateOrUpdateClientService(ctx, etcdcluster, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      GetClientServiceName(etcdcluster),
				Namespace: "default",
			}, svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
			Expect(svc.Spec.ClusterIP).To(Not(Equal("None")))

			Expect(k8sClient.Delete(ctx, svc)).To(Succeed())
		})

		It("should successfully create the client service with custom spec", func() {
			svc := &corev1.Service{}
			cluster := etcdcluster.DeepCopy()
			cluster.Spec.ServiceTemplate = &etcdaenixiov1alpha1.ServiceSpec{
				EmbeddedObjectMetadata: etcdaenixiov1alpha1.EmbeddedObjectMetadata{
					Name:        "client-name",
					Labels:      map[string]string{"label": "value"},
					Annotations: map[string]string{"annotation": "value"},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Name: "client",
							Port: 2389,
						},
					},
					LoadBalancerClass: ptr.To("someClass"),
				},
			}

			err := CreateOrUpdateClientService(ctx, cluster, k8sClient, k8sClient.Scheme())
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.Get(ctx, types.NamespacedName{
				Name:      cluster.Spec.ServiceTemplate.Name,
				Namespace: "default",
			}, svc)
			Expect(err).NotTo(HaveOccurred())

			By("checking the service metadata", func() {
				Expect(svc.Name).To(Equal(cluster.Spec.ServiceTemplate.Name))
				Expect(svc.Labels).To(Equal(cluster.Spec.ServiceTemplate.Labels))
				Expect(svc.Annotations).To(Equal(cluster.Spec.ServiceTemplate.Annotations))
			})

			By("checking the overrided service spec", func() {
				Expect(svc.Spec.Type).To(Equal(cluster.Spec.ServiceTemplate.Spec.Type))
				Expect(svc.Spec.Ports).To(HaveLen(1))
				Expect(svc.Spec.Ports[0].Port).To(Equal(cluster.Spec.ServiceTemplate.Spec.Ports[0].Port))
				Expect(svc.Spec.Ports[0].Name).To(Equal(cluster.Spec.ServiceTemplate.Spec.Ports[0].Name))
				Expect(svc.Spec.LoadBalancerClass).To(Equal(cluster.Spec.ServiceTemplate.Spec.LoadBalancerClass))
			})

			By("checing default service spec", func() {
				expectedLabels := NewLabelsBuilder().WithName().WithInstance(cluster.Name).WithManagedBy()

				Expect(svc.Spec.Selector).To(HaveLen(3))
				Expect(svc.Spec.Selector["app.kubernetes.io/name"]).To(Equal(expectedLabels["app.kubernetes.io/name"]))
				Expect(svc.Spec.Selector["app.kubernetes.io/instance"]).To(Equal(expectedLabels["app.kubernetes.io/instance"]))
				Expect(svc.Spec.Selector["app.kubernetes.io/managed-by"]).To(Equal(expectedLabels["app.kubernetes.io/managed-by"]))
			})

			By("Deleting the service", func() {
				Expect(k8sClient.Delete(ctx, svc)).To(Succeed())
			})
		})

		It("should fail on creating the client service with invalid owner reference", func() {
			emptyScheme := runtime.NewScheme()

			err := CreateOrUpdateClientService(ctx, etcdcluster, k8sClient, emptyScheme)
			Expect(err).To(HaveOccurred())
		})
	})
})
