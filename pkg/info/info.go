package info

// 节点角色的枚举
type NodeRole int

const (
	Master NodeRole = iota
	Worker
)

type NodeInfo struct {
	NodeId       string
	NodeName     string
	Architecture string   // 节点架构 (如 "x86", "arm", "x86+arm"...)
	Role         NodeRole // 节点角色 (Master / Worker 等)
	CpuNum       int      // 节点可用 CPU 个数
	GpuNum       int      // 节点可用 GPU 个数
	Memory       float64  // 节点可用内存大小
	OutboundIp   string
	Port         int
	Address      string
	Addresses    *string

	Resource      *Resource    // 节点空闲可分配资源
	ContainerList []*Container // 节点上已部署的容器列表
	ImageCache    []*Image     // 节点上缓存的镜像列表
}

type Resource struct {
	CpuNum int     // 节点空闲可分配 CPU 个数
	GpuNum int     // 节点空闲可分配 GPU 个数
	Memory float64 // 节点空闲可分配内存大小
}

type Container struct {
	Id     string
	Volume string  // 挂载（sourcepath:target）
	Thot   float64 // 容器热启动时间（近似为常数）
}

type Image struct {
	Name  string  // 镜像名（registry/user/name:tag）
	Tcold float64 // 容器冷启动时间（近似为常数）
}

type Taskinfo struct {
	TaskId      string
	ImageNameID string               // 本次任务需要的镜像名
	ResourceReq *ResourceRequirement // 资源需求
}

type ResourceRequirement struct {
	RequiresGpu  bool
	CPUReq       int32
	MemReq       float64
	GPUReq       int32
	Architecture string  // 任务需要的节点架构
	ExpectedTime float64 // 期望完成时间(毫秒或秒)
}
