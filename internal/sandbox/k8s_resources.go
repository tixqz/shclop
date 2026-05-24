package sandbox

import (
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BuildWorkspacePVC(agentID, sandboxID, storageClassName, size string) (*corev1.PersistentVolumeClaim, error) {
	if size == "" {
		size = "10Gi"
	}
	qty, err := resource.ParseQuantity(size)
	if err != nil {
		return nil, fmt.Errorf("invalid workspace size %q: %w", size, err)
	}
	labels := RuntimeLabels(agentID, "", sandboxID)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "shclop-workspace-" + agentID,
			Labels: labels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: qty}},
			VolumeMode:  func() *corev1.PersistentVolumeMode { m := corev1.PersistentVolumeFilesystem; return &m }(),
		},
	}
	if storageClassName != "" {
		pvc.Spec.StorageClassName = &storageClassName
	}
	return pvc, nil
}

func BuildRuntimePod(spec AgentPodSpec) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: spec.Name, Labels: spec.Labels},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: &spec.AutomountServiceAccountToken,
			HostNetwork:                  spec.HostNetwork,
			HostPID:                      spec.HostPID,
			HostIPC:                      spec.HostIPC,
			Containers:                   []corev1.Container{buildRuntimeContainer(spec.Container)},
			Volumes:                      buildVolumes(spec.Volumes),
			SecurityContext:              &corev1.PodSecurityContext{FSGroup: &spec.FSGroup},
		},
	}
	if spec.RuntimeClassName != "" {
		pod.Spec.RuntimeClassName = &spec.RuntimeClassName
	}
	// Runtime pods use the node DNS resolver directly (DNSDefault) instead
	// of the cluster DNS (ClusterFirst) because Kata micro-VMs may not
	// route cluster DNS traffic correctly, and runtime pods only need
	// external DNS (LLM gateway addresses, not in-cluster services).
	pod.Spec.DNSPolicy = corev1.DNSDefault
	return pod
}

func buildRuntimeContainer(spec ContainerSpec) corev1.Container {
	container := corev1.Container{
		Name:            spec.Name,
		Image:           spec.Image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Env:             append(buildEnvVars(spec.Env), buildSecretEnvVars(spec.EnvFrom)...),
		VolumeMounts:    buildVolumeMounts(spec.VolumeMounts),
		SecurityContext: &corev1.SecurityContext{Privileged: boolPtr(false), AllowPrivilegeEscalation: boolPtr(false), ReadOnlyRootFilesystem: boolPtr(true), RunAsNonRoot: boolPtr(true), RunAsUser: &spec.RunAsUser, SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}},
	}
	if spec.CPU != "" || spec.Memory != "" {
		container.Resources = corev1.ResourceRequirements{Requests: corev1.ResourceList{}, Limits: corev1.ResourceList{}}
		if spec.CPU != "" {
			q := resource.MustParse(spec.CPU)
			container.Resources.Requests[corev1.ResourceCPU] = q
			container.Resources.Limits[corev1.ResourceCPU] = q
		}
		if spec.Memory != "" {
			q := resource.MustParse(spec.Memory)
			container.Resources.Requests[corev1.ResourceMemory] = q
			container.Resources.Limits[corev1.ResourceMemory] = q
		}
	}
	if len(spec.DropCapabilities) > 0 {
		caps := make([]corev1.Capability, 0, len(spec.DropCapabilities))
		for _, c := range spec.DropCapabilities {
			caps = append(caps, corev1.Capability(c))
		}
		container.SecurityContext.Capabilities = &corev1.Capabilities{Drop: caps}
	}
	return container
}

func buildEnvVars(env map[string]string) []corev1.EnvVar {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]corev1.EnvVar, 0, len(keys))
	for _, k := range keys {
		out = append(out, corev1.EnvVar{Name: k, Value: env[k]})
	}
	return out
}

func buildSecretEnvVars(envFrom []EnvFromSource) []corev1.EnvVar {
	if len(envFrom) == 0 {
		return nil
	}
	out := make([]corev1.EnvVar, 0, len(envFrom))
	for _, e := range envFrom {
		if e.SecretName == "" || e.SecretKey == "" || e.EnvVar == "" {
			continue
		}
		out = append(out, corev1.EnvVar{
			Name: e.EnvVar,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: e.SecretName},
					Key:                  e.SecretKey,
				},
			},
		})
	}
	return out
}

func buildVolumeMounts(mounts []VolumeMount) []corev1.VolumeMount {
	out := make([]corev1.VolumeMount, 0, len(mounts))
	for _, m := range mounts {
		out = append(out, corev1.VolumeMount{Name: m.Name, MountPath: m.MountPath, ReadOnly: m.ReadOnly})
	}
	return out
}

func buildVolumes(volumes []VolumeSpec) []corev1.Volume {
	out := make([]corev1.Volume, 0, len(volumes))
	for _, v := range volumes {
		vol := corev1.Volume{Name: v.Name}
		switch {
		case v.PVC != "":
			vol.VolumeSource.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: v.PVC}
		case v.SecretName != "":
			sv := &corev1.SecretVolumeSource{SecretName: v.SecretName}
			if v.SecretKey != "" || v.SecretPath != "" {
				path := v.SecretPath
				if path == "" {
					path = v.SecretKey
				}
				sv.Items = []corev1.KeyToPath{{Key: v.SecretKey, Path: path}}
			}
			vol.VolumeSource.Secret = sv
		}
		out = append(out, vol)
	}
	return out
}

func boolPtr(v bool) *bool { return &v }
