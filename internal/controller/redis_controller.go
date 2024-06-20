/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cachev1 "having.dlrow/redis-controller/api/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=cache.ha-ving.store,resources=redis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cache.ha-ving.store,resources=redis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cache.ha-ving.store,resources=redis/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Redis object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *RedisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	redis := &cachev1.Redis{}
	if err := r.Get(ctx, req.NamespacedName, redis); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// TODO(user): your logic here

	if redis.Spec.Size != 3 {
		return ctrl.Result{}, fmt.Errorf("Size must be 3")
	}

	masterName := fmt.Sprintf("%s-master", redis.Name)
	slave1Name := fmt.Sprintf("%s-slave1", redis.Name)
	slave2Name := fmt.Sprintf("%s-slave2", redis.Name)

	// Redis master
	master := r.newRedisPod(redis, masterName)
	if err := r.ensurePod(ctx, master); err != nil {
		return ctrl.Result{}, err
	}

	// Redis slave 1
	slave1 := r.newRedisPod(redis, slave1Name)
	slave1.Spec.Containers[0].Args = []string{"--slaveof", masterName, "6379"}
	if err := r.ensurePod(ctx, slave1); err != nil {
		return ctrl.Result{}, err
	}

	// Redis slave 2
	slave2 := r.newRedisPod(redis, slave2Name)
	slave2.Spec.Containers[0].Args = []string{"--slaveof", masterName, "6379"}
	if err := r.ensurePod(ctx, slave2); err != nil {
		return ctrl.Result{}, err
	}

	redis.Status.Master = masterName
	redis.Status.Slaves = []string{slave1Name, slave2Name}

	if err := r.Status().Update(ctx, redis); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1.Redis{}).
		Complete(r)
}

func (r *RedisReconciler) newRedisPod(redis *cachev1.Redis, name string) *corev1.Pod {
	labels := map[string]string{
		"app":      "redis",
		"redis_cr": redis.Name,
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: redis.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "redis",
					Image: "redis:6.2.6",
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 6379,
						},
					},
				},
			},
		},
	}
}

func (r *RedisReconciler) ensurePod(ctx context.Context, pod *corev1.Pod) error {
	found := &corev1.Pod{}
	err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		err = r.Create(ctx, pod)
	}
	return err
}
