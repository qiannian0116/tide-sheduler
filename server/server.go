package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// ---------------------------(1) 数据结构与枚举---------------------------

// 节点角色枚举
type NodeRole int

const (
	Master NodeRole = iota
	Worker
)

// NodeInfo：节点信息
type NodeInfo struct {
	NodeId       string   `json:"node_id"`
	NodeName     string   `json:"node_name"`
	Architecture string   `json:"architecture"` // x86 / arm / x86+arm ...
	Role         NodeRole `json:"role"`
	CpuNum       int      `json:"cpu_num"`
	GpuNum       int      `json:"gpu_num"`
	Memory       float64  `json:"memory"`
	OutboundIp   string   `json:"outbound_ip"`
	Port         int      `json:"port"`
	Address      string   `json:"address"`
	Addresses    *string  `json:"addresses"`

	Resource      *Resource    `json:"resource"`
	ContainerList []*Container `json:"container_list"`
	ImageCache    []*Image     `json:"image_cache"`
}

// Resource：空闲资源
type Resource struct {
	CpuNum int     `json:"cpu_num"`
	GpuNum int     `json:"gpu_num"`
	Memory float64 `json:"memory"`
}

// Container：容器信息
type Container struct {
	Id     string  `json:"id"`
	Volume string  `json:"volume"`
	Thot   float64 `json:"thot"`
}

// Image：镜像信息
type Image struct {
	Name  string  `json:"name"`
	Tcold float64 `json:"tcold"`
}

// Taskinfo：任务
type Taskinfo struct {
	TaskId      string               `json:"task_id"`
	ImageNameID string               `json:"image_name_id"`
	ResourceReq *ResourceRequirement `json:"resource_req"`
}

// ResourceRequirement：任务资源需求
type ResourceRequirement struct {
	RequiresGpu  bool    `json:"requires_gpu"`
	CPUReq       int32   `json:"cpu_req"`
	MemReq       float64 `json:"mem_req"`
	GPUReq       int32   `json:"gpu_req"`
	Architecture string  `json:"architecture"`
	ExpectedTime float64 `json:"expected_time"`
}

// ---------------------------(2) 集群管理器---------------------------

type ClusterManager struct {
	// nodeID -> NodeInfo
	Nodes map[string]*NodeInfo
	// 为了线程安全，这里加一把锁
	mu sync.RWMutex
}

func NewClusterManager() *ClusterManager {
	return &ClusterManager{
		Nodes: make(map[string]*NodeInfo),
	}
}

// AddOrUpdateNode 添加或更新节点信息
func (cm *ClusterManager) AddOrUpdateNode(node *NodeInfo) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.Nodes[node.NodeId] = node
}

// ListAllNodes 获取所有节点
func (cm *ClusterManager) ListAllNodes() []*NodeInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var ret []*NodeInfo
	for _, n := range cm.Nodes {
		ret = append(ret, n)
	}
	return ret
}

// GetNode 根据节点ID获取
func (cm *ClusterManager) GetNode(nodeID string) *NodeInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.Nodes[nodeID]
}

// UpdateNodeResource 扣减节点空闲资源
func (cm *ClusterManager) UpdateNodeResource(nodeID string, usedCPU int, usedMem float64, usedGPU int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	node, ok := cm.Nodes[nodeID]
	if !ok || node.Resource == nil {
		return
	}
	node.Resource.CpuNum -= usedCPU
	node.Resource.Memory -= usedMem
	node.Resource.GpuNum -= usedGPU

	if node.Resource.CpuNum < 0 {
		node.Resource.CpuNum = 0
	}
	if node.Resource.Memory < 0 {
		node.Resource.Memory = 0
	}
	if node.Resource.GpuNum < 0 {
		node.Resource.GpuNum = 0
	}
}

// ---------------------------(3) 调度器---------------------------

// Scheduler：调度器
type Scheduler struct {
	cm *ClusterManager
}

func NewScheduler(cm *ClusterManager) *Scheduler {
	return &Scheduler{
		cm: cm,
	}
}

