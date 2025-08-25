// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"istio.io/istio/pkg/ptr"
)

var ctx = context.Background()

func TestReconcile(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(configuration *admissionv1.MutatingWebhookConfiguration)
		probeFunc    func(context.Context, *admissionv1.MutatingWebhookConfiguration) (bool, error)
		interceptors interceptor.Funcs
		expectResult ctrl.Result
		expectErr    error
		expectValue  string
	}{
		{
			name: "ready",
			probeFunc: func(context.Context, *admissionv1.MutatingWebhookConfiguration) (bool, error) {
				return true, nil
			},
			expectResult: ctrl.Result{RequeueAfter: defaultPeriodSeconds * time.Second},
			expectValue:  "true",
		},
		{
			name: "not ready",
			probeFunc: func(context.Context, *admissionv1.MutatingWebhookConfiguration) (bool, error) {
				return false, nil
			},
			expectResult: ctrl.Result{RequeueAfter: defaultPeriodSeconds * time.Second},
			expectValue:  "false",
		},
		{
			name: "probe error",
			probeFunc: func(context.Context, *admissionv1.MutatingWebhookConfiguration) (bool, error) {
				return false, fmt.Errorf("some error")
			},
			expectResult: ctrl.Result{RequeueAfter: defaultPeriodSeconds * time.Second},
			expectValue:  "false",
		},
		{
			name: "update error",
			probeFunc: func(context.Context, *admissionv1.MutatingWebhookConfiguration) (bool, error) {
				return true, nil
			},
			interceptors: interceptor.Funcs{
				Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					return errors.New("some error")
				},
			},
			expectResult: ctrl.Result{},
			expectErr:    errors.New("some error"),
			expectValue:  "",
		},
		{
			name: "honors period annotation",
			setup: func(webhook *admissionv1.MutatingWebhookConfiguration) {
				webhook.Annotations = map[string]string{
					constants.WebhookReadinessProbePeriodSecondsAnnotationKey: "123",
				}
			},
			probeFunc: func(context.Context, *admissionv1.MutatingWebhookConfiguration) (bool, error) {
				return true, nil
			},
			expectResult: ctrl.Result{RequeueAfter: 123 * time.Second},
			expectValue:  "true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			webhook := &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "istio-sidecar-injector",
				},
			}
			if tt.setup != nil {
				tt.setup(webhook)
			}

			cl := newFakeClientBuilder().
				WithObjects(webhook).
				WithInterceptorFuncs(tt.interceptors).
				Build()
			r := NewReconciler(newReconcilerTestConfig(t), cl, scheme.Scheme)
			r.probe = tt.probeFunc

			result, err := r.Reconcile(ctx, webhook)

			g.Expect(result).To(Equal(tt.expectResult))
			if tt.expectErr != nil {
				g.Expect(err).To(Equal(tt.expectErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			g.Expect(cl.Get(ctx, kube.Key("istio-sidecar-injector"), webhook)).To(Succeed())
			g.Expect(webhook.Annotations[constants.WebhookReadinessProbeStatusAnnotationKey]).To(Equal(tt.expectValue), "Unexpected annotation value")
		})
	}
}

