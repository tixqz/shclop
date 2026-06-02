package controllers

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	appsv1apply "k8s.io/client-go/applyconfigurations/apps/v1"
	corev1apply "k8s.io/client-go/applyconfigurations/core/v1"
	metav1apply "k8s.io/client-go/applyconfigurations/meta/v1"
	networkingv1apply "k8s.io/client-go/applyconfigurations/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/mipopov/shclop/internal/k8s"
)

const fieldManager = "shclop-mcp-controller"

// MCPServerController reconciles MCPServer CRs into Deployments, Services, and
// NetworkPolicies inside the given runtime namespace.
type MCPServerController struct {
	Client    *k8s.Client
	Informer  cache.SharedIndexInformer
	Namespace string
	Logger    *slog.Logger

	queue workqueue.TypedRateLimitingInterface[string]
}

// NewMCPServerController constructs a controller.  Call Run to start it.
func NewMCPServerController(client *k8s.Client, informer cache.SharedIndexInformer, namespace string, logger *slog.Logger) *MCPServerController {
	return &MCPServerController{
		Client:    client,
		Informer:  informer,
		Namespace: namespace,
		Logger:    logger,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "mcpserver"},
		),
	}
}

// Run starts N workers and blocks until ctx is done.
func (c *MCPServerController) Run(ctx context.Context, workers int) error {
	_, err := c.Informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(key)
			}
		},
		UpdateFunc: func(_, newObj any) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				c.queue.Add(key)
			}
		},
		DeleteFunc: func(obj any) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				c.queue.Add(key)
			}
		},
	})
	if err != nil {
		return fmt.Errorf("add event handler: %w", err)
	}

	if !cache.WaitForCacheSync(ctx.Done(), c.Informer.HasSynced) {
		return fmt.Errorf("cache sync timed out")
	}

	for range workers {
		go func() {
			for c.processNextItem(ctx) {
			}
		}()
	}

	<-ctx.Done()
	c.queue.ShutDown()
	return nil
}

func (c *MCPServerController) processNextItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	err := c.reconcile(ctx, key)
	if err != nil {
		c.Logger.Warn("mcpserver_controller: reconcile error", "key", key, "err", err)
		c.queue.AddRateLimited(key)
		c.queue.Done(key)
		return true
	}

	c.queue.Forget(key)
	c.queue.Done(key)
	return true
}

func (c *MCPServerController) reconcile(ctx context.Context, key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("split key %q: %w", key, err)
	}

	item, exists, err := c.Informer.GetStore().GetByKey(key)
	if err != nil {
		return fmt.Errorf("store get %q: %w", key, err)
	}
	if !exists {
		return c.cleanup(ctx, ns, name)
	}

	u, ok := item.(*unstructured.Unstructured)
	if !ok {
		c.Logger.Warn("mcpserver_controller: unexpected object type", "key", key, "type", fmt.Sprintf("%T", item))
		return nil
	}

	cr, err := k8s.UnstructuredToMCPServer(u)
	if err != nil {
		c.Logger.Warn("mcpserver_controller: conversion failed", "key", key, "err", err)
		return nil
	}

	if err := c.applyDeployment(ctx, cr); err != nil {
		return fmt.Errorf("apply deployment: %w", err)
	}
	if err := c.applyService(ctx, cr); err != nil {
		return fmt.Errorf("apply service: %w", err)
	}
	if err := c.applyNetworkPolicy(ctx, cr); err != nil {
		return fmt.Errorf("apply networkpolicy: %w", err)
	}

	c.updateStatus(ctx, cr)
	return nil
}

// resourceName returns the name of child resources created for a CR.
func resourceName(crName string) string {
	return "mcp-" + crName
}

// selectorLabels returns the labels used on pod templates, deployment selectors,
// service selectors, and NetworkPolicy podSelectors.
func selectorLabels(crName string) map[string]string {
	return map[string]string{
		"shclop.io/managed-by": fieldManager,
		"shclop.io/mcp-server": crName,
	}
}

func (c *MCPServerController) applyDeployment(ctx context.Context, cr *k8s.MCPServer) error {
	replicas := cr.Spec.Replicas
	if replicas <= 0 {
		replicas = 1
	}

	labels := selectorLabels(cr.Name)

	envVars := make([]*corev1apply.EnvVarApplyConfiguration, 0, len(cr.Spec.Env))
	for i := range cr.Spec.Env {
		ev := &cr.Spec.Env[i]
		e := corev1apply.EnvVar().WithName(ev.Name)
		if ev.ValueFrom != nil {
			src := corev1apply.EnvVarSource()
			if ev.ValueFrom.SecretKeyRef != nil {
				src = src.WithSecretKeyRef(
					corev1apply.SecretKeySelector().
						WithName(ev.ValueFrom.SecretKeyRef.Name).
						WithKey(ev.ValueFrom.SecretKeyRef.Key),
				)
			}
			e = e.WithValueFrom(src)
		} else {
			e = e.WithValue(ev.Value)
		}
		envVars = append(envVars, e)
	}

	container := corev1apply.Container().
		WithName("mcp").
		WithImage(cr.Spec.Image).
		WithPorts(
			corev1apply.ContainerPort().WithContainerPort(cr.Spec.Port),
		).
		WithEnv(envVars...)

	if cr.Spec.Resources.Limits != nil || cr.Spec.Resources.Requests != nil {
		res := corev1apply.ResourceRequirements()
		if cr.Spec.Resources.Limits != nil {
			res = res.WithLimits(cr.Spec.Resources.Limits)
		}
		if cr.Spec.Resources.Requests != nil {
			res = res.WithRequests(cr.Spec.Resources.Requests)
		}
		container = container.WithResources(res)
	}

	podSpec := corev1apply.PodSpec().
		WithContainers(container)
	if cr.Spec.ServiceAccountName != "" {
		podSpec = podSpec.WithServiceAccountName(cr.Spec.ServiceAccountName)
	}

	deploy := appsv1apply.Deployment(resourceName(cr.Name), c.Namespace).
		WithLabels(labels).
		WithSpec(
			appsv1apply.DeploymentSpec().
				WithReplicas(replicas).
				WithSelector(
					metav1apply.LabelSelector().WithMatchLabels(labels),
				).
				WithTemplate(
					corev1apply.PodTemplateSpec().
						WithLabels(labels).
						WithSpec(podSpec),
				),
		)

	_, err := c.Client.Kubernetes.AppsV1().Deployments(c.Namespace).Apply(
		ctx, deploy, metav1.ApplyOptions{FieldManager: fieldManager, Force: true},
	)
	return err
}

