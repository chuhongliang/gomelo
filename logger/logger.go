package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	mu      sync.Mutex
	level   Level
	output  io.Writer
	prefix  string
	format  string
	console bool
	file    *os.File
}

type Option func(*Logger)

func WithOutput(w io.Writer) Option {
	return func(l *Logger) { l.output = w }
}

func WithPrefix(prefix string) Option {
	return func(l *Logger) { l.prefix = prefix }
}

func WithLevel(level Level) Option {
	return func(l *Logger) { l.level = level }
}

func WithConsole(enable bool) Option {
	return func(l *Logger) { l.console = enable }
}

func WithFile(path string) Option {
	return func(l *Logger) {
		if path == "" {
			return
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return
		}
		l.file = f
	}
}

func WithFormat(format string) Option {
	return func(l *Logger) { l.format = format }
}

func New(output io.Writer, opts ...Option) *Logger {
	if output == nil {
		output = os.Stdout
	}

	l := &Logger{
		level:   InfoLevel,
		output:  output,
		format:  "[%s] %s %s %s\n",
		console: true,
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	l.level = level
	l.mu.Unlock()
}

func (l *Logger) Level() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

func (l *Logger) shouldLog(level Level) bool {
	return level >= l.level
}

func (l *Logger) log(level Level, v ...any) {
	if !l.shouldLog(level) {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprint(v...)

	line := fmt.Sprintf(l.format, timestamp, level.String(), l.prefix, msg)

	if l.console {
		os.Stdout.Write([]byte(line))
	}

	if l.output != nil && l.output != os.Stdout {
		l.output.Write([]byte(line))
	}

	if l.file != nil {
		l.file.Write([]byte(line))
	}
}

func (l *Logger) Debug(v ...any) {
	l.log(DebugLevel, v...)
}

func (l *Logger) Info(v ...any) {
	l.log(InfoLevel, v...)
}

func (l *Logger) Warn(v ...any) {
	l.log(WarnLevel, v...)
}

func (l *Logger) Error(v ...any) {
	l.log(ErrorLevel, v...)
}

func (l *Logger) Fatal(v ...any) {
	l.log(FatalLevel, v...)
	os.Exit(1)
}

func (l *Logger) Debugf(format string, v ...any) {
	l.log(DebugLevel, fmt.Sprintf(format, v...))
}

func (l *Logger) Infof(format string, v ...any) {
	l.log(InfoLevel, fmt.Sprintf(format, v...))
}

func (l *Logger) Warnf(format string, v ...any) {
	l.log(WarnLevel, fmt.Sprintf(format, v...))
}

func (l *Logger) Errorf(format string, v ...any) {
	l.log(ErrorLevel, fmt.Sprintf(format, v...))
}

func (l *Logger) Fatalf(format string, v ...any) {
	l.log(FatalLevel, fmt.Sprintf(format, v...))
	os.Exit(1)
}

var (
	defaultLogger *Logger
	defaultMu     sync.RWMutex
)

func init() {
	defaultLogger = New(os.Stdout, WithPrefix("pomelo"))
}

func Default() *Logger {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultLogger
}

func SetDefault(l *Logger) {
	defaultMu.Lock()
	defaultLogger = l
	defaultMu.Unlock()
}

func SetLevel(level Level) {
	Default().SetLevel(level)
}

func Debug(v ...any) { Default().Debug(v...) }
func Info(v ...any)  { Default().Info(v...) }
func Warn(v ...any)  { Default().Warn(v...) }
func Error(v ...any) { Default().Error(v...) }
func Fatal(v ...any) { Default().Fatal(v...) }

func Debugf(format string, v ...any) { Default().Debugf(format, v...) }
func Infof(format string, v ...any)  { Default().Infof(format, v...) }
func Warnf(format string, v ...any)  { Default().Warnf(format, v...) }
func Errorf(format string, v ...any) { Default().Errorf(format, v...) }
func Fatalf(format string, v ...any) { Default().Fatalf(format, v...) }

type FileLogger struct {
	*Logger
	file *os.File
}

func NewFileLogger(dir string, prefix string) (*FileLogger, error) {
	if dir == "" {
		dir = "./logs"
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	filename := fmt.Sprintf("%s_%s.log", prefix, time.Now().Format("2006-01-02"))
	fpath := filepath.Join(dir, filename)

	f, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &FileLogger{
		Logger: New(f, WithPrefix(prefix), WithConsole(true)),
		file:   f,
	}, nil
}

func (f *FileLogger) Close() error {
	if f.file != nil {
		return f.file.Close()
	}
	return nil
}

type RotatingLogger struct {
	*Logger
	dir      string
	prefix   string
	maxSize  int64
	maxFiles int
	curSize  int64
}

func NewRotatingLogger(dir string, prefix string, maxSize int64, maxFiles int) (*RotatingLogger, error) {
	if dir == "" {
		dir = "./logs"
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	rl := &RotatingLogger{
		dir:      dir,
		prefix:   prefix,
		maxSize:  maxSize,
		maxFiles: maxFiles,
	}

	f, err := rl.createFile()
	if err != nil {
		return nil, err
	}

	rl.Logger = New(f, WithPrefix(prefix), WithConsole(true))

	return rl, nil
}

func (r *RotatingLogger) createFile() (*os.File, error) {
	filename := fmt.Sprintf("%s_%s.log", r.prefix, time.Now().Format("2006-01-02_15-04-05"))
	fpath := filepath.Join(r.dir, filename)

	f, err := os.OpenFile(fpath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	stat, _ := f.Stat()
	r.curSize = stat.Size()

	return f, nil
}

func (r *RotatingLogger) rotate() error {
	if f, ok := r.Logger.output.(*os.File); ok {
		f.Close()
	}

	f, err := r.createFile()
	if err != nil {
		return err
	}

	r.Logger.output = f

	r.cleanOldFiles()

	return nil
}

func (r *RotatingLogger) cleanOldFiles() {
	if r.maxFiles <= 0 {
		return
	}

	pattern := filepath.Join(r.dir, r.prefix+"_*.log")
	matches, _ := filepath.Glob(pattern)

	if len(matches) > r.maxFiles {
		for i := 0; i < len(matches)-r.maxFiles; i++ {
			os.Remove(matches[i])
		}
	}
}

func (r *RotatingLogger) Debug(v ...any) {
	r.Logger.Debug(v...)
	r.checkRotate()
}

func (r *RotatingLogger) Info(v ...any) {
	r.Logger.Info(v...)
	r.checkRotate()
}

func (r *RotatingLogger) Warn(v ...any) {
	r.Logger.Warn(v...)
	r.checkRotate()
}

func (r *RotatingLogger) Error(v ...any) {
	r.Logger.Error(v...)
	r.checkRotate()
}

func (r *RotatingLogger) checkRotate() {
	if r.curSize >= r.maxSize {
		r.rotate()
	}
}

func (r *RotatingLogger) Close() error {
	if f, ok := r.Logger.output.(*os.File); ok {
		return f.Close()
	}
	return nil
}
