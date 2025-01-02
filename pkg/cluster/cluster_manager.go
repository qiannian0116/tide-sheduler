package cluster

import "github.com/qiannian116/tide-scheduler/pkg/info"

type ClusterManager struct {
	// nodeID -> NodeInfo
	Nodes map[string]*info.NodeInfo
}

// NewClusterManager 创建一个空的集群管理器
func NewClusterManager() *ClusterManager {
	return &ClusterManager{
		Nodes: make(map[string]*info.NodeInfo),
	}
}

// AddOrUpdateNode 添加或更新节点信息
func (cm *ClusterManager) AddOrUpdateNode(node *info.NodeInfo) {
	cm.Nodes[node.NodeId] = node
}

// GetNode 根据节点ID获取对应的 NodeInfo
func (cm *ClusterManager) GetNode(nodeID string) *info.NodeInfo {
	return cm.Nodes[nodeID]
}

// ListAllNodes 获取所有节点的列表
func (cm *ClusterManager) ListAllNodes() []*info.NodeInfo {
	result := make([]*info.NodeInfo, 0, len(cm.Nodes))
	for _, node := range cm.Nodes {
		result = append(result, node)
	}
	return result
}

// UpdateNodeResource 简单演示：扣减节点的空闲资源
func (cm *ClusterManager) UpdateNodeResource(nodeID string, usedCPU int, usedMem float64, usedGPU int) {
	node := cm.GetNode(nodeID)
	if node == nil || node.Resource == nil {
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
