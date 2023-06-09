package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/golang/glog"
	"k8s.io/client-go/rest"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1 "k8s.io/api/admission/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	admissionApi  = "admission.k8s.io/v1"
	admissionKind = "AdmissionReview"
)

var (
	owner = bool(true)
)

// NavlinksServerHandler listen to admission requests and serve responses
type NavlinksServerHandler struct {
}

func (nls *NavlinksServerHandler) healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (nls *NavlinksServerHandler) serve(w http.ResponseWriter, r *http.Request) {

	var body []byte
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// Url path of metrics
	if r.URL.Path == "/metrics" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Url path of admission
	if r.URL.Path != "/validate" {
		glog.Error("no validate")
		http.Error(w, "no validate", http.StatusBadRequest)
		return
	}

	if len(body) == 0 {
		glog.Error("empty body")
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	// count each request for prometheus metric
	opsProcessed.Inc()
	arRequest := v1.AdmissionReview{}
	if err := json.Unmarshal(body, &arRequest); err != nil {
		glog.Error("incorrect body")
		http.Error(w, "incorrect body", http.StatusBadRequest)
		return
	}

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Error("cant get InCluster config: ", err)
		nls.response(false, "InCluster config failed", w, &arRequest)
	}
	// creates the clientset
	clientset := NewForConfigOrDie(config)
	if err != nil {
		glog.Error("cant setup clientset: ", arRequest.Request.Namespace)
		nls.response(false, "Setup clientset failed", w, &arRequest)
	}

	// check if navlink resource is available on api server
	_, err = clientset.Navlinks().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		glog.Error("navlinks resource not available: ", err)
		nls.response(true, "Navlink resource not available, skill all", w, &arRequest)
	}

	// switch operation mode
	operation := arRequest.Request.Operation
	switch operation {
	case v1.Create:

		raw := arRequest.Request.Object.Raw
		prom := monitoringv1.Prometheus{}
		if err := json.Unmarshal(raw, &prom); err != nil {
			glog.Error("error deserializing pod")
			nls.response(false, "Deserializing failed", w, &arRequest)
			return
		}

		ns := prom.Namespace
		if len(ns) == 0 {
			glog.Errorf("No namespace found %s/%s", prom.Name, prom.Namespace)
			resp, err := json.Marshal(admissionResponse(200, true, "Success", "Navlinks create skipped", &arRequest))
			if err != nil {
				glog.Errorf("Can't encode response: %v", err)
				http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
			}
			if _, err := w.Write(resp); err != nil {
				glog.Errorf("Can't write response: %v", err)
				http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
			}
			return
		}
		// create navlink resource prometheus-operated
		navPrometheus := specNavlinks(ns, "prometheus-operated", "9090", string(arRequest.Request.UID), logoPrometheus)
		_, err = clientset.Navlinks().Create(context.TODO(), &navPrometheus, metav1.CreateOptions{})
		if err != nil {
			if k8serrors.IsAlreadyExists(err) {
				glog.Error("navlinks prometheus already exists for ", ns)
				nls.response(true, "Navlink prometheus already exists, skipped", w, &arRequest)
				return
			}
			glog.Errorf("error creating navlinks: %v", err)
			nls.response(false, "Navlink prometheus creating failed", w, &arRequest)
		}
		glog.Info("navlinks created: ", navPrometheus.Name)

		// create navlink resource alertmanager-operated
		navAlertManager := specNavlinks(ns, "alertmanager-operated", "9093", string(arRequest.Request.UID), logoAlertmanager)
		_, err = clientset.Navlinks().Create(context.TODO(), &navAlertManager, metav1.CreateOptions{})
		if err != nil {
			if k8serrors.IsAlreadyExists(err) {
				glog.Error("navlinks alertmanager already exists for ", ns)
				nls.response(true, "Navlink alertmanager already exists, skipped", w, &arRequest)
				return
			}
			glog.Errorf("error creating navlinks: %v", err)
			nls.response(false, "Navlink alertmanager creating failed", w, &arRequest)
		}
		glog.Info("navlinks created: ", navAlertManager.Name)

		// create navlink resource prometheus-monitoring-grafana
		navGrafana := specNavlinks(ns, "project-monitoring-grafana", "80", string(arRequest.Request.UID), logoGrafana)
		_, err = clientset.Navlinks().Create(context.TODO(), &navGrafana, metav1.CreateOptions{})
		if err != nil {
			if k8serrors.IsAlreadyExists(err) {
				glog.Error("navlinks grafana already exists for ", ns)
				nls.response(true, "Navlink grafana already exists, skipped", w, &arRequest)
				return
			}
			glog.Errorf("error creating navlinks: %v", err)
			nls.response(false, "Navlink grafana creating failed", w, &arRequest)
		}
		glog.Info("navlinks created: ", navGrafana.Name)

		resp, err := json.Marshal(admissionResponse(200, true, "Success", "Navlinks create", &arRequest))
		if err != nil {
			glog.Errorf("Can't encode response: %v", err)
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		}
		if _, err := w.Write(resp); err != nil {
			glog.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
	case v1.Delete:
		// delete navlink resource prometheus-operated
		navPrometheus := specNavlinks(arRequest.Request.Namespace, "prometheus-operated", "9090", string(arRequest.Request.UID), logoPrometheus)
		err = clientset.Navlinks().Delete(context.TODO(), navPrometheus.Name, metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				glog.Error("navlinks prometheus already deleted for ", arRequest.Request.Namespace)
				nls.response(true, "Navlink prometheus already exists, skipped", w, &arRequest)
				return
			}
			glog.Errorf("error deleting navlinks: %v", err)
			nls.response(false, "Navlink prometheus deleting failed", w, &arRequest)
		}
		glog.Info("navlinks deleted: ", navPrometheus.Name)

		// delete navlink resource alertmanager-operated
		navAlertManager := specNavlinks(arRequest.Request.Namespace, "alertmanager-operated", "9093", string(arRequest.Request.UID), logoAlertmanager)
		err = clientset.Navlinks().Delete(context.TODO(), navAlertManager.Name, metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				glog.Error("navlinks alertmanager already deleted for ", arRequest.Request.Namespace)
				nls.response(true, "Navlink alertmanager already deleted, skipped", w, &arRequest)
				return
			}
			glog.Errorf("error deleting navlinks: %v", err)
			nls.response(false, "Navlink alertmanager deleting failed", w, &arRequest)
		}
		glog.Info("navlinks deleted: ", navAlertManager.Name)

		// delete navlink resource prometheus-monitoring-grafana
		navGrafana := specNavlinks(arRequest.Request.Namespace, "project-monitoring-grafana", "80", string(arRequest.Request.UID), logoGrafana)
		err = clientset.Navlinks().Delete(context.TODO(), navGrafana.Name, metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				glog.Error("navlinks grafana already deleted for ", arRequest.Request.Namespace)
				nls.response(true, "Navlink grafana already deleted, skipped", w, &arRequest)
				return
			}
			glog.Errorf("error deleting navlinks: %v", err)
			nls.response(false, "Navlink grafana deleting failed", w, &arRequest)
		}
		glog.Info("navlinks deleted: ", navGrafana.Name)

		resp, err := json.Marshal(admissionResponse(200, true, "Success", "Navlinks delete", &arRequest))
		if err != nil {
			glog.Errorf("Can't encode response: %v", err)
			http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
		}
		if _, err := w.Write(resp); err != nil {
			glog.Errorf("Can't write response: %v", err)
			http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
		}
	default:
		glog.Error("wrong operation mode")
	}

}

func (nls *NavlinksServerHandler) response(allowed bool, message string, w http.ResponseWriter, arRequest *v1.AdmissionReview) {
	resp, err := json.Marshal(admissionResponse(200, allowed, "Success", message, arRequest))
	if err != nil {
		glog.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	if _, err := w.Write(resp); err != nil {
		glog.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}
