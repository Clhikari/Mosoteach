package main

import (
	"fmt"
	"mosoteach/internal/config"
	"mosoteach/internal/web"
	"os"
)

func main() {
	cfg := config.GetConfig()
	if err := cfg.Load(); err != nil {
		fmt.Printf("错误: 加载配置失败: %v\n", err)
		fmt.Println("请确保 user_data.json 文件存在且格式正确")
		os.Exit(1)
	}

	server := web.NewServer()
	if err := server.Start(11451); err != nil {
		fmt.Printf("错误: 启动服务器失败: %v\n", err)
		os.Exit(1)
	}
}
