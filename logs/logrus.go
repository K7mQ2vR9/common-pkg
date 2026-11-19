package logs

import (
	"context"
	"fmt"

	"github.com/K7mQ2vR9/base-pkg/consts"

	"github.com/sirupsen/logrus"
	"github.com/zeromicro/go-zero/core/logx"
)

type LogrusWriter struct {
	logger *logrus.Logger
	ctx    context.Context
}

func NewLogrusWriter(opts ...func(logger *logrus.Logger)) logx.Writer {
	logger := logrus.New()
	for _, opt := range opts {
		opt(logger)
	}

	return &LogrusWriter{
		logger: logger,
	}
}

func (w *LogrusWriter) Alert(v interface{}) {
	w.logger.Error(v)
}

// Close 完成同步日志适配器的关闭流程，可重复或并发调用。
// 输出及 Hook 的生命周期由配置它们的调用方管理。
func (w *LogrusWriter) Close() error {
	return nil
}

func (w *LogrusWriter) Debug(v interface{}, fields ...logx.LogField) {
	w.logger.WithFields(toLogrusFields(fields...)).Debug(v)
}

func (w *LogrusWriter) Info(v interface{}, fields ...logx.LogField) {
	w.logger.WithFields(toLogrusFields(fields...)).Info(v)
}

// Severe 按 fatal 级别记录严重错误，由调用方决定后续退出或清理流程。
func (w *LogrusWriter) Severe(v interface{}) {
	w.logger.Log(logrus.FatalLevel, v)
}

func (w *LogrusWriter) Slow(v interface{}, fields ...logx.LogField) {
	w.logger.WithFields(toLogrusFields(fields...)).Warn(v)
}

func (w *LogrusWriter) Stack(v interface{}) {
	w.logger.Error(v)
}

func (w *LogrusWriter) Stat(v interface{}, fields ...logx.LogField) {
	w.logger.WithFields(toLogrusFields(fields...)).Info(v)
}

func (w *LogrusWriter) Error(v interface{}, fields ...logx.LogField) {
	w.logger.WithFields(toLogrusFields(fields...)).Error(v)
}

func (w *LogrusWriter) Errorf(format string, v ...any) {
	w.Error(fmt.Sprintf(format, v...))
}

func (w *LogrusWriter) Infof(format string, v ...any) {
	w.Info(fmt.Sprintf(format, v...))
}

func (w *LogrusWriter) Errorv(v interface{}, fields ...logx.LogField) {
	w.logger.WithFields(toLogrusFields(fields...)).Error(v)
}

func (w *LogrusWriter) Infov(v interface{}, fields ...logx.LogField) {
	w.logger.WithFields(toLogrusFields(fields...)).Info(v)
}

func toLogrusFields(fields ...logx.LogField) logrus.Fields {
	logrusFields := make(logrus.Fields)
	for _, field := range fields {
		if field.Key == consts.HeaderToken {
			continue
		}
		logrusFields[field.Key] = field.Value
	}
	return logrusFields
}

func Error(ctx context.Context, v ...any) {
	getLogger(ctx).Error(v...)
}

func Info(ctx context.Context, v ...any) {
	getLogger(ctx).Info(v...)
}

func getLogger(ctx context.Context) logx.Logger {
	return logx.WithContext(ctx).WithCallerSkip(1)
}
