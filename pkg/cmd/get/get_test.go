package get

import (
	"bytes"
	"strings"
	"testing"
	"time"

	appsv1alpha1 "github.com/NVIDIA/k8s-nim-operator/api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// NIMService tests.
func newBaseNS(name, ns string) appsv1alpha1.NIMService {
	return appsv1alpha1.NIMService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
}

func withImage(ns appsv1alpha1.NIMService, repo, tag string) appsv1alpha1.NIMService {
	ns.Spec.Image.Repository = repo
	ns.Spec.Image.Tag = tag
	return ns
}

func withExposeService(ns appsv1alpha1.NIMService, name string, port int32) appsv1alpha1.NIMService {
	ns.Spec.Expose.Service.Name = name
	ns.Spec.Expose.Service.Port = ptr.To(port)
	return ns
}

func withReplicas(ns appsv1alpha1.NIMService, r int) appsv1alpha1.NIMService {
	ns.Spec.Replicas = r
	return ns
}

func withScale(ns appsv1alpha1.NIMService, enabled bool, min *int32, max int32) appsv1alpha1.NIMService {
	ns.Spec.Scale.Enabled = ptr.To(enabled)
	ns.Spec.Scale.HPA.MinReplicas = min
	ns.Spec.Scale.HPA.MaxReplicas = max
	return ns
}

func withStorageNIMCache(ns appsv1alpha1.NIMService, name, profile string) appsv1alpha1.NIMService {
	ns.Spec.Storage.NIMCache.Name = name
	ns.Spec.Storage.NIMCache.Profile = profile
	return ns
}

func withStoragePVC(ns appsv1alpha1.NIMService, name, size string) appsv1alpha1.NIMService {
	ns.Spec.Storage.PVC.Name = name
	ns.Spec.Storage.PVC.Size = size
	return ns
}

func withStorageHostPath(ns appsv1alpha1.NIMService, p string) appsv1alpha1.NIMService {
	ns.Spec.Storage.HostPath = ptr.To(p)
	return ns
}

func withSvcResources(ns appsv1alpha1.NIMService, limits, requests corev1.ResourceList, claims []corev1.ResourceClaim) appsv1alpha1.NIMService {
	ns.Spec.Resources = &corev1.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
		Claims:   claims,
	}
	return ns
}

func Test_getExpose(t *testing.T) {
	ns1 := withExposeService(newBaseNS("a", "ns"), "api", 8080)
	if got := getExpose(&ns1); got != "Name: api, Port: 8080" {
		t.Fatalf("getExpose(name+port) = %q, want %q", got, "Name: api, Port: 8080")
	}

	ns2 := withExposeService(newBaseNS("b", "ns"), "", 9090)
	if got := getExpose(&ns2); got != "Port: 9090" {
		t.Fatalf("getExpose(port only) = %q, want %q", got, "Port: 9090")
	}

	ns3 := newBaseNS("c", "ns")
	if got := getExpose(&ns3); got != "" {
		t.Fatalf("getExpose(none) = %q, want empty", got)
	}
}

func Test_getScale(t *testing.T) {
	min := int32(2)
	ns1 := withScale(newBaseNS("a", "ns"), true, &min, 5)
	if got := getScale(&ns1); got != "min: 2, max: 5" {
		t.Fatalf("getScale(min+max) = %q", got)
	}

	ns2 := withScale(newBaseNS("b", "ns"), true, nil, 10)
	if got := getScale(&ns2); got != "max: 10" {
		t.Fatalf("getScale(max only) = %q", got)
	}

	ns3 := newBaseNS("c", "ns")
	if got := getScale(&ns3); got != "disabled" {
		t.Fatalf("getScale(disabled) = %q", got)
	}
}

func Test_getStorage(t *testing.T) {
	// NIMCache
	ns1 := withStorageNIMCache(newBaseNS("a", "ns"), "cache-a", "fp8")
	if got := getStorage(&ns1); got != "NIMCache: name: cache-a, profile: fp8" {
		t.Fatalf("getStorage(NIMCache) = %q", got)
	}

	// PVC with name
	ns2 := withStoragePVC(newBaseNS("b", "ns"), "pvc-b", "20Gi")
	if got := getStorage(&ns2); got != "PVC: pvc-b, 20Gi" {
		t.Fatalf("getStorage(PVC name) = %q", got)
	}

	// PVC without name
	ns3 := withStoragePVC(newBaseNS("c", "ns"), "", "50Gi")
	if got := getStorage(&ns3); got != "PVC: 50Gi" {
		t.Fatalf("getStorage(PVC size only) = %q", got)
	}

	// HostPath
	ns4 := withStorageHostPath(newBaseNS("d", "ns"), "/models")
	if got := getStorage(&ns4); got != "HostPath: /models" {
		t.Fatalf("getStorage(HostPath) = %q", got)
	}
}

