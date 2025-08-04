package logger

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"okx-market-sentry/pkg/types"
)

// Logger zap日志包装器
type Logger struct {
	*zap.Logger
}

// InitLogger 初始化zap日志器
func InitLogger(config types.LogConfig) {
	// 从配置文件中解析日志级别
	var logMode = new(zapcore.Level)
	if err := logMode.UnmarshalText([]byte(config.Level)); err != nil {
		// 如果解析失败，使用默认的info级别
		*logMode = zapcore.InfoLevel
	}

	// 创建编码器
	encoder := getEncoder()
	// 创建写入器
	writeSyncer := getWriteSyncer(config)

	// 创建核心
	core := zapcore.NewTee(
		// 日志写入文件 级别为配置文件中的级别
		zapcore.NewCore(encoder, writeSyncer, *logMode),
		// 日志写入控制台 zapcore.Lock(os.Stdout) 在写入日志前获取锁 保证日志不会被其他日志打断
		zapcore.NewCore(getConsoleEncoder(), zapcore.Lock(os.Stdout), *logMode),
	)

	// AddCaller 将 Logger 配置为使用 zap 调用者的文件名、行号和函数名称注释每条消息
	lg := zap.New(core, zap.AddCaller())
	// 替换全局的logger
	zap.ReplaceGlobals(lg)
}

// New 创建logger实例（兼容性保留）
func New(level string) *Logger {
	return &Logger{Logger: zap.L()}
}

// 以下方法为兼容性保留，实际使用建议直接使用 zap.L()
func (l *Logger) Info(v ...interface{}) {
	zap.L().Sugar().Info(v...)
}

func (l *Logger) Warn(v ...interface{}) {
	zap.L().Sugar().Warn(v...)
}

func (l *Logger) Error(v ...interface{}) {
	zap.L().Sugar().Error(v...)
}

func (l *Logger) Debug(v ...interface{}) {
	zap.L().Sugar().Debug(v...)
}

// getEncoder 获取日志编码器
func getEncoder() zapcore.Encoder {
	// 编码器配置
	encoderConfig := zap.NewProductionEncoderConfig()
	// 将时间格式的key从 ts 改为 time
	encoderConfig.TimeKey = "time"
	// CapitalLevelEncoder 将 Level 序列化为全大写字符串
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// 时间格式化
	encoderConfig.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(t.Local().Format(time.DateTime))
	}
	return zapcore.NewJSONEncoder(encoderConfig)
}

// getConsoleEncoder 获取控制台编码器（更易读的格式）
func getConsoleEncoder() zapcore.Encoder {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(t.Local().Format("15:04:05"))
	}
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// getWriteSyncer 获取日志写入器 指定日志文件路径
func getWriteSyncer(config types.LogConfig) zapcore.WriteSyncer {
	// 获取系统分隔符
	stSeparator := string(filepath.Separator)
	// 获取当前工作目录
	stRootDir, _ := os.Getwd()
	// 日志文件路径 = 当前工作目录 + 日志文件路径 + 当前日期
	stLogFilePath := stRootDir + stSeparator + config.FilePath + stSeparator +
		time.Now().Format(time.DateOnly) + ".log"

	// 日志分割器
	lumberjackSyncer := &lumberjack.Logger{
		Filename:   stLogFilePath,     // 日志文件路径
		MaxSize:    config.MaxSize,    // 日志文件大小 单位：MB，超限后会自动切割
		MaxBackups: config.MaxBackups, // 日志文件备份数量
		MaxAge:     config.MaxAge,     // 日志文件存放时间 单位：天
		Compress:   config.Compress,   // 日志文件压缩
	}

	return zapcore.AddSync(lumberjackSyncer)
}
