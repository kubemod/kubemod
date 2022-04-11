package jsonpath_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/PaesslerAG/gval"

	"github.com/kubemod/kubemod/jsonpath"
)

func ExampleGet() {
	v := interface{}(nil)

	json.Unmarshal([]byte(`{
		"welcome":{
				"message":["Good Morning", "Hello World!"]
			}
		}`), &v)

	welcome, err := jsonpath.Get("$.welcome.message[1]", v)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(welcome)

	// Output:
	// Hello World!
}

func ExampleGet_wildcard() {
	v := interface{}(nil)

	json.Unmarshal([]byte(`{
		"welcome":{
				"message":["Good Morning", "Hello World!"]
			}
		}`), &v)

	welcome, err := jsonpath.Get("$.welcome.message[*]", v)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, value := range welcome.([]interface{}) {
		fmt.Printf("%v\n", value)
	}

	// Output:
	// Good Morning
	// Hello World!
}

func ExampleGet_filter() {
	v := interface{}(nil)

	json.Unmarshal([]byte(`[
		{"key":"a","value" : "I"},
		{"key":"b","value" : "II"},
		{"key":"c","value" : "III"}
		]`), &v)

	values, err := jsonpath.Get(`$[? @.key=="b"].value`, v)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, value := range values.([]interface{}) {
		fmt.Println(value)
	}

	// Output:
	// II
}

func Example_gval() {
	builder := gval.Full(jsonpath.PlaceholderExtension())

	path, err := builder.NewEvaluable("{#1: $..[?@.ping && @.speed > 100].name}")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	v := interface{}(nil)
	err = json.Unmarshal([]byte(`{
		"device 1":{
			"name": "fancy device",
			"ping": true,
			"speed": 200,
				"subdevice 1":{
					"ping" : true,
					"speed" : 99,
					"name" : "boring subdevice"
				},
				"subdevice 2":{
					"ping" : true,
					"speed" : 150,
					"name" : "fancy subdevice"
				},
				"not an device":{
					"name" : "ping me but I have no speed property",
					"ping" : true
				}
			},
		"fictive device":{
			"ping" : false,
			"speed" : 1000,
			"name" : "dream device"
			}
		}`), &v)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	devices, err := path(context.Background(), v)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for device, name := range devices.(map[string]interface{}) {
		fmt.Printf("%s -> %v\n", device, name)
	}

	// Unordered output:
	// device 1 -> fancy device
	// subdevice 2 -> fancy subdevice
}