// ScheduleTask：调度任务，返回容器ID或空字符串
func (s *Scheduler) ScheduleTask(task *Taskinfo) string {
	candidateNodes := s.filterCandidateNodes(task)
	if len(candidateNodes) == 0 {
		fmt.Printf("[Scheduler] No candidate nodes for task %s\n", task.TaskId)
		return ""
	}

	var chosenNode *NodeInfo
	var chosenContainerID string

	// 简单策略：找到第一个满足 ExpectedTime 的节点
	for _, node := range candidateNodes {
		isHot := s.nodeHasImage(node, task.ImageNameID)
		startupTime := s.estimateContainerTime(node, task, isHot)
		if startupTime <= task.ResourceReq.ExpectedTime {
			chosenNode = node
			chosenContainerID = s.createContainerOnNode(node, task, isHot)
			break
		}
	}

	// 若没找到满足 ExpectedTime 的节点 -> fallback
	if chosenNode == nil {
		// 尽力而为：只要资源够，就选第一个
		for _, node := range candidateNodes {
			if s.canSatisfyResource(node, task.ResourceReq) {
				isHot := s.nodeHasImage(node, task.ImageNameID)
				chosenContainerID = s.createContainerOnNode(node, task, isHot)
				chosenNode = node
				fmt.Printf("[Scheduler] Fallback => assigned task %s to node %s ignoring ExpectedTime.\n",
					task.TaskId, node.NodeId)
				break
			}
		}
	}

	if chosenNode == nil {
		fmt.Printf("[Scheduler] No suitable node found for task %s, scheduling failed.\n", task.TaskId)
		return ""
	}
	return chosenContainerID
}

// ---------------------------(3.1) 调度器辅助函数---------------------------

// 筛选候选节点
func (s *Scheduler) filterCandidateNodes(task *Taskinfo) []*NodeInfo {
	var result []*NodeInfo
	nodes := s.cm.ListAllNodes()
	for _, node := range nodes {
		if !strings.Contains(node.Architecture, task.ResourceReq.Architecture) {
			continue
		}
		// GPU需求
		if task.ResourceReq.RequiresGpu && node.Resource != nil && node.Resource.GpuNum <= 0 {
			continue
		}
		// 判断资源
		if !s.canSatisfyResource(node, task.ResourceReq) {
			continue
		}
		result = append(result, node)
	}
	return result
}

// 判断节点是否满足资源需求
func (s *Scheduler) canSatisfyResource(node *NodeInfo, req *ResourceRequirement) bool {
	if node.Resource == nil {
		return false
	}
	if node.Resource.CpuNum < int(req.CPUReq) {
		return false
	}
	if node.Resource.Memory < req.MemReq {
		return false
	}
	if req.RequiresGpu && node.Resource.GpuNum < int(req.GPUReq) {
		return false
	}
	return true
}

// 判断节点是否拥有指定镜像（热启动）
func (s *Scheduler) nodeHasImage(node *NodeInfo, imageName string) bool {
	for _, img := range node.ImageCache {
		if img.Name == imageName {
			return true
		}
	}
	return false
}

// 估算容器启动时间
func (s *Scheduler) estimateContainerTime(node *NodeInfo, task *Taskinfo, isHot bool) float64 {
	if isHot {
		// 假设节点上任意一个容器的 Thot 作为参考；或者给个默认值
		if len(node.ContainerList) > 0 {
			return node.ContainerList[0].Thot
		}
		return 300.0
	}
	// 冷启动：若节点 ImageCache 没有，则默认 Tcold=1000
	// 也可进一步在全局查找镜像 Tcold
	for _, img := range node.ImageCache {
		if img.Name == task.ImageNameID {
			return img.Tcold
		}
	}
	return 1000.0
}

// 在节点上创建容器（模拟）并扣减资源
func (s *Scheduler) createContainerOnNode(node *NodeInfo, task *Taskinfo, isHot bool) string {
	cid := uuid.NewString()[:8]
	c := &Container{
		Id:     cid,
		Volume: "",
		Thot:   300.0,
	}
	if !isHot {
		c.Thot = 1000.0
	}

	node.ContainerList = append(node.ContainerList, c)

	// 扣减资源
	s.cm.UpdateNodeResource(node.NodeId,
		int(task.ResourceReq.CPUReq),
		task.ResourceReq.MemReq,
		int(task.ResourceReq.GPUReq),
	)

	fmt.Printf("[Scheduler] createContainerOnNode => node=%s, container=%s, isHot=%v\n",
		node.NodeId, cid, isHot)
	return cid
}

// ---------------------------(4) 服务端路由---------------------------

var (
	// 全局管理器 & 调度器
	globalCM  = NewClusterManager()
	scheduler = NewScheduler(globalCM)
)

// handleAddNode 处理添加或更新节点的请求
func handleAddNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var node NodeInfo
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	globalCM.AddOrUpdateNode(&node)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Node %s added/updated.\n", node.NodeId)
}

// handleScheduleTask 处理调度任务的请求
func handleScheduleTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var task Taskinfo
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	containerID := scheduler.ScheduleTask(&task)
	resp := map[string]string{
		"task_id":      task.TaskId,
		"container_id": containerID,
	}

	// 返回 JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ---------------------------(5) 启动 HTTP 服务---------------------------

func main() {
	http.HandleFunc("/addNode", handleAddNode)
	http.HandleFunc("/scheduleTask", handleScheduleTask)

	fmt.Println("Server listening on :8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
