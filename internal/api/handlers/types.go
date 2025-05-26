package handlers

// DownloadRequest 下载请求结构体
type DownloadRequest struct {
	Url            string `json:"url" binding:"required"`
	Output         string `json:"output" binding:"required"`
	C              int    `json:"c"`
	CustomFileName string `json:"customFileName"`
	DeleteTs       bool   `json:"deleteTs"`
	ConvertToMp4   bool   `json:"convertToMp4"`
}

// CreateFolderRequest 创建文件夹请求
type CreateFolderRequest struct {
	Path string `json:"path" binding:"required"`
	Name string `json:"name" binding:"required"`
}

// Response API通用响应结构体
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// TaskInfo 用于API返回的任务信息
type TaskInfo struct {
	ID       string  `json:"id"`       // 任务ID
	URL      string  `json:"url"`      // 下载链接
	Output   string  `json:"output"`   // 输出路径
	C        int     `json:"c"`        // 线程数
	Progress int     `json:"progress"` // 下载进度 (0-100)
	Status   string  `json:"status"`   // 任务状态
	Message  string  `json:"message"`  // 状态信息
	Created  int64   `json:"created"`  // 创建时间
	FileName string  `json:"fileName"` // 输出文件名
	Speed    float64 `json:"speed"`    // 下载速度（字节/秒）
}
