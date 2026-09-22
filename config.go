package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
)

type DriveName string

const (
	DriveMakc                   = "makc"
	DriveWinputWithWindow       = "winputWithWindow"
	DriveWinputWithInterception = "winputWithInterception"
	DriveFakerInput             = "fakerInput"
	DriveNoop                   = "noop"
)

func (d *DriveName) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	driveName := DriveName(s)
	switch driveName {
	case DriveMakc, DriveWinputWithWindow, DriveWinputWithInterception, DriveFakerInput, DriveNoop:
		*d = driveName
		return nil
	default:
		return fmt.Errorf("unknown drive name: %s", driveName)
	}
}

func (d *DriveName) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(*d))
}

type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelError LogLevel = "error"
	LogLevelWarn  LogLevel = "warn"
	LogLevelInfo  LogLevel = "info"
)

func (l *LogLevel) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	logLevel := LogLevel(s)
	switch logLevel {
	case LogLevelDebug, LogLevelError, LogLevelWarn, LogLevelInfo:
		*l = logLevel
		return nil
	default:
		return fmt.Errorf("unknown log level: %s", logLevel)
	}
}

func (l *LogLevel) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(*l))
}

func LogLevel2slogLevel(logLevel LogLevel) slog.Level {
	switch logLevel {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelError:
		return slog.LevelError
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelInfo:
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

type Config struct {
	EnabledDriveName                   DriveName `json:"driveName"`                          // 驱动
	GRPCAddress                        string    `json:"gRPCAddress"`                        // 监听地址
	LogLevel                           LogLevel  `json:"logLevel"`                           // 日志等级
	GRPCKeepaliveTime                  int       `json:"gRPCKeepaliveTime"`                  // ServerParameters Time 单位 秒
	GRPCKeepaliveTimeOut               int       `json:"gRPCKeepaliveTimeOut"`               // ServerParameters TimeOut 单位 秒
	GRPCKeepaliveMaxConnectionIdle     int       `json:"gRPCKeepaliveMaxConnectionIdle"`     // ServerParameters MaxConnectionIdle 单位 秒
	GRPCEnforcementPolicyMinTime       int       `json:"gRPCEnforcementPolicyMinTime"`       // EnforcementPolicy MinTime 单位 秒
	GRPCEnforcementPermitWithoutStream bool      `json:"gRPCEnforcementPermitWithoutStream"` // EnforcementPolicy PermitWithoutStream
	GRPCServerShutdownTimeout          int       `json:"gRPCServerShutdownTimeout"`          // 服务关闭超时
	WinputWindowProcessName            string    `json:"winputWindowProcessName"`            // (winput无Interception驱动) FindByProcessName
	WinputWindowUseStaticIndex         bool      `json:"winputWindowUseStaticIndex"`         // (winput无Interception驱动) 是否使用固定index
	WinputWindowIndex                  int       `json:"winputWindowIndex"`                  // (winput无Interception驱动) 窗口index
	PprofAddress                       string    `json:"pprofAddress"`                       // 可选 pprof 监听地址，空表示关闭
	NoopLatencyMicros                  int       `json:"noopLatencyMicros"`                  // (noop后端) 每次按键模拟延迟 单位 微秒
}

var GlobalConfig Config

func LoadConfig() error {
	_, err := os.Stat("config.json")
	if os.IsNotExist(err) {
		slog.Warn("config.json not found, using defaults")
		defaultConfig := Config{
			EnabledDriveName:                   DriveMakc,
			GRPCAddress:                        "localhost:50051",
			LogLevel:                           LogLevelInfo,
			GRPCKeepaliveTime:                  2,
			GRPCKeepaliveTimeOut:               1,
			GRPCKeepaliveMaxConnectionIdle:     2,
			GRPCEnforcementPolicyMinTime:       10,
			GRPCEnforcementPermitWithoutStream: true,
			GRPCServerShutdownTimeout:          10,
			WinputWindowProcessName:            "Game.exe",
			WinputWindowUseStaticIndex:         true,
			WinputWindowIndex:                  0,
			PprofAddress:                       "",
			NoopLatencyMicros:                  0,
		}
		jsonBytes, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return err
		}
		err = os.WriteFile("config.json", jsonBytes, 0600)
		if err != nil {
			return err
		}
		GlobalConfig = defaultConfig
		return nil
	} else if err != nil {
		return fmt.Errorf("loadConfig Error: %v", err)
	}
	data, err := os.ReadFile("config.json")
	if err != nil {
		return fmt.Errorf("readConfig Error: %v", err)
	}
	err = json.Unmarshal(data, &GlobalConfig)
	if err != nil {
		return fmt.Errorf("unmarshal Error: %v", err)
	}
	slog.Info("loaded config.json")
	return nil
}
