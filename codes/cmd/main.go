package main

import (
	"bytes"
	"fmt"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"net/http"
)

func createPod() error {
	pod := createPodObject()
	serializer := getJSONSerializer()
	postBody, err := serializePodObject(serialzier, pod)
	if err != nil {
		return err
	}
	reqCreate, err := buildPostRequest(postBody)
	if err != nil {
		return err
	}
	client := &http.Client{}
	resp, err := client.Do(reqCreate)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 300 {
		createdPod, err := deserializePodBody(serializer, body)
		if err != nil {
			return err
		}
		json, err := json.MarshalIndent(createdPod, "", "")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", json)
	} else {
		status, err := deserializeStatusBody(serializer, body)
		if err != nil {
			return err
		}
		json, err := json.MarshalIndent(status, "", "")
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", json)
	}
	return nil
}

func getJSONSerializer() runtime.Serializer {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(schema.GroupVersion{
		Version: "v1",
		Group:   "",
	},
		&corev1.Pod{},
		&metav1.Status{},
	)
	serializer := json.NewSerializerWithOptions(
		json.SimpleMetaFactory{},
		nil,
		scheme,
		json.SerializerOptions{},
	)
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

func serializePodObject(
	serializer runtime.Serializer,
	pod *corev1.Pod,
) (io.Reader, error) {
	var buf bytes.Buffer
	err := serializer.Encode(pod, &buf)
	if err != nil {
		return nil, err
	}
	return &buf, nil
}