func TestDoProbe(t *testing.T) {
	svc := admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"}
	host := svc.Name + "." + svc.Namespace + ".svc"

	// Generate a self-signed certificate and key
	certPEM, keyPEM, err := generateSelfSignedCert(host)
	if err != nil {
		panic(err)
	}

	// Load the certificate and key
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}

	// Create a custom TLS configuration using the certificate and key
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	tests := []struct {
		name           string
		webhook        *admissionv1.MutatingWebhookConfiguration
		httpStatus     int
		serverDelay    time.Duration
		contextTimout  time.Duration
		maxDuration    time.Duration
		expectedResult bool
		expectedError  string
	}{
		{
			name: "No webhooks",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				Webhooks: []admissionv1.MutatingWebhook{},
			},
			expectedResult: false,
			expectedError:  "mutatingwebhookconfiguration contains no webhooks",
		},
		{
			name: "No service in client config",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				Webhooks: []admissionv1.MutatingWebhook{
					{ClientConfig: admissionv1.WebhookClientConfig{Service: nil}},
				},
			},
			expectedResult: false,
			expectedError:  "missing webhooks[].clientConfig.service",
		},
		{
			name: "Missing CA bundle",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				Webhooks: []admissionv1.MutatingWebhook{
					{
						ClientConfig: admissionv1.WebhookClientConfig{
							Service:  &svc,
							CABundle: nil,
						},
					},
				},
			},
			expectedResult: false,
			expectedError:  "webhooks[].clientConfig.caBundle hasn't been set; check if the remote istiod can access this cluster",
		},
		{
			name: "Invalid CA bundle",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				Webhooks: []admissionv1.MutatingWebhook{
					{
						ClientConfig: admissionv1.WebhookClientConfig{
							Service:  &svc,
							CABundle: []byte("invalid"),
						},
					},
				},
			},
			expectedResult: false,
			expectedError:  "failed to append CA bundle to cert pool",
		},
		{
			name: "Unsuccessful HTTP response",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				Webhooks: []admissionv1.MutatingWebhook{
					{
						ClientConfig: admissionv1.WebhookClientConfig{
							Service:  &svc,
							CABundle: certPEM,
						},
					},
				},
			},
			httpStatus:     http.StatusInternalServerError,
			expectedResult: false,
			expectedError:  "",
		},
		{
			name: "Successful HTTP response",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				Webhooks: []admissionv1.MutatingWebhook{
					{
						ClientConfig: admissionv1.WebhookClientConfig{
							Service:  &svc,
							CABundle: certPEM,
						},
					},
				},
			},
			httpStatus:     http.StatusOK,
			expectedResult: true,
			expectedError:  "",
		},
		{
			name: "Context timeout",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				Webhooks: []admissionv1.MutatingWebhook{
					{
						ClientConfig: admissionv1.WebhookClientConfig{
							Service:  &svc,
							CABundle: certPEM,
						},
					},
				},
			},
			httpStatus:     http.StatusOK,
			serverDelay:    10 * time.Second,
			contextTimout:  1 * time.Second,
			maxDuration:    1500 * time.Millisecond,
			expectedResult: false,
			expectedError:  "context deadline exceeded",
		},
		{
			name: "Default probe timeout",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				Webhooks: []admissionv1.MutatingWebhook{
					{
						ClientConfig: admissionv1.WebhookClientConfig{
							Service:  &svc,
							CABundle: certPEM,
						},
					},
				},
			},
			httpStatus:     http.StatusOK,
			serverDelay:    defaultTimeoutSeconds + 10*time.Second,
			maxDuration:    defaultTimeoutSeconds*time.Second + 500*time.Millisecond,
			expectedResult: false,
			expectedError:  "context deadline exceeded",
		},
		{
			name: "Probe timeout annotation",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						constants.WebhookReadinessProbeTimeoutSecondsAnnotationKey: "1",
					},
				},
				Webhooks: []admissionv1.MutatingWebhook{
					{
						ClientConfig: admissionv1.WebhookClientConfig{
							Service:  &svc,
							CABundle: certPEM,
						},
					},
				},
			},
			httpStatus:     http.StatusOK,
			serverDelay:    10 * time.Second,
			maxDuration:    1500 * time.Millisecond,
			expectedResult: false,
			expectedError:  "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.serverDelay > 0 {
					delay := time.NewTimer(tt.serverDelay)
					select {
					case <-delay.C:
					case <-r.Context().Done():
						delay.Stop()
					}
				}

				if r.URL.Path == "/ready" {
					w.WriteHeader(tt.httpStatus)
				} else {
					http.Error(w, "Not Found", http.StatusNotFound)
				}
			}))
			server.TLS = tlsConfig
			server.StartTLS()
			defer server.Close()

			customDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				if addr == host+":443" {
					return net.Dial(network, server.Listener.Addr().String())
				}
				return net.Dial(network, addr)
			}
			defer func() {
				customDialContext = nil
			}()

			var probeCtx context.Context
			if tt.contextTimout > 0 {
				var cancel context.CancelFunc
				probeCtx, cancel = context.WithTimeout(ctx, tt.contextTimout)
				defer cancel()
			} else {
				probeCtx = ctx
			}

			startTime := time.Now()
			result, err := doProbe(probeCtx, tt.webhook)
			stopTime := time.Now()

			if tt.maxDuration > 0 {
				g.Expect(stopTime.Sub(startTime)).To(BeNumerically("<=", tt.maxDuration))
			}

			g.Expect(result).To(Equal(tt.expectedResult))
			if tt.expectedError == "" {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedError))
			}
		})
	}
}

