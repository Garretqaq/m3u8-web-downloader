package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Settings 定义应用配置项
// 与前端表单及 settings.json 字段保持一致
// 如果新增字段，请同时在此处、前端及默认文件中补充
//
// 说明:
//   1. 所有默认值仅在此处维护，避免在代码各处出现魔法值;
//   2. 通过 Get/Load/Save 统一读写配置文件。
//
// 在其它包内请通过 config.Get() 获取最新配置。
// 调用 Save() 成功后全局配置将被自动刷新。
//
// 注意: 本包仅依赖标准库，避免发生循环导入。

type Settings struct {
	DefaultOutputPath     string `json:"defaultOutputPath"`
	DefaultThreadCount    int    `json:"defaultThreadCount"`
	DefaultConvertToMp4   bool   `json:"defaultConvertToMp4"`
	DefaultDeleteTs       bool   `json:"defaultDeleteTs"`
	MaxConcurrentDownload int    `json:"maxConcurrentDownload"`
	DownloadSpeedLimit    int    `json:"downloadSpeedLimit"` // 单位: KB/s，0 表示不限速
}

var (
	settings     Settings
	once         sync.Once
	settingsPath = "./settings.json"
	mu           sync.RWMutex
)

// defaultSettings 定义应用的硬编码默认值，仅此处出现一次
var defaultSettings = Settings{
	DefaultOutputPath:     "/app/downloads",
	DefaultThreadCount:    25,
	DefaultConvertToMp4:   true,
	DefaultDeleteTs:       true,
	MaxConcurrentDownload: 3,
	DownloadSpeedLimit:    0,
}

// Load 读取配置文件，只在首次调用时真正执行磁盘 IO。
// 之后再次调用返回缓存值。
func Load() (Settings, error) {
	var err error
	once.Do(func() {
		settings, err = loadFromFile()
		if err != nil {
			// 若读取失败，回退到默认配置，避免应用直接奔溃
			settings = defaultSettings
		}
	})
	mu.RLock()
	defer mu.RUnlock()
	return settings, err
}

// Get 返回当前内存中的配置，**不会**触发磁盘 IO。
// 请确保在 main 包初始化阶段调用过 Load()。
func Get() Settings {
	mu.RLock()
	defer mu.RUnlock()
	return settings
}

// Save 覆盖并持久化配置，同时刷新全局内存变量。
func Save(s Settings) error {
	// 写入磁盘
	if err := saveToFile(s); err != nil {
		return err
	}

	// 刷新缓存
	mu.Lock()
	settings = s
	mu.Unlock()

	return nil
}

// -------- 内部方法 --------

func loadFromFile() (Settings, error) {
	// 如果文件不存在，创建带默认值的文件
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		if err := saveToFile(defaultSettings); err != nil {
			return defaultSettings, err
		}
		return defaultSettings, nil
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return defaultSettings, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return defaultSettings, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return s, nil
}

func saveToFile(s Settings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("编码配置失败: %w", err)
	}
	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}
