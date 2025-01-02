package main

import (
	"fmt"

	"github.com/qiannian116/tide-scheduler/pkg/cluster"
	"github.com/qiannian116/tide-scheduler/pkg/info"
	"github.com/qiannian116/tide-scheduler/pkg/scheduler"
)

func main() {
	// 1) 初始化集群管理器
	cm := cluster.NewClusterManager()

	// 2) 构造节点示例
	node1 := &info.NodeInfo{
		NodeId:       "node-1",
		NodeName:     "worker-1",
		Architecture: "x86",
		Role:         info.Worker,
		CpuNum:       8,
		GpuNum:       1,
		Memory:       16.0,
		OutboundIp:   "192.168.1.101",
		Port:         8080,
		Address:      "192.168.1.101:8080",
		Resource: &info.Resource{
			CpuNum: 8,
			GpuNum: 1,
			Memory: 16.0,
		},
		ContainerList: []*info.Container{
			{Id: "c1", Volume: "/data:/data", Thot: 200.0},
		},
		ImageCache: []*info.Image{
			{Name: "registry.io/user/resnet:latest", Tcold: 800},
		},
	}

	node2 := &info.NodeInfo{
		NodeId:       "node-2",
		NodeName:     "worker-2",
		Architecture: "arm",
		Role:         info.Worker,
		CpuNum:       4,
		GpuNum:       2,
		Memory:       8.0,
		OutboundIp:   "192.168.1.102",
		Port:         9090,
		Address:      "192.168.1.102:9090",
		Resource: &info.Resource{
			CpuNum: 4,
			GpuNum: 2,
			Memory: 8.0,
		},
		ContainerList: []*info.Container{},
		ImageCache: []*info.Image{
			{Name: "registry.io/user/bert:latest", Tcold: 1200},
		},
	}

	// 3) 将节点加入集群
	cm.AddOrUpdateNode(node1)
	cm.AddOrUpdateNode(node2)

	// 4) 创建调度器
	scheduler := scheduler.NewScheduler(cm)

	// 5) 构造一个任务
	taskA := &info.Taskinfo{
		TaskId:      "task-A",
		ImageNameID: "registry.io/user/resnet:latest",
		ResourceReq: &info.ResourceRequirement{
			RequiresGpu:  false,
			CPUReq:       2,
			MemReq:       2.0,
			GPUReq:       0,
			Architecture: "x86",
			ExpectedTime: 1000.0, // 希望在1000ms内启动完成 (含容器启动)
		},
	}

	// 6) 调度任务
	containerID := scheduler.ScheduleTask(taskA)
	if containerID == "" {
		fmt.Println("Task scheduling failed.")
	} else {
		fmt.Println("Task scheduled successfully, containerID =", containerID)
	}
}
