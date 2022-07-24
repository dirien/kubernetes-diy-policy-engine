package cmd

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"log"
	"net/http"
	"os"
	"strings"
)

var rootCmd = &cobra.Command{
	Use:   "mutating-webhook",
	Short: "Kubernetes DIY mutating webhook",
	Long: `Kubernetes DIY mutating webhook.
Example:
mutating-webhook --port <port> --tls-cert <tls_cert> --tls-key <tls_key>`,
	RunE: runMutatingWebhook,
}

var logger = log.New(os.Stdout, "", log.LstdFlags)

func init() {
	rootCmd.Flags().String("tls-cert", "", "TLS Certificate")
	rootCmd.Flags().String("tls-key", "", "Key for TLS Certificate")
	rootCmd.Flags().Int("port", 8443, "Port to listen on")
}

func runMutatingWebhook(cmd *cobra.Command, _ []string) error {
	tlsCert, err := cmd.Flags().GetString("tls-cert")
	if err != nil {
		return err
	}
	if len(tlsCert) == 0 {
		return errors.New("please provide a valid TLS Certificate")
	}
	tlsKey, err := cmd.Flags().GetString("tls-key")
	if err != nil {
		return err
	}
	if len(tlsKey) == 0 {
		return errors.New("please provide a valid TLS Key")
	}
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		return err
	}
	err = runMutatingWebhookServer(tlsCert, tlsKey, port)
	if err != nil {
		return err
	}
	return nil
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

const (
	ContentTypeJSON = "application/json"
	ContentTypeKey  = "Content-Type"
)

func admissionReviewFromRequest(r *http.Request, deserializer runtime.Decoder) (*admissionv1.AdmissionReview, error) {
	if r.Header.Get(ContentTypeKey) != ContentTypeJSON {
		return nil, fmt.Errorf("contentType=%s, expected %s", r.Header.Get(ContentTypeKey), ContentTypeJSON)
	}

	var body []byte
	if r.Body != nil {
		requestData, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		body = requestData
	}

	// Decode the request body into
	admissionReviewRequest := &admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, admissionReviewRequest); err != nil {
		return nil, err
	}

	return admissionReviewRequest, nil
}

func writeErrorResponse(w http.ResponseWriter, err error) {
	logger.Printf(err.Error())
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(err.Error()))
}

func mutate(w http.ResponseWriter, r *http.Request) {
	log.Printf("mutate request")

	// https://godoc.org/k8s.io/apimachinery/pkg/runtime#Scheme
	scheme := runtime.NewScheme()

	// https://godoc.org/k8s.io/apimachinery/pkg/runtime/serializer#CodecFactory
	codecFactory := serializer.NewCodecFactory(scheme)
	deserializer := codecFactory.UniversalDeserializer()

	admissionReviewRequest, err := admissionReviewFromRequest(r, deserializer)
	if err != nil {
		writeErrorResponse(w, errors.New(fmt.Sprintf("can't retrieve admission review from request: %v", err)))
		return
	}

	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if admissionReviewRequest.Request.Resource != podResource {
		writeErrorResponse(w, errors.New(fmt.Sprintf("review request is not from kind pod, got %s", admissionReviewRequest.Request.Resource.Resource)))
		return
	}

	rawRequest := admissionReviewRequest.Request.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := deserializer.Decode(rawRequest, nil, &pod); err != nil {
		writeErrorResponse(w, errors.New(fmt.Sprintf("can't decode raw pod definition: %v", err)))
		return
	}

	admissionResponse := &admissionv1.AdmissionResponse{}
	var patch string
	patchType := admissionv1.PatchTypeJSONPatch

	for i := 0; i < len(pod.Spec.Containers); i++ {
		if pod.Spec.Containers[i].Resources.Limits == nil {
			patch = fmt.Sprintf(`{"op": "add", "path": "/spec/containers/%d/resources/limits", "value": {"cpu": "100m", "memory": "100Mi"}}, %s`, i, patch)
			patch = strings.TrimSpace(patch)
		}
	}

	if len(patch) > 0 {
		patch = strings.TrimRight(patch, ",")
		patch = fmt.Sprintf(`[%s]`, patch)
	}

	admissionResponse.Allowed = true
	if patch != "" {
		admissionResponse.PatchType = &patchType
		admissionResponse.Patch = []byte(patch)
	}

	var admissionReviewResponse admissionv1.AdmissionReview
	admissionReviewResponse.Response = admissionResponse
	admissionReviewResponse.SetGroupVersionKind(admissionReviewRequest.GroupVersionKind())
	admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID

	resp, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		writeErrorResponse(w, errors.New(fmt.Sprintf("not possible marshall response: %v", err)))
		return
	}

	w.Header().Set(ContentTypeKey, ContentTypeJSON)
	w.Write(resp)
}

func runMutatingWebhookServer(tlsCert, tlsKey string, port int) error {
	logger.Print("Starting DIY mutating webhook server")
	cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
	if err != nil {
		logger.Fatal(err)
	}

	http.HandleFunc("/mutate", mutate)
	server := http.Server{
		Addr: fmt.Sprintf(":%d", port),
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
		ErrorLog: logger,
	}

	if err := server.ListenAndServeTLS("", ""); err != nil {
		logger.Panic(err)
	}
	return nil
}