func (c *MCPServerController) applyService(ctx context.Context, cr *k8s.MCPServer) error {
	labels := selectorLabels(cr.Name)

	svc := corev1apply.Service(resourceName(cr.Name), c.Namespace).
		WithLabels(labels).
		WithSpec(
			corev1apply.ServiceSpec().
				WithType(corev1.ServiceTypeClusterIP).
				WithSelector(labels).
				WithPorts(
					corev1apply.ServicePort().
						WithName("mcp").
						WithPort(cr.Spec.Port).
						WithTargetPort(intstr.FromInt32(cr.Spec.Port)),
				),
		)

	_, err := c.Client.Kubernetes.CoreV1().Services(c.Namespace).Apply(
		ctx, svc, metav1.ApplyOptions{FieldManager: fieldManager, Force: true},
	)
	return err
}

func (c *MCPServerController) applyNetworkPolicy(ctx context.Context, cr *k8s.MCPServer) error {
	labels := selectorLabels(cr.Name)

	ingressRule := networkingv1apply.NetworkPolicyIngressRule().
		WithFrom(
			networkingv1apply.NetworkPolicyPeer().
				WithPodSelector(
					metav1apply.LabelSelector().WithMatchLabels(map[string]string{
						"shclop.io/role": "agent-runtime",
					}),
				),
		)

	var egressRules []*networkingv1apply.NetworkPolicyEgressRuleApplyConfiguration
	cidrs := cr.Spec.NetworkPolicy.EgressCIDRs
	if len(cidrs) == 0 {
		cidrs = []string{"0.0.0.0/0"}
	}
	for _, cidr := range cidrs {
		egressRules = append(egressRules,
			networkingv1apply.NetworkPolicyEgressRule().
				WithTo(
					networkingv1apply.NetworkPolicyPeer().
						WithIPBlock(
							networkingv1apply.IPBlock().WithCIDR(cidr),
						),
				),
		)
	}

	np := networkingv1apply.NetworkPolicy(resourceName(cr.Name), c.Namespace).
		WithLabels(labels).
		WithSpec(
			networkingv1apply.NetworkPolicySpec().
				WithPodSelector(
					metav1apply.LabelSelector().WithMatchLabels(labels),
				).
				WithIngress(ingressRule).
				WithEgress(egressRules...).
				WithPolicyTypes(networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress),
		)

	_, err := c.Client.Kubernetes.NetworkingV1().NetworkPolicies(c.Namespace).Apply(
		ctx, np, metav1.ApplyOptions{FieldManager: fieldManager, Force: true},
	)
	return err
}

// updateStatus patches status.serviceURL and status.phase on the CR.
// Failures are logged but do not trigger a requeue.
func (c *MCPServerController) updateStatus(ctx context.Context, cr *k8s.MCPServer) {
	serviceURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
		resourceName(cr.Name), c.Namespace, cr.Spec.Port)

	uo, err := c.Client.MCPServers(c.Namespace).Get(ctx, cr.Name, metav1.GetOptions{})
	if err != nil {
		c.Logger.Warn("mcpserver_controller: status get failed", "name", cr.Name, "err", err)
		return
	}

	if err := unstructured.SetNestedField(uo.Object, serviceURL, "status", "serviceURL"); err != nil {
		c.Logger.Warn("mcpserver_controller: set serviceURL failed", "err", err)
		return
	}
	if err := unstructured.SetNestedField(uo.Object, "Ready", "status", "phase"); err != nil {
		c.Logger.Warn("mcpserver_controller: set phase failed", "err", err)
		return
	}

	if _, err := c.Client.MCPServers(c.Namespace).UpdateStatus(ctx, uo, metav1.UpdateOptions{}); err != nil {
		c.Logger.Warn("mcpserver_controller: UpdateStatus failed", "name", cr.Name, "err", err)
	}
}

// cleanup deletes all resources owned by the CR (best-effort, ignores NotFound).
func (c *MCPServerController) cleanup(ctx context.Context, ns, crName string) error {
	name := resourceName(crName)
	del := metav1.DeleteOptions{}

	if err := c.Client.Kubernetes.AppsV1().Deployments(ns).Delete(ctx, name, del); err != nil && !apierrors.IsNotFound(err) {
		c.Logger.Warn("mcpserver_controller: cleanup deployment failed", "name", name, "err", err)
	}
	if err := c.Client.Kubernetes.CoreV1().Services(ns).Delete(ctx, name, del); err != nil && !apierrors.IsNotFound(err) {
		c.Logger.Warn("mcpserver_controller: cleanup service failed", "name", name, "err", err)
	}
	if err := c.Client.Kubernetes.NetworkingV1().NetworkPolicies(ns).Delete(ctx, name, del); err != nil && !apierrors.IsNotFound(err) {
		c.Logger.Warn("mcpserver_controller: cleanup networkpolicy failed", "name", name, "err", err)
	}
	return nil
}

