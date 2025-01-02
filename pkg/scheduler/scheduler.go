package scheduler

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/qiannian116/tide-scheduler/pkg/cluster"
	"github.com/qiannian116/tide-scheduler/pkg/info"
)

// Scheduler 实现调度逻辑
type Scheduler struct {
	cm *cluster.ClusterManager
}

// NewScheduler 创建一个调度器
func NewScheduler(cm *cluster.ClusterManager) *Scheduler {
	return &Scheduler{
		cm: cm,
	}
}

// ScheduleTask 调度任务，返回容器ID 或 空字符串表示调度失败
func (s *Scheduler) ScheduleTask(task *info.Taskinfo) string {
	// 1) 从集群中筛选可用节点
	candidateNodes := s.filterCandidateNodes(task)
	if len(candidateNodes) == 0 {
		fmt.Printf("[Scheduler] No candidate nodes found for task %s.\n", task.TaskId)
		return ""
	}

	// 2) 在候选节点中选择最佳节点
	//    这里仅做示例：我们简单从头到尾选出 "第一个" 能满足 ExpectedTime 的节点。
	//    在实际中，可以对节点进行打分（CPU空闲率/内存空闲量等）。
	var chosenNode *info.NodeInfo
	var chosenContainerID string
	for _, node := range candidateNodes {
		// 判断是热启动还是冷启动
		isHot := s.nodeHasImage(node, task.ImageNameID)
		containerTime := s.estimateContainerTime(node, task, isHot)
		// 判断 totalTime 是否满足 ExpectedTime
		if containerTime <= task.ResourceReq.ExpectedTime {
			// 选中该节点
			chosenNode = node
			// 创建容器(模拟)
			chosenContainerID = s.createContainerOnNode(node, task, isHot)
			break
		}
	}

	if chosenNode == nil {
		// 如果没有找到满足期望时间的节点, 可以做 "尽力而为" 或直接返回失败
		fmt.Printf("[Scheduler] All candidate nodes exceed expected time for task %s.\n", task.TaskId)
		// 这里演示“尽力而为”：选第一个可用资源的节点
		// （如果您不想尽力而为，可以直接 return ""）
		for _, node := range candidateNodes {
			// 只要能分配资源就行，不管时间
			if s.canSatisfyResource(node, task.ResourceReq) {
				isHot := s.nodeHasImage(node, task.ImageNameID)
				chosenContainerID = s.createContainerOnNode(node, task, isHot)
				chosenNode = node
				fmt.Printf("[Scheduler] (Fallback) Assign task %s to node %s ignoring ExpectedTime.\n",
					task.TaskId, node.NodeId)
				break
			}
		}
	}

	// 如果最终 chosenNode 仍为空，说明没办法分配
	if chosenNode == nil {
		fmt.Printf("[Scheduler] Cannot schedule task %s - no suitable node.\n", task.TaskId)
		return ""
	}
	return chosenContainerID
}

// ----------------- 辅助函数 -----------------

// filterCandidateNodes 筛选出可用于本任务的节点
func (s *Scheduler) filterCandidateNodes(task *info.Taskinfo) []*info.NodeInfo {
	var result []*info.NodeInfo
	for _, node := range s.cm.ListAllNodes() {
		// 1) 架构匹配
		if !strings.Contains(node.Architecture, task.ResourceReq.Architecture) {
			continue
		}
		// 2) 若需要GPU，则节点必须GPU>0
		if task.ResourceReq.RequiresGpu && node.Resource != nil && node.Resource.GpuNum <= 0 {
			continue
		}
		// 3) 节点资源是否 >= 需求
		if !s.canSatisfyResource(node, task.ResourceReq) {
			continue
		}
		result = append(result, node)
	}
	return result
}

// canSatisfyResource 判断节点空闲资源是否足以满足任务需求
func (s *Scheduler) canSatisfyResource(node *info.NodeInfo, req *info.ResourceRequirement) bool {
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

// nodeHasImage 判断节点上是否缓存了指定镜像(热启动)
func (s *Scheduler) nodeHasImage(node *info.NodeInfo, imageName string) bool {
	for _, img := range node.ImageCache {
		if img.Name == imageName {
			return true
		}
	}
	return false
}

// estimateContainerTime 根据热/冷启动估计容器启动时间
func (s *Scheduler) estimateContainerTime(node *info.NodeInfo, task *info.Taskinfo, isHot bool) float64 {
	var startupTime float64
	if isHot {
		// 取节点上任意一个与镜像同名的容器 Thot，
		// 或者直接从 node.ImageCache 里找 Tcold，但这个是 "镜像" 的冷启动时间
		// 这里演示：从 ContainerList 里找一下(如果没有，则用个默认值).
		thot := 300.0 // 假设一个默认热启动时间
		for _, c := range node.ContainerList {
			if c.Thot > 0 {
				thot = c.Thot
				break
			}
		}
		startupTime = thot
	} else {
		// 冷启动
		// 找节点上没有缓存，但可能会下载 => 从 node.ImageCache 找到与镜像同名的 Image.Tcold
		// 但既然是“冷启动”，该镜像应该不在 cache 中(或可以找镜像列表?).
		// 这里简化：直接去节点的全部 ImageCache 里看有没有 Task.ImageNameID 对应的 Tcold
		// 其实“冷启动”表示还没缓存，大概率不会在 node.ImageCache 中出现，但可以在全局镜像列表找到
		// 此处演示：先在 node.ImageCache 中找一下(万一有人手动拉了一半...). 否则给个默认1000
		tcold := 1000.0
		for _, img := range node.ImageCache {
			if img.Name == task.ImageNameID {
				tcold = img.Tcold
				break
			}
		}
		startupTime = tcold
	}

	// 这里不考虑“容器排队 + 任务执行时间”的复杂性，仅用 startupTime 近似对比
	// 若您想模拟“执行时间 + 启动时间”，可再加上(任务执行时长)。
	return startupTime
}

// createContainerOnNode 在节点上创建容器(模拟)并扣减资源
func (s *Scheduler) createContainerOnNode(node *info.NodeInfo, task *info.Taskinfo, isHot bool) string {
	// 1) 创建一个容器ID
	cid := uuid.NewString()[:8]
	// 2) 寻找一个可复用的 Container 结构 或 新建
	c := &info.Container{
		Id:     cid,
		Volume: "",  // 暂时空置
		Thot:   300, // 默认值, 可根据 isHot/其他逻辑设置
	}
	if !isHot {
		// 也可以设置 c.Thot = s.estimateContainerTime(node, task, false)
		// 但 Thot 和 Tcold 是不同的概念, 这里仅简化处理
		c.Thot = 1000
	}
	node.ContainerList = append(node.ContainerList, c)

	// 3) 扣减节点资源
	usedCPU := int(task.ResourceReq.CPUReq)
	usedMem := task.ResourceReq.MemReq
	usedGPU := int(task.ResourceReq.GPUReq)
	s.cm.UpdateNodeResource(node.NodeId, usedCPU, usedMem, usedGPU)

	fmt.Printf("[Scheduler] CreateContainerOnNode: node=%s, container=%s, isHot=%v\n",
		node.NodeId, cid, isHot)
	return cid
}
