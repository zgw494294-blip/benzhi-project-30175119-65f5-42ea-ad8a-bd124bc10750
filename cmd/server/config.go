package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type config struct {
	addr      string
	dbPath    string
	selfcheck bool
}

func defaultAddress() (string, error) {
	p := strings.TrimSpace(os.Getenv("PORT"))
	if p == "" {
		return "127.0.0.1:19081", nil
	}
	port, err := strconv.Atoi(p)
	if err != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), nil
}

func parseConfig(args []string) (config, error) {
	def, err := defaultAddress()
	if err != nil {
		return config{}, err
	}
	fs := flag.NewFlagSet("dialect-release", flag.ContinueOnError)
	var c config
	fs.StringVar(&c.addr, "addr", def, "HTTP 监听地址")
	fs.StringVar(&c.dbPath, "db", "dialect-release.db", "SQLite 数据库路径")
	fs.BoolVar(&c.selfcheck, "selfcheck", false, "执行真实 HTTP 发布闭环后退出")
	if err = fs.Parse(args); err != nil {
		return c, err
	}
	host, portText, err := net.SplitHostPort(c.addr)
	if err != nil {
		return c, fmt.Errorf("-addr 必须是 host:port：%w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return c, fmt.Errorf("-addr 端口必须是 1 到 65535")
	}
	if strings.TrimSpace(host) == "" {
		return c, fmt.Errorf("-addr 不允许省略主机地址")
	}
	return c, nil
}
