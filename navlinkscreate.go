package main

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	uiv1 "github.com/rancher/rancher/pkg/apis/ui.cattle.io/v1"
)

func specNavlinks(namespace string, service string, port string, uid string, icon string) uiv1.NavLink {
	return uiv1.NavLink{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "monitoring-" + namespace + "-" + service,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "monitoring.coreos.com/v1",
					Kind:               "Prometheus",
					Name:               "valinkswebhook",
					UID:                types.UID(uid),
					Controller:         &owner,
					BlockOwnerDeletion: &owner,
				},
			},
		},
		Spec: uiv1.NavLinkSpec{
			Target: "_blank",
			Group:  "monitoring-" + namespace,
			ToService: &uiv1.NavLinkTargetService{
				Namespace: namespace,
				Name:      service,
				Scheme:    "http",
				Port:      &intstr.IntOrString{Type: intstr.String, StrVal: port},
				Path:      "",
			},
			IconSrc: icon,
		},
	}
}
