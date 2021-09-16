package main

import (
	"fmt"
	"io/ioutil"

	"github.com/goccy/go-yaml"
)

type ipxe struct {
	Tftp string `yaml:"tftp"`
	Url  string `yaml:"url"`
}

type hw struct {
	AllowPxe bool   `yaml:"allowPxe"`
	Uefi     bool   `yaml:"uefi"`
	Arch     string `yaml:"arch"`
	Ipxe     ipxe   `yaml:"ipxe"`
}

type data struct {
	Binaries binaries      `yaml:"binaries"`
	Globals  globals       `yaml:"globals"`
	HwAddrs  map[string]hw `yaml:"hwAddrs"`
}
type binaries struct {
	X86PC    string `yaml:"x86PC"`
	EfiIA32  string `yaml:"efiIA32"`
	Efix8664 string `yaml:"efix8664"`
	EfiBC    string `yaml:"efiBC"`
}

type globals struct {
	Tftp string `yaml:"tftp"`
	URL  string `yaml:"url"`
}

func main() {
	fmt.Println("Hello, World!")
	yamlFile, err := readFile("examples/backend-file.yaml")
	if err != nil {
		panic(err)
	}
	var d data
	//d := make(map[string]hw)
	if err := yaml.Unmarshal([]byte(yamlFile), &d); err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", string(yamlFile))
	fmt.Printf("%+v\n", d)
}

func readFile(filePath string) ([]byte, error) {
	yamlFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return yamlFile, nil
}
