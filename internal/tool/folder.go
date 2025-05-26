package tool

import (
	"fmt"
	"m3u8-go/internal/config"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FolderInfo 文件夹信息
type FolderInfo struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	Children []FolderInfo `json:"children,omitempty"`
}

// FolderListResponse 文件夹列表响应
type FolderListResponse struct {
	RootPath string       `json:"rootPath"`
	Folders  []FolderInfo `json:"folders"`
}

// GetFolderList 获取指定目录下的文件夹列表
func GetFolderList(targetPath string) (*FolderListResponse, error) {
	// 如果没有指定路径，使用默认下载路径作为根目录
	if targetPath == "" {
		settings := config.Get()
		targetPath = settings.DefaultOutputPath

		// 确保默认目录存在
		if err := os.MkdirAll(targetPath, 0755); err != nil {
			return nil, fmt.Errorf("无法创建默认下载目录: %w", err)
		}
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("无法获取绝对路径: %w", err)
	}

	// 检查目录是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("目录不存在: %s", absPath)
	}

	// 检查目录是否可读
	if !isReadableDir(absPath) {
		return nil, fmt.Errorf("目录无法读取: %s", absPath)
	}

	// 读取目录内容，增加递归深度到5层，提供更完整的文件树
	folders, err := scanFolders(absPath, 5)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	response := &FolderListResponse{
		RootPath: absPath,
		Folders:  folders,
	}

	return response, nil
}

// scanFolders 扫描文件夹，支持递归
func scanFolders(dirPath string, maxDepth int) ([]FolderInfo, error) {
	if maxDepth <= 0 {
		return []FolderInfo{}, nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return []FolderInfo{}, nil
	}

	var folders []FolderInfo

	for _, entry := range entries {
		// 跳过隐藏文件夹和特殊文件夹
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		if !entry.IsDir() {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())

		// 检查是否可读
		if !isReadableDir(fullPath) {
			continue
		}

		folderInfo := FolderInfo{
			Name: entry.Name(),
			Path: fullPath,
		}

		// 递归获取子文件夹（减少深度）
		if maxDepth > 1 {
			children, err := scanFolders(fullPath, maxDepth-1)
			if err == nil && len(children) > 0 {
				folderInfo.Children = children
			}
		}

		folders = append(folders, folderInfo)
	}

	// 按名称排序
	sort.Slice(folders, func(i, j int) bool {
		return folders[i].Name < folders[j].Name
	})

	return folders, nil
}

// isReadableDir 检查目录是否可读
func isReadableDir(dirPath string) bool {
	_, err := os.ReadDir(dirPath)
	return err == nil
}

// CreateFolder 创建新文件夹
func CreateFolder(parentPath, folderName string) error {
	// 清理文件夹名称，移除非法字符
	cleanName := cleanFolderName(folderName)
	if cleanName == "" {
		return fmt.Errorf("文件夹名称无效")
	}

	// 构建完整路径
	fullPath := filepath.Join(parentPath, cleanName)

	// 检查父目录是否存在
	if _, err := os.Stat(parentPath); os.IsNotExist(err) {
		return fmt.Errorf("父目录不存在: %s", parentPath)
	}

	// 检查目标文件夹是否已存在
	if _, err := os.Stat(fullPath); !os.IsNotExist(err) {
		return fmt.Errorf("文件夹已存在: %s", cleanName)
	}

	// 创建文件夹
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return fmt.Errorf("创建文件夹失败: %w", err)
	}

	return nil
}

// cleanFolderName 清理文件夹名称，移除或替换非法字符
func cleanFolderName(name string) string {
	// 移除前后空格
	name = strings.TrimSpace(name)

	// 定义非法字符（Windows和Linux通用）
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}

	// 替换非法字符为下划线
	for _, char := range invalidChars {
		name = strings.ReplaceAll(name, char, "_")
	}

	// 移除连续的下划线
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}

	// 移除开头和结尾的下划线和点号
	name = strings.Trim(name, "_.")

	// 检查长度（大多数文件系统支持255字符的文件名）
	if len(name) > 200 {
		name = name[:200]
	}

	return name
}

// ValidatePath 验证路径是否安全且可访问
func ValidatePath(path string) error {
	// 获取绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("无效路径: %w", err)
	}

	// 检查路径是否存在
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("路径不存在: %s", absPath)
	}
	if err != nil {
		return fmt.Errorf("无法访问路径: %w", err)
	}

	// 检查是否为目录
	if !info.IsDir() {
		return fmt.Errorf("路径不是目录: %s", absPath)
	}

	// 检查是否可写
	testFile := filepath.Join(absPath, ".test_write_permission")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("目录无写入权限: %s", absPath)
	}
	// 清理测试文件
	os.Remove(testFile)

	return nil
}
