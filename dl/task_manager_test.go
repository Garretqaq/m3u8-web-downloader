package dl

import (
	"testing"
)

func TestGenerateUniqueFileName(t *testing.T) {
	// 初始化任务管理器
	manager := GetTaskManager()
	
	// 测试基本文件名生成
	folder := "/tmp/test"
	fileName := "video.ts"
	
	// 第一次生成文件名，应该是原始文件名
	name1 := manager.GenerateUniqueFileName(folder, fileName)
	if name1 != fileName {
		t.Errorf("首次生成的文件名应该是原始文件名，预期: %s, 实际: %s", fileName, name1)
	}
	
	// 第二次生成文件名，应该是带下划线和序号的文件名
	name2 := manager.GenerateUniqueFileName(folder, fileName)
	expected := "video_1.ts"
	if name2 != expected {
		t.Errorf("第二次生成的文件名应该是序号1, 预期: %s, 实际: %s", expected, name2)
	}
	
	// 第三次生成文件名，序号应该是2
	name3 := manager.GenerateUniqueFileName(folder, fileName)
	expected = "video_2.ts"
	if name3 != expected {
		t.Errorf("第三次生成的文件名应该是序号2, 预期: %s, 实际: %s", expected, name3)
	}
	
	// 测试不同文件夹下的同名文件
	anotherFolder := "/tmp/another"
	nameInAnotherFolder := manager.GenerateUniqueFileName(anotherFolder, fileName)
	if nameInAnotherFolder != fileName {
		t.Errorf("不同文件夹下应该可以使用相同的基本文件名, 预期: %s, 实际: %s", fileName, nameInAnotherFolder)
	}
	
	// 测试CheckFileNameExists方法
	if !manager.CheckFileNameExists(folder, name1) {
		t.Errorf("文件名 %s 应该被标记为已占用", name1)
	}
	
	if !manager.CheckFileNameExists(folder, name2) {
		t.Errorf("文件名 %s 应该被标记为已占用", name2)
	}
	
	if !manager.CheckFileNameExists(folder, name3) {
		t.Errorf("文件名 %s 应该被标记为已占用", name3)
	}
	
	if !manager.CheckFileNameExists(anotherFolder, nameInAnotherFolder) {
		t.Errorf("文件名 %s 应该被标记为已占用", nameInAnotherFolder)
	}
	
	// 测试未占用的文件名
	unusedName := "unused.ts"
	if manager.CheckFileNameExists(folder, unusedName) {
		t.Errorf("文件名 %s 不应该被标记为已占用", unusedName)
	}
}

func TestTaskCleanup(t *testing.T) {
	// 初始化任务管理器
	manager := GetTaskManager()
	
	// 创建下载任务
	folder := "/tmp/cleanup_test"
	fileName := "cleanup.ts"
	
	// 标记文件名为已占用
	finalFileName := manager.GenerateUniqueFileName(folder, fileName)
	
	// 创建一个模拟的下载器实例
	task := &Downloader{
		ID:       "test_id",
		folder:   folder,
		FileName: finalFileName,
	}
	
	// 添加任务到管理器
	manager.AddTask(task)
	
	// 验证文件名被占用
	if !manager.CheckFileNameExists(folder, finalFileName) {
		t.Errorf("文件名 %s 应该被标记为已占用", finalFileName)
	}
	
	// 删除任务
	manager.DeleteTask("test_id")
	
	// 验证文件名占用被释放
	if manager.CheckFileNameExists(folder, finalFileName) {
		t.Errorf("文件名 %s 应该在任务删除后被释放", finalFileName)
	}
} 