package base

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
)

type Command struct {
	// LogLevel sets the verbosity level of the application logging.
	LogLevel string `short:"L" long:"log-level" description:"The level of logging produced by the application." optional:"yes" choice:"off" choice:"debug" choice:"info" choice:"warn" choice:"error" default:"warn" env:"CLOUDCTL_LOG_LEVEL"`
	// LogStream is the output channel to use for logging.
	LogStream string `short:"S" long:"log-stream" description:"The output stream to use for logging." optional:"yes" choice:"stdout" choice:"stderr" choice:"file" choice:"none" default:"stderr" env:"CLOUDCTL_LOG_STREAM"`
	// LogStream is the type of logger to use.
	LogFormat string `short:"F" long:"log-format" description:"The format of the logging messages." optional:"yes" choice:"text" choice:"json" default:"text" env:"CLOUDCTL_LOG_FORMAT"`
	// CPUProfile sets the (optional) path of the file for CPU profiling info.
	CPUProfile string `short:"C" long:"cpu-profile" description:"The (optional) path where the CPU profiler will store its data." optional:"yes"`
	// MemProfile sets the (optional) path of the file for memory profiling info.
	MemProfile string `short:"M" long:"mem-profile" description:"The (optional) path where the memory profiler will store its data." optional:"yes"`
	// AutomationFriendly enables automation-friendly JSON output.
	AutomationFriendly bool `short:"A" long:"automation-friendly" description:"Whether to output in automation friendly JSON format." optional:"yes"`
}

// Init initialises the command consuming the standard, common arguments.
func (cmd *Command) Init() {
	var err error
	var stream io.Writer = os.Stderr
	options := &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}
	switch cmd.LogLevel {
	case "off":
		options.Level = slog.LevelError
		// also make sure that the few error log messages get discarded
		stream = io.Discard
	case "error":
		options.Level = slog.LevelError
	case "warn":
		options.Level = slog.LevelWarn
	case "info":
		options.Level = slog.LevelInfo
	case "debug":
		options.Level = slog.LevelDebug
	}

	switch cmd.LogStream {
	case "stdout":
		stream = os.Stdout
	case "stderr":
		stream = os.Stderr
	case "file":
		exe, _ := os.Executable()
		path := fmt.Sprintf("%s-%d.log", strings.Replace(exe, ".exe", "", -1), os.Getpid())
		if stream, err = os.Create(path); err != nil {
			stream = io.Discard
		}
	case "none":
		stream = io.Discard
	}

	var handler slog.Handler
	switch cmd.LogFormat {
	case "text":
		handler = slog.NewTextHandler(stream, options)
	case "json":
		handler = slog.NewJSONHandler(stream, options)
	}

	slog.SetDefault(slog.New(handler))
}

func (cmd *Command) ProfileCPU() *Closer {
	var f *os.File
	if cmd.CPUProfile != "" {
		var err error
		f, err = os.Create(cmd.CPUProfile)
		if err != nil {
			slog.Error("could not create CPU profile", "path", cmd.CPUProfile, "error", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			slog.Error("could not start CPU profiler", "error", err)
		}
	}
	return &Closer{
		file: f,
	}
}

func (cmd *Command) ProfileMemory() {
	if cmd.MemProfile != "" {
		f, err := os.Create(cmd.MemProfile)
		if err != nil {
			slog.Error("could not create memory profile", "path", cmd.MemProfile, "error", err)
		}
		defer f.Close()
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			slog.Error("could not write memory profile", "error", err)
		}
	}
}
