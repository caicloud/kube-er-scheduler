package main

import (
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/glog"
)

const (
	addr = ":8089"
)

var mux map[string]func(http.ResponseWriter, *http.Request)

// SchedulerHandler implements custom handler
type SchedulerHandler struct{}

func (*SchedulerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	glog.V(2).Infof("request url: %s", r.URL.Path)
	if h, ok := mux[r.URL.Path]; ok {
		h(w, r)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	io.WriteString(w, "Your request URL is not found")
}

func main() {
	var master, kubeConfig *string
	if home := homeDir(); home != "" {
		kubeConfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeConfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	master = flag.String("master", "http://127.0.0.1:8080", "kubernetes cluster default address")
	flag.Parse()

	clientset, err := CreateClientset(master, kubeConfig)
	if err != nil {
		glog.Fatalf("create clientset error: %v", err)
	}

	mux = make(map[string]func(http.ResponseWriter, *http.Request))
	mux["/scheduler/predicates"] = Predicates(clientset)

	server := &http.Server{
		Addr:         addr,
		Handler:      &SchedulerHandler{},
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	glog.V(2).Info("scheduler server is starting")

	if err := server.ListenAndServe(); err != nil {
		glog.Fatalf("scheduler server start failed: %v", err)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
