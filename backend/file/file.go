package file

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"

	"github.com/fsnotify/fsnotify"
	"github.com/goccy/go-yaml"
	"github.com/jacobweinstock/proxydhcp/proxy"
)

type Config struct {
	IP       net.IP
	FilePath string
	Data     *Data
}

type Data struct {
	Binaries binaries      `yaml:"binaries"`
	Globals  globals       `yaml:"globals"`
	HwAddrs  map[string]hw `yaml:"hwAddrs"`
}

type binaries struct {
	X86PC      string `yaml:"x86PC"`
	EfiIA32    string `yaml:"efiIA32"`
	Efix8664   string `yaml:"efix8664"`
	EfiBC      string `yaml:"efiBC"`
	EFIAARCH64 string `yaml:"efiAARCH64"`
}

type globals struct {
	Tftp string `yaml:"tftp"`
	URL  string `yaml:"url"`
}

type hw struct {
	AllowPxe bool   `yaml:"allowPxe"`
	Uefi     bool   `yaml:"uefi"`
	Arch     string `yaml:"arch"`
	Ipxe     ipxe   `yaml:"ipxe"`
}

type ipxe struct {
	Tftp string `yaml:"tftp"` // TODO yaml validation. cant be longer than 64 chars; https://github.com/danderson/netboot/blob/fc2840fa7b05c2f2447452e0dcc35a5a76f6acfa/dhcp4/packet.go#L211
	URL  string `yaml:"url"`  // TODO yaml validation. this cant be longer than 128 chars; https://github.com/danderson/netboot/blob/fc2840fa7b05c2f2447452e0dcc35a5a76f6acfa/dhcp4/packet.go#L223
}

func (c *Config) Locate(_ context.Context, mac net.HardwareAddr, uc proxy.UserClass, arch proxy.Architecture) (string, string, error) {
	/*yamlFile, err := readFile(c.FilePath)
	if err != nil {
		return "", "", err
	}
	var d Data
	if err := yaml.Unmarshal([]byte(yamlFile), &d); err != nil {
		return "", "", err
	}*/

	hw, ok := c.Data.HwAddrs[mac.String()]
	if !ok {
		return "", "", fmt.Errorf("could not find record for %v", mac)
	}
	if !hw.AllowPxe {
		return "", "", fmt.Errorf("hwAddr %v does not allow pxe", mac)
	}
	var bootfilename, bootservername string
	if hw.Ipxe.Tftp == "" {
		bootservername = c.Data.Globals.Tftp
	} else {
		bootservername = hw.Ipxe.Tftp
	}

	switch arch {
	case proxy.X86PC:
		bootfilename = c.Data.Binaries.X86PC
		// bootservername = hw.Ipxe.Tftp
	case proxy.EFIIA32:
		bootfilename = c.Data.Binaries.EfiIA32
		// bootservername = hw.Ipxe.Tftp
	case proxy.EFIx8664:
		bootfilename = c.Data.Binaries.Efix8664
		// bootservername = hw.Ipxe.Tftp
	case proxy.EFIBC:
		bootfilename = c.Data.Binaries.EfiBC
		// bootservername = hw.Ipxe.Tftp
	case proxy.EFIAARCH64:
		bootfilename = c.Data.Binaries.EFIAARCH64
	case proxy.EFIAARCH64Http:
		if hw.Ipxe.URL == "" {
			bootfilename = c.Data.Globals.URL
		} else {
			bootfilename = hw.Ipxe.URL
		}
	default:
		bootfilename = "/unsupported"
		bootservername = ""
	}
	switch uc {
	case proxy.IPXE, proxy.Tinkerbell:
		if hw.Ipxe.URL == "" {
			bootfilename = c.Data.Globals.URL
		} else {
			bootfilename = hw.Ipxe.URL
		}
		bootservername = ""
	default:
	}

	return bootfilename, bootservername, nil
}

func readFile(filePath string) ([]byte, error) {
	yamlFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return yamlFile, nil
}

func (c *Config) FirstLoad() error {
	yamlFile, err := readFile(c.FilePath)
	if err != nil {
		return err
	}
	var d Data
	if err := yaml.Unmarshal(yamlFile, &d); err != nil {
		return err
	}
	c.Data = &d
	return nil
}

func (c *Config) Watcher() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			fmt.Println("starting")
			select {
			case event := <-watcher.Events:
				/*if !ok {
					fmt.Println("1. not ok")
					return
				}*/
				fmt.Println("event:", event)
				switch event.Op {
				case fsnotify.Write:
					fmt.Println("modified file:", event.Name)
					if err := c.Load(); err != nil {
						fmt.Println("2. ", err)
					}
				case fsnotify.Create:
					fmt.Println("created file:", event.Name)
				case fsnotify.Remove:
					fmt.Println("removed file:", event.Name)
				case fsnotify.Rename:
					fmt.Println("renamed file:", event.Name)
					if err := c.Load(); err != nil {
						fmt.Println("2. ", err)
					}
				case fsnotify.Chmod:
					fmt.Println("chmod file:", event.Name)
					if err := c.Load(); err != nil {
						fmt.Println("2. ", err)
					}
				default:
					fmt.Println("unknown event:", event)
				}

				/*if event.Op&fsnotify.Write == fsnotify.Write {
					fmt.Println("modified file:", event.Name)
					fmt.Println("reading and updating in memory")

				}*/
			case err := <-watcher.Errors:
				/*if !ok {
					fmt.Println("4. error")
					return
				}*/
				fmt.Println("error:", err)
			}
			fmt.Println("back to the start")
		}
	}()

	fmt.Println("added file to watch")
	err = watcher.Add(c.FilePath)
	if err != nil {
		log.Fatal(err)
	}
	<-done
	fmt.Println("done")
}

func (c *Config) Load() error {
	fmt.Println("loading")
	yamlFile, err := readFile(c.FilePath)
	if err != nil {
		fmt.Println("2. ", err)
		return err
	}
	var d Data
	if err := yaml.Unmarshal(yamlFile, &d); err != nil {
		fmt.Println("3.", err)
		return err
	}
	c.Data = &d
	fmt.Println("loaded")
	return nil
}
