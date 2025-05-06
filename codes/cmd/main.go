package main

import (
	"bytes"
	json_ "encoding/json"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"net/http"
)

func main() {
	err := createPod()
	if err != nil {
		fmt.Printf("Error Invoking createdPod : %v\n", err)
	} else {
		fmt.Println("Pod created successfully")
	}
}

func createPod() error {
	pod := createPodObject()
	serializer := getJSONSerializer()
	postBody, err := serializePodObject(serializer, pod)
	if err != nil {
		return err
	}
	reqCreate, err := buildPostRequest(postBody)
	if err != nil {
		return err
	}
	client := &http.Client{}
	resp, err := client.Do(reqCreate)

	if err != nil {
		fmt.Printf("Error creating pod: %v\n", err)
		return err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 300 {
		createdPod, err := deserializePodBody(serializer, body)
		if err != nil {
			return err
		}
		rJson, err := json_.MarshalIndent(createdPod, "", "")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", rJson)
	} else {
		status, err := deserializeStatusBody(serializer, body)
		if err != nil {
			return err
		}
		jsonRes, err := json_.MarshalIndent(status, "", "")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", jsonRes)
	}
	return nil
}

func deserializePodBody(serializer runtime.Serializer, body []byte) (*metav1.Status, error) {

	var status metav1.Status
	_, _, err := serializer.Decode(body, nil, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}
func deserializeStatusBody(serializer runtime.Serializer, body []byte) (*corev1.Pod, error) {
	var pod corev1.Pod
	_, _, err := serializer.Decode(body, nil, &pod)
	if err != nil {
		return nil, err
	}
	return &pod, nil
}

func buildPostRequest(body io.Reader) (*http.Request, error) {
	reqCreate, err := http.NewRequest("POST", "http://127.0.0.1:8080/api/v1/namespaces/default/pods", body)

	if err != nil {
		return nil, err
	}
	reqCreate.Header.Set("Accept", "application/json")

	return reqCreate, nil

}

func getJSONSerializer() runtime.Serializer {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(schema.GroupVersion{
		Version: "v1",
		Group:   "",
	}, &corev1.Pod{}, &metav1.Status{})
	serializer := json.NewSerializerWithOptions(json.SimpleMetaFactory{}, nil, scheme, json.SerializerOptions{})
	return serializer
}

func createPodObject() *corev1.Pod {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "runtime",
					Image: "nginx",
				},
			},
		},
	}
	pod.SetName("my-pod")
	pod.SetLabels(map[string]string{
		"app.kubernetes.io/component": "my-component",
		"app.kubernetes.io/name":      "my-app",
	})
	return &pod
}

func serializePodObject(serializer runtime.Serializer, pod *corev1.Pod) (io.Reader, error) {
	var buf bytes.Buffer
	err := serializer.Encode(pod, &buf)
	if err != nil {
		return nil, err
	}
	return &buf, nil
}
