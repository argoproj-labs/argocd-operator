package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"

	//filelog "smtplistener/internal/log"
	"smtplistener/internal/processor"
	"smtplistener/internal/util"
	"strconv"
	"strings"
	"syscall"

	"github.com/flashmob/go-guerrilla"
	"github.com/flashmob/go-guerrilla/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	defaultPidFile = "/var/run/blaze_listener.pid"
)

var (
	configPath string
	pidFile    string

	serveCmd = &cobra.Command{
		Use:   "serve",
		Short: "start the small SMTP server",
		Run:   serve,
	}

	signalChannel = make(chan os.Signal, 1) // for trapping SIGHUP and friends
	mainlog       log.Logger

	d guerrilla.Daemon
)

func init() {

	//Blaze Logging
	lvl, ok := os.LookupEnv("LOG_LEVEL")
	// LOG_LEVEL not set, let's default to info
	if !ok {
		lvl = "info"
	}
	level, err := logrus.ParseLevel(lvl)
	if err != nil {
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)
	logrus.SetReportCaller(true)
	formatter := &logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.999999999Z07:00",
		// FullTimestamp:          true,
		// DisableLevelTruncation: false,
		// PadLevelText:           true,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "@timestamp",
			logrus.FieldKeyLevel: "@level",
			logrus.FieldKeyMsg:   "@message",
			logrus.FieldKeyFunc:  "@caller",
		},
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			function := strings.Split(f.Function, ("/"))
			return function[len(function)-1], fmt.Sprintf("%s:%d", formatFilePath(f.File), f.Line)
		},
	}
	logrus.SetFormatter(formatter)
	// Blaze logging setup

	// log to stderr on startup
	//var err error
	mainlog, err = log.GetLogger(log.OutputStderr.String(), log.InfoLevel.String())
	if err != nil {
		mainlog.WithError(err).Errorf("Failed creating a logger to %s", log.OutputStderr)
	}
	configFile := util.GetConfig()
	serveCmd.PersistentFlags().StringVarP(&configPath, "config", "c",
		configFile, "Path to the configuration file")
	// intentionally didn't specify default pidFile; value from config is used if flag is empty
	serveCmd.PersistentFlags().StringVarP(&pidFile, "pidFile", "p",
		"", "Path to the pid file")
	rootCmd.AddCommand(serveCmd)
}

func sigHandler() {
	// handle SIGHUP for reloading the configuration while running
	signal.Notify(signalChannel,
		syscall.SIGHUP,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGINT,
		syscall.SIGKILL,
		syscall.SIGUSR1,
	)
	// Keep the daemon busy by waiting for signals to come
	for sig := range signalChannel {
		if sig == syscall.SIGHUP {
			d.ReloadConfigFile(configPath)
		} else if sig == syscall.SIGUSR1 {
			d.ReopenLogs()
		} else if sig == syscall.SIGTERM || sig == syscall.SIGQUIT || sig == syscall.SIGINT {
			mainlog.Infof("Shutdown signal caught")
			d.Shutdown()
			mainlog.Infof("Shutdown completed, exiting.")
			return
		} else {
			mainlog.Infof("Shutdown, unknown signal caught")
			return
		}
	}
}

func serve(cmd *cobra.Command, args []string) {
	//logVersion()

	// Here we initialize our Guerrilla Daemon
	// See the reference docs here:
	d = guerrilla.Daemon{Logger: mainlog}

	// add the Processor to be identified as "MailDir"
	d.AddProcessor("FileWriter", processor.FileWriter)
	// add the FastCGI processor
	//d.AddProcessor("FastCGI", fcgi_processor.Processor)

	err := readConfig(configPath, pidFile)
	if err != nil {
		mainlog.WithError(err).Fatal("Error while reading config")
	}
	// Check that max clients is not greater than system open file limit.
	fileLimit := getFileLimit()
	if fileLimit > 0 {
		maxClients := 0
		for _, s := range d.Config.Servers {
			maxClients += s.MaxClients
		}
		if maxClients > fileLimit {
			mainlog.Fatalf("Combined max clients for all servers (%d) is greater than open file limit (%d). "+
				"Please increase your open file limit or decrease max clients.", maxClients, fileLimit)
		}
	}
	logpath := d.Config.BackendConfig["logging_file_path"]
	fmt.Println(logpath)
	//filelog.Filepath = logpath.(string)
	err = d.Start()
	if err != nil {
		mainlog.WithError(err).Error("Error(s) when starting server(s)")
		os.Exit(1)
	}

	sigHandler()
}

// Superset of `guerrilla.AppConfig` containing options specific
// the the command line interface.
type CmdConfig struct {
	guerrilla.AppConfig
}

func (c *CmdConfig) emitChangeEvents(oldConfig *CmdConfig, app guerrilla.Guerrilla) {
	// if your CmdConfig has any extra fields, you can emit events here
	// ...

	// call other emitChangeEvents
	c.AppConfig.EmitChangeEvents(&oldConfig.AppConfig, app)
}

// ReadConfig which should be called at startup
func readConfig(path string, pidFile string) error {

	if _, err := d.LoadConfig(path); err != nil {
		return err
	}

	// override config pidFile with with flag from the command line
	if len(pidFile) > 0 {
		d.Config.PidFile = pidFile
	} else if len(d.Config.PidFile) == 0 {
		d.Config.PidFile = defaultPidFile
	}

	if len(d.Config.AllowedHosts) == 0 {
		return errors.New("Empty `allowed_hosts` is not allowed")
	}
	return nil
}

func getFileLimit() int {
	cmd := exec.Command("ulimit", "-n")
	out, err := cmd.Output()
	if err != nil {
		return -1
	}
	limit, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return -1
	}
	return limit
}

func formatFilePath(path string) string {
	arr := strings.Split(path, "/")
	return fmt.Sprintf("%s:%s", arr[len(arr)-2], arr[len(arr)-1])
}
