package commands

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/config"
	"github.com/Ivlyth/process-bandwidth/debug"
	"github.com/Ivlyth/process-bandwidth/engine"
	"github.com/Ivlyth/process-bandwidth/logging"
	"github.com/Ivlyth/process-bandwidth/pkg/util/kernel"
	"github.com/Ivlyth/process-bandwidth/web/api"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	errChan = make(chan error, 20)

	rootCmd = &cobra.Command{
		Use:   "process-bandwidth",
		Short: "A ebpf based program that can show you the bandwidth of each process (even each connection)",
		//TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "version" || cmd.Name() == "process-bandwidth" {
				return nil
			}

			stopper := make(chan os.Signal, 1)
			signal.Notify(stopper, syscall.SIGINT, syscall.SIGTERM)

			//环境检测
			//系统内核版本检测
			kv, err := kernel.HostVersion()
			if err != nil {
				fmt.Printf("error when get kernel version info: %s\n", err)
				os.Exit(1)
			}
			if kv < kernel.VersionCode(4, 18, 0) {
				fmt.Printf("Linux Kernel version %v is not supported. Need > 4.18 .\n", kv)
				os.Exit(1)
			}

			errChan := make(chan error, 20)

			if config.GlobalConfig.ProfilePort > 0 {
				logger.Debugf("start profile server at port: %d\n", config.GlobalConfig.ProfilePort)
				go debug.StartProfileServer(config.GlobalConfig.ProfilePort, errChan)
			}

			if config.GlobalConfig.WebServerPort > 0 {
				logger.Debugf("start web server at port: %d\n", config.GlobalConfig.WebServerPort)
				go api.StartWebServer(config.GlobalConfig.WebServerPort, errChan)
			}

			go engine.StartEngine(errChan)

			// FIXME
			go func() {
				select {
				case err = <-errChan:
					fmt.Printf("error: %s\n", err)
					os.Exit(1)
				case <-stopper:
					logger.Debugf("user stopped\n")
					os.Exit(0)
				}
			}()
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			newArgs := make([]string, len(args)+1)
			newArgs[0] = "top"
			copy(newArgs[1:], args)

			cmd.SetArgs(newArgs)
			return cmd.Execute()
		},
	}

	logger = logging.GetLogger()
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().Bool("debug", false, "debug mode")
	rootCmd.PersistentFlags().Uint16P("profile-port", "p", 0, "profile port, 0 disable profile")
	rootCmd.PersistentFlags().Uint16P("web-server-port", "w", 0, "web server port, 0 disable web server")
	rootCmd.PersistentFlags().DurationP("idle-time", "i", 10*time.Second, "idle timeout")
	rootCmd.PersistentFlags().IntP("worker-count", "c", 1, "worker count")
	rootCmd.PersistentFlags().IntP("snapshot-count", "H", 30, "snapshot count")
	rootCmd.PersistentFlags().IntP("channel-size", "s", 10000, "channel size for perf reader to worker")
}

func initConfig() {

	d, _ := rootCmd.PersistentFlags().GetBool("debug")
	config.GlobalConfig.Debug = d

	p, _ := rootCmd.PersistentFlags().GetUint16("profile-port")
	config.GlobalConfig.ProfilePort = p

	p, _ = rootCmd.PersistentFlags().GetUint16("web-server-port")
	config.GlobalConfig.WebServerPort = p

	du, _ := rootCmd.PersistentFlags().GetDuration("idle-time")
	config.GlobalConfig.IdleTimeout = du

	i, _ := rootCmd.PersistentFlags().GetInt("worker-count")
	config.GlobalConfig.WorkersCount = i

	i, _ = rootCmd.PersistentFlags().GetInt("snapshot-count")
	config.GlobalConfig.SnapShotCount = i

	i, _ = rootCmd.PersistentFlags().GetInt("channel-size")
	config.GlobalConfig.ChannelSize = i
}

func Execute() error {
	return rootCmd.Execute()
}
