package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// 与服务端相同的数据结构，供客户端构造请求
type NodeRole int

const (
	Master NodeRole = iota
	Worker
)

type Resource struct {
	CpuNum int     `json:"cpu_num"`
	GpuNum int     `json:"gpu_num"`
	Memory float64 `json:"memory"`
}

type Container struct {
	Id     string  `json:"id"`
	Volume string  `json:"volume"`
	Thot   float64 `json:"thot"`
}

type Image struct {
	Name  string  `json:"name"`
	Tcold float64 `json:"tcold"`
}

type NodeInfo struct {
	NodeId        string       `json:"node_id"`
	NodeName      string       `json:"node_name"`
	Architecture  string       `json:"architecture"`
	Role          NodeRole     `json:"role"`
	CpuNum        int          `json:"cpu_num"`
	GpuNum        int          `json:"gpu_num"`
	Memory        float64      `json:"memory"`
	OutboundIp    string       `json:"outbound_ip"`
	Port          int          `json:"port"`
	Address       string       `json:"address"`
	Addresses     *string      `json:"addresses"`
	Resource      *Resource    `json:"resource"`
	ContainerList []*Container `json:"container_list"`
	ImageCache    []*Image     `json:"image_cache"`
}

type ResourceRequirement struct {
	RequiresGpu  bool    `json:"requires_gpu"`
	CPUReq       int32   `json:"cpu_req"`
	MemReq       float64 `json:"mem_req"`
	GPUReq       int32   `json:"gpu_req"`
	Architecture string  `json:"architecture"`
	ExpectedTime float64 `json:"expected_time"`
}

type Taskinfo struct {
	TaskId      string               `json:"task_id"`
	ImageNameID string               `json:"image_name_id"`
	ResourceReq *ResourceRequirement `json:"resource_req"`
}

func main() {
	// 目标服务地址
	serverURL := "http://localhost:8080"

	// 1) 添加节点
	node := NodeInfo{
		NodeId:       "node-1",
		NodeName:     "worker-1",
		Architecture: "x86",
		Role:         Worker,
		CpuNum:       8,
		GpuNum:       1,
		Memory:       16.0,
		OutboundIp:   "192.168.1.101",
		Port:         8080,
		Address:      "192.168.1.101:8080",
		Resource: &Resource{
			CpuNum: 8,
			GpuNum: 1,
			Memory: 16.0,
		},
		ContainerList: []*Container{},
		ImageCache: []*Image{
			{
				Name:  "registry.io/user/resnet:latest",
				Tcold: 800,
			},
		},
	}

	// 发起 POST /addNode
	if err := postJSON(serverURL+"/addNode", node); err != nil {
		log.Println("Add node error:", err)
	} else {
		log.Println("Add node success")
	}

	// 2) 构造任务
	task := Taskinfo{
		TaskId:      "task-A",
		ImageNameID: "registry.io/user/resnet:latest",
		ResourceReq: &ResourceRequirement{
			RequiresGpu:  false,
			CPUReq:       2,
			MemReq:       2.0,
			GPUReq:       0,
			Architecture: "x86",
			ExpectedTime: 500.0, // 期望在 500ms 内启动完成
		},
	}

	// 发起 POST /scheduleTask
	respData, err := postJSONAndGetResponse(serverURL+"/scheduleTask", task)
	if err != nil {
		log.Println("Schedule task error:", err)
		return
	}
	log.Println("Schedule task response:", respData)
}

// postJSON 仅发送 JSON，不关注响应体
func postJSON(url string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("status code %d, body=%s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// postJSONAndGetResponse 发送 JSON 并返回响应的字符串
func postJSONAndGetResponse(url string, data interface{}) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code %d, body=%s", resp.StatusCode, string(respBytes))
	}

	return string(respBytes), nil
}
