package main

import (
	"fmt"
	"github.com/kiquetal/kubernetes-programming-with-go-book.git/internal/podcreator"
)

func main() {
	err := podcreator.CreatePod()
	if err != nil {
		fmt.Printf("Error Invoking CreatePod: %v\n", err)
	} else {
		fmt.Println("Pod created successfully")
	}
}