func Test_resourceListToOneLine_and_claimsToOneLine(t *testing.T) {
	limits := corev1.ResourceList{
		corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
		corev1.ResourceCPU:                    resource.MustParse("2"),
		corev1.ResourceMemory:                 resource.MustParse("4Gi"),
	}
	got := resourceListToOneLine(limits)
	// Keys should be sorted alphabetically by their string name
	if got != "cpu: 2, memory: 4Gi, nvidia.com/gpu: 1" {
		t.Fatalf("resourceListToOneLine = %q", got)
	}

	claims := []corev1.ResourceClaim{{Name: "a", Request: "req-a"}, {Name: "b", Request: "req-b"}}
	if s := claimsToOneLine(claims); s != "a(req-a), b(req-b)" {
		t.Fatalf("claimsToOneLine = %q", s)
	}
}

func Test_getNIMServiceResources(t *testing.T) {
	limits := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("4"),
		corev1.ResourceMemory: resource.MustParse("8Gi"),
	}
	requests := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("2"),
		corev1.ResourceMemory: resource.MustParse("4Gi"),
	}
	claims := []corev1.ResourceClaim{{Name: "a", Request: "req-a"}}

	ns := withSvcResources(newBaseNS("a", "ns"), limits, requests, claims)

	out := getNIMServiceResources(&ns)
	for _, sub := range []string{
		"Limits: cpu: 4, memory: 8Gi",
		"Requests: cpu: 2, memory: 4Gi",
		"Claims: a(req-a)",
	} {
		if !strings.Contains(out, sub) {
			t.Fatalf("resources output missing %q in:\n%s", sub, out)
		}
	}
}

func Test_printNIMServices(t *testing.T) {
	// Item 1
	ns1 := withReplicas(withExposeService(withImage(newBaseNS("svc1", "ns1"), "repo1", "v1"), "api", 8080), 2)
	ns1.Spec.Scale.Enabled = ptr.To(false)
	ns1 = withStoragePVC(ns1, "pvc1", "10Gi")
	ns1.Status.State = "Creating"
	ns1.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Now().Add(-1 * time.Hour))

	// Item 2
	min := int32(1)
	ns2 := withReplicas(withExposeService(withImage(newBaseNS("svc2", "ns2"), "repo2", "v2"), "", 9090), 3)
	ns2 = withScale(ns2, true, &min, 5)
	ns2 = withStorageNIMCache(ns2, "nimc", "fp8")
	ns2.Status.State = "Ready"

	list := &appsv1alpha1.NIMServiceList{Items: []appsv1alpha1.NIMService{ns1, ns2}}

	var buf bytes.Buffer
	if err := printNIMServices(list, &buf); err != nil {
		t.Fatalf("printNIMServices error: %v", err)
	}
	out := buf.String()

	// Headers (printer uppercases them)
	for _, h := range []string{"NAME", "NAMESPACE", "IMAGE", "EXPOSE SERVICE", "REPLICAS", "SCALE", "STORAGE", "RESOURCES", "STATE", "AGE"} {
		if !strings.Contains(out, h) {
			t.Fatalf("output missing header %q:\n%s", h, out)
		}
	}

	// Row assertions
	for _, s := range []string{"svc1", "ns1", "repo1 v1", "Name: api, Port: 8080", "2", "disabled", "PVC: pvc1, 10Gi", "Creating"} {
		if !strings.Contains(out, s) {
			t.Fatalf("output missing cell %q:\n%s", s, out)
		}
	}
	for _, s := range []string{"svc2", "ns2", "repo2 v2", "Port: 9090", "3", "min: 1, max: 5", "NIMCache: name: nimc, profile: fp8", "Ready"} {
		if !strings.Contains(out, s) {
			t.Fatalf("output missing cell %q:\n%s", s, out)
		}
	}
}

