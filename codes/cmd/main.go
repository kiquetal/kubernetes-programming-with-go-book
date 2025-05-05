package main

import(
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
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
  if err !=nil {
    return err
  }
  if resp.StatusCode < 300 {
    createdPod, err := deserializePodBody(serializer, body)
    if err !=nil {
      return err
    }
    json, err := json.MarshalIndent(createdPod, "","")
    if err!=nil {
	    return err
    }
    fmt.Printf("%s\n",json)
  } else {
    status, err := deserializeStatusBody(serializer,body)
    if err !=nil {
	    return err
    }
    json, err := json.MarshalIndent(status,"", "")
    if err !=nil {
	    return err
    }
    fmt.Printf("%s\n",json)
    }
    return nil
 }



