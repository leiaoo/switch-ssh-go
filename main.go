package main

import (
	"bufio"
	"fmt"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"io"
	"io/ioutil"
	"GoNetworkSSH/ssh"
	"os"
	"strings"
	"sync"
	"time"
)

type DeviceInfo struct {
	UserName string
	Password string
	Host     string
	Port     string
	Brand    string
}

type DevicesCommand struct {
	Huawei []string `mapstructure:"Huawei"`
	H3C    []string `mapstructure:"H3C"`
}

func main() {
	v := viper.New()
	v.SetConfigFile("conf/device_conf.yaml")
	err := v.ReadInConfig()
	if err != nil {
		zap.S().Errorf("设备命令配置文件出错：%s", err.Error())
		return
	}
	deviceCmds := &DevicesCommand{}
	if err = v.Unmarshal(deviceCmds); err != nil {
		zap.S().Errorf("设备命令出错：%s", err.Error())
		return
	}

	var wg sync.WaitGroup
	var device_list []DeviceInfo
	file, err := os.Open("device_list.txt")
	defer file.Close()
	if err != nil {
		fmt.Println(err)
	}
	read := bufio.NewReader(file)
	for {
		var device DeviceInfo
		line, err := read.ReadString('\n')
		if err == io.EOF {
			break
		}
		lineList := strings.Split(line, " ")
		device.Host = lineList[0]
		device.UserName = lineList[1]
		device.Password = lineList[2]
		device.Port = "22"
		device.Brand = strings.TrimRight(lineList[3], "\r\n")
		device_list = append(device_list, device)
	}
	for _, device := range device_list {
		wg.Add(1)
		var cmds []string
		switch device.Brand {
		case "huawei":
			cmds = deviceCmds.Huawei
		case "h3c":
			cmds = deviceCmds.H3C
		}
		go connectDevice(device.UserName, device.Password, device.Host, device.Port, device.Brand, cmds, &wg)
	}
	wg.Wait()
}

func connectDevice(user, pwd, ip, port, brand string, cmds []string, wg *sync.WaitGroup) {
	defer wg.Done()
	ipPort := fmt.Sprintf("%s:%s", ip, port)
	result, err := ssh.RunCommandsWithBrand(user, pwd, ipPort, brand, cmds...)
	if err != nil {
		zap.S().Errorf("RunCommands err:%s", err.Error())
		return
	}
	datestr := time.Now().Format("2006-01-02")
	dir_path := fmt.Sprintf("%s/%s/", brand, datestr)
	_, err = os.Stat(dir_path)
	if os.IsNotExist(err) {
		_ = os.MkdirAll(dir_path, os.ModePerm)
	}
	filename := fmt.Sprintf("%s/%s.txt", dir_path, ip)
	err = ioutil.WriteFile(filename, []byte(result), os.ModePerm)
	if err != nil {
		zap.S().Errorf("创建文件失败：%s", err.Error())
		return
	}
}