// NIMCache tests.
func newBaseNC(name, ns string) appsv1alpha1.NIMCache {
	return appsv1alpha1.NIMCache{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
}

func ncWithNGC(name, ns, modelPuller string) appsv1alpha1.NIMCache {
	nc := newBaseNC(name, ns)
	nc.Spec.Source.NGC = &appsv1alpha1.NGCSource{ModelPuller: modelPuller}
	return nc
}

func ncWithDataStoreEndpoint(name, ns, endpoint string) appsv1alpha1.NIMCache {
	nc := newBaseNC(name, ns)
	nc.Spec.Source.DataStore = &appsv1alpha1.NemoDataStoreSource{Endpoint: endpoint}
	return nc
}

func ncWithDataStoreModel(name, ns, modelName string) appsv1alpha1.NIMCache {
	nc := newBaseNC(name, ns)
	nc.Spec.Source.DataStore = &appsv1alpha1.NemoDataStoreSource{DSHFCommonFields: appsv1alpha1.DSHFCommonFields{ModelName: &modelName}}
	return nc
}

func withPVC(nc appsv1alpha1.NIMCache, name, size string) appsv1alpha1.NIMCache {
	nc.Spec.Storage.PVC.Name = name
	nc.Spec.Storage.PVC.Size = size
	return nc
}

func withResources(nc appsv1alpha1.NIMCache, cpu, mem string) appsv1alpha1.NIMCache {
	if cpu != "" {
		nc.Spec.Resources.CPU = resource.MustParse(cpu)
	}
	if mem != "" {
		nc.Spec.Resources.Memory = resource.MustParse(mem)
	}
	return nc
}

func withCreationTime(nc appsv1alpha1.NIMCache, t time.Time) appsv1alpha1.NIMCache {
	nc.ObjectMeta.CreationTimestamp = metav1.NewTime(t)
	return nc
}

func withState(nc appsv1alpha1.NIMCache, s string) appsv1alpha1.NIMCache {
	nc.Status.State = s
	return nc
}

func Test_getSource(t *testing.T) {
	tests := []struct {
		name string
		nc   appsv1alpha1.NIMCache
		want string
	}{
		{"NGC", ncWithNGC("a", "ns", "img:tag"), "NGC"},
		{"DataStore", ncWithDataStoreEndpoint("b", "ns", "https://datastore"), "NVIDIA NeMo DataStore"},
		{"DefaultHF", newBaseNC("c", "ns"), "HuggingFace Hub"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSource(&tt.nc)
			if got != tt.want {
				t.Fatalf("getSource() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_getModel(t *testing.T) {
	tests := []struct {
		name string
		nc   appsv1alpha1.NIMCache
		want string
	}{
		{"NGC returns ModelPuller", ncWithNGC("a", "ns", "img:tag"), "img:tag"},
		{"DataStore with Endpoint", ncWithDataStoreEndpoint("b", "ns", "https://datastore"), "https://datastore"},
		{"DataStore with ModelName", ncWithDataStoreModel("c", "ns", "my-model"), "my-model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getModel(&tt.nc)
			if got != tt.want {
				t.Fatalf("getModel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_getPVCDetails(t *testing.T) {
	nc1 := withPVC(newBaseNC("a", "ns"), "pvc-a", "10Gi")
	nc2 := withPVC(newBaseNC("b", "ns"), "", "20Gi")

	if got := getPVCDetails(&nc1); got != "pvc-a, 10Gi" {
		t.Fatalf("getPVCDetails(name+size) = %q, want %q", got, "pvc-a, 10Gi")
	}
	if got := getPVCDetails(&nc2); got != "20Gi" {
		t.Fatalf("getPVCDetails(size only) = %q, want %q", got, "20Gi")
	}
}

func Test_printNIMCaches(t *testing.T) {
	// Item 1: NGC
	nc1 := withState(withResources(withPVC(
		ncWithNGC("nc-ngc", "ns1", "img:tag"),
		"pvc1", "50Gi",
	), "2", "4Gi"), "Creating")
	nc1 = withCreationTime(nc1, time.Now().Add(-2*time.Hour))

	// Item 2: DataStore with model name, zero timestamps to force <unknown> age.
	nc2 := withState(withResources(withPVC(
		ncWithDataStoreModel("nc-ds", "ns2", "mymodel"),
		"", "200Gi",
	), "8", "32Gi"), "Ready")

	list := &appsv1alpha1.NIMCacheList{Items: []appsv1alpha1.NIMCache{nc1, nc2}}

	var buf bytes.Buffer
	if err := printNIMCaches(list, &buf); err != nil {
		t.Fatalf("printNIMCaches error: %v", err)
	}
	out := buf.String()

	for _, h := range []string{
		"NAME", "NAMESPACE", "SOURCE", "MODEL/MODELPULLER", "CPU", "MEMORY", "PVC VOLUME", "STATE", "AGE",
	} {
		if !strings.Contains(out, h) {
			t.Fatalf("output missing header %q:\n%s", h, out)
		}
	}

	for _, s := range []string{"nc-ngc", "ns1", "NGC", "img:tag", "2", "4Gi", "pvc1, 50Gi", "Creating"} {
		if !strings.Contains(out, s) {
			t.Fatalf("output missing NGC cell %q:\n%s", s, out)
		}
	}

	for _, s := range []string{"nc-ds", "ns2", "NVIDIA NeMo DataStore", "mymodel", "8", "32Gi", "200Gi", "Ready"} {
		if !strings.Contains(out, s) {
			t.Fatalf("output missing DS cell %q:\n%s", s, out)
		}
	}
}