func TestIsOwnedByRevisionWithRemoteControlPlane(t *testing.T) {
	tests := []struct {
		name         string
		ownerRefs    []metav1.OwnerReference
		objects      []client.Object
		interceptors interceptor.Funcs
		expected     bool
	}{
		{
			name:      "No owner references",
			ownerRefs: []metav1.OwnerReference{},
			expected:  false,
		},
		{
			name: "Owner reference not IstioRevision",
			ownerRefs: []metav1.OwnerReference{
				{
					APIVersion: "someothergroup/v1",
					Kind:       "SomeKind",
				},
			},
			expected: false,
		},
		{
			name: "IstioRevision not found",
			ownerRefs: []metav1.OwnerReference{
				{
					APIVersion: v1.GroupVersion.String(),
					Kind:       v1.IstioRevisionKind,
					Name:       "revision1",
				},
			},
			expected: false,
		},
		{
			name: "IstioRevision fetch error",
			ownerRefs: []metav1.OwnerReference{
				{
					APIVersion: v1.GroupVersion.String(),
					Kind:       v1.IstioRevisionKind,
					Name:       "revision1",
				},
			},
			interceptors: interceptor.Funcs{
				Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errors.New("some error")
				},
			},
			expected: false,
		},
		{
			name: "IstioRevision not using remote profile",
			ownerRefs: []metav1.OwnerReference{
				{
					APIVersion: v1.GroupVersion.String(),
					Kind:       v1.IstioRevisionKind,
					Name:       "revision1",
				},
			},
			objects: []client.Object{
				&v1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "revision1",
					},
					Spec: v1.IstioRevisionSpec{},
				},
			},
			expected: false,
		},
		{
			name: "IstioRevision uses remote profile",
			ownerRefs: []metav1.OwnerReference{
				{
					APIVersion: v1.GroupVersion.String(),
					Kind:       v1.IstioRevisionKind,
					Name:       "revision1",
				},
			},
			objects: []client.Object{
				&v1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: "revision1",
					},
					Spec: v1.IstioRevisionSpec{
						Values: &v1.Values{
							Profile: ptr.Of("remote"),
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := newFakeClientBuilder().
				WithObjects(tt.objects...).
				WithInterceptorFuncs(tt.interceptors).
				Build()

			obj := &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: tt.ownerRefs,
				},
			}

			result := IsOwnedByRevisionWithRemoteControlPlane(cl, obj)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestGetReadinessProbeURL(t *testing.T) {
	tests := []struct {
		name      string
		config    admissionv1.WebhookClientConfig
		expectURL string
		expectErr bool
	}{
		{
			name: "URL",
			config: admissionv1.WebhookClientConfig{
				URL: ptr.Of("https://some.url"),
			},
			expectErr: true,
		},
		{
			name:      "no URL or Service",
			config:    admissionv1.WebhookClientConfig{},
			expectErr: true,
		},
		{
			name: "default port",
			config: admissionv1.WebhookClientConfig{
				Service: &admissionv1.ServiceReference{
					Name:      "istiod",
					Namespace: "istio-system",
				},
			},
			expectURL: "https://istiod.istio-system.svc:443/ready",
		},
		{
			name: "custom port",
			config: admissionv1.WebhookClientConfig{
				Service: &admissionv1.ServiceReference{
					Name:      "istiod",
					Namespace: "istio-system",
					Port:      ptr.Of(int32(123)),
				},
			},
			expectURL: "https://istiod.istio-system.svc:123/ready",
		},
		{
			name: "ignores path",
			config: admissionv1.WebhookClientConfig{
				Service: &admissionv1.ServiceReference{
					Name:      "istiod",
					Namespace: "istio-system",
					Path:      ptr.Of("/some/path"),
				},
			},
			expectURL: "https://istiod.istio-system.svc:443/ready",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got, err := getReadinessProbeURL(tt.config)

			g.Expect(got).To(Equal(tt.expectURL))
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func newFakeClientBuilder() *fake.ClientBuilder {
	return fake.NewClientBuilder().
		WithScheme(scheme.Scheme)
}

func generateSelfSignedCert(dnsNames ...string) (certPEM []byte, keyPEM []byte, err error) {
	// Generate a private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	// Create a template for the certificate
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
	}

	// Generate a self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	// Encode the certificate and key to PEM format
	certPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	})

	return certPEM, keyPEM, nil
}

func newReconcilerTestConfig(t *testing.T) config.ReconcilerConfig {
	return config.ReconcilerConfig{
		ResourceDirectory:       t.TempDir(),
		Platform:                config.PlatformKubernetes,
		DefaultProfile:          "",
		MaxConcurrentReconciles: 1,
	}
}
