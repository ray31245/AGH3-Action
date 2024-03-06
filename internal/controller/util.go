package controller

import (
	"fmt"
	"slices"

	actionv1 "github.com/Leukocyte-Lab/AGH3-Action/api/v1"
	corev1 "k8s.io/api/core/v1"
)

type ActionContainerConverter struct {
	containers     []actionv1.Container
	initContainers []actionv1.Container
}

func (c ActionContainerConverter) Converter(pod *corev1.PodSpec) {
	map2EnvVar := func(env map[string]string) []corev1.EnvVar {
		if len(env) > 0 {
			res := []corev1.EnvVar{}
			for k, v := range env {
				res = append(res, corev1.EnvVar{Name: k, Value: v})
			}
			return res
		}
		return nil
	}
	map2VolumeMounts := func(VolumeMounts map[string]string) []corev1.VolumeMount {
		res := []corev1.VolumeMount{}
		for k, v := range VolumeMounts {
			upsertVolumes(pod, k)

			res = append(res, corev1.VolumeMount{Name: k, MountPath: v})
		}
		return res
	}
	resContainers := []corev1.Container{}
	for i, container := range c.containers {
		resContainers = append(resContainers, corev1.Container{
			Name:         fmt.Sprintf("%d", i),
			Image:        container.Image,
			Command:      container.Command,
			Args:         container.Args,
			Env:          map2EnvVar(container.Env),
			VolumeMounts: map2VolumeMounts(container.VolumeMounts),
		})
	}
	pod.Containers = resContainers

	resInitContainers := []corev1.Container{}
	for i, container := range c.initContainers {
		resContainers = append(resContainers, corev1.Container{
			Name:         fmt.Sprintf("%d", i),
			Image:        container.Image,
			Command:      container.Command,
			Args:         container.Args,
			Env:          map2EnvVar(container.Env),
			VolumeMounts: map2VolumeMounts(container.VolumeMounts),
		})
	}
	pod.InitContainers = resInitContainers
}

// append volume if the volume that have volumeName not exist
func upsertVolumes(pod *corev1.PodSpec, volumeName string) {
	volumeExist := slices.ContainsFunc(pod.Volumes, func(vol corev1.Volume) bool {
		return vol.Name == volumeName
	})
	if !volumeExist {
		pod.Volumes = append(pod.Volumes, corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: "Memory",
				},
			},
		})
	}
}
