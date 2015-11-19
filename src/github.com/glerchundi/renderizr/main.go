package main

import (
	"strings"
	"os"

	"github.com/docker/libkv/store"
	renderizr "github.com/glerchundi/renderizr/pkg"
	"github.com/glerchundi/renderizr/pkg/config"
	"github.com/glerchundi/renderizr/pkg/util"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
)

const (
	cliName        = "renderizr"
	cliDescription = "renderizr manages local application configuration files using templates and data."
)

var (
	globalCfg = config.NewGlobalConfig()
	consulCfg = config.NewConsulBackendConfig()
	etcdCfg = config.NewEtcdBackendConfig()
	zookeeperCfg = config.NewZookeeperBackendConfig()

	backendCfgs = map[store.Backend]config.BackendConfig{
		store.CONSUL: consulCfg,
		store.ETCD:   etcdCfg,
		store.ZK:     zookeeperCfg,
	}
)

func AddGlobalFlags(fs *flag.FlagSet, gc *config.GlobalConfig) {
	fs.StringVar(&gc.Prefix, "prefix", gc.Prefix, "Key path prefix")
	fs.StringSliceVar(&gc.Templates, "template", gc.Templates, "Template parameters like 'file.conf.tmpl;file.conf;0600;check;reload-cmd'")
	fs.BoolVar(&gc.Onetime, "onetime", gc.Onetime, "Run once and exit")
	fs.BoolVar(&gc.Watch, "watch", gc.Watch, "Enable watch")
	fs.DurationVar(&gc.ResyncInterval, "resync-interval", gc.ResyncInterval, "Backend polling resync interval")
	fs.BoolVar(&gc.NoOp, "noop", gc.NoOp, "Only show pending changes")
}

func AddConsulFlags(fs *flag.FlagSet, cbc *config.ConsulBackendConfig) {
	fs.StringSliceVar(&cbc.Endpoints, "endpoint", cbc.Endpoints, "List of consul endpoints")
	fs.StringVar(&cbc.CertFile, "cert-file", cbc.CertFile, "Identify HTTPS client using this SSL certificate file")
	fs.StringVar(&cbc.KeyFile, "key-file", cbc.KeyFile, "Identify HTTPS client using this SSL key file")
	fs.StringVar(&cbc.CAFile, "ca-file", cbc.CAFile, "Verify certificates of HTTPS-enabled servers using this CA bundle")
}

func AddEtcdFlags(fs *flag.FlagSet, ebc *config.EtcdBackendConfig) {
	fs.StringSliceVar(&ebc.Endpoints, "endpoint", ebc.Endpoints, "List of etcd endpoints")
	fs.StringVar(&ebc.CertFile, "cert-file", ebc.CertFile, "Identify HTTPS client using this SSL certificate file")
	fs.StringVar(&ebc.KeyFile, "key-file", ebc.KeyFile, "Identify HTTPS client using this SSL key file")
	fs.StringVar(&ebc.CAFile, "ca-file", ebc.CAFile, "Verify certificates of HTTPS-enabled servers using this CA bundle")
}

func AddZookeeperFlags(fs *flag.FlagSet, zbc *config.ZookeeperBackendConfig) {
	fs.StringSliceVar(&zbc.Endpoints, "endpoint", zbc.Endpoints, "List of zookeeper endpoints")
}

func main() {
	// initialize logs
	util.InitLogs()
	defer util.FlushLogs()

	// commands
	rootCmd := &cobra.Command{
		Use:   cliName,
		Short: cliDescription,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	rootCmd.SetGlobalNormalizationFunc(
		func(f *flag.FlagSet, name string) flag.NormalizedName {
			if strings.Contains(name, "_") {
				return flag.NormalizedName(strings.Replace(name, "_", "-", -1))
			}
			return flag.NormalizedName(name)
		},
	)

	consulCmd := &cobra.Command{Use: string(store.CONSUL), Run: run}
	rootCmd.AddCommand(consulCmd)

	etcdCmd := &cobra.Command{Use: string(store.ETCD), Run: run}
	rootCmd.AddCommand(etcdCmd)

	zookeeperCmd := &cobra.Command{Use: string(store.ZK), Run: run}
	rootCmd.AddCommand(zookeeperCmd)

	// flags
	AddGlobalFlags(rootCmd.PersistentFlags(), globalCfg)
	AddConsulFlags(consulCmd.Flags(), consulCfg)
	AddEtcdFlags(etcdCmd.Flags(), etcdCfg)
	AddZookeeperFlags(zookeeperCmd.Flags(), zookeeperCfg)

	// execute!
	rootCmd.Execute()
}

func run(cmd *cobra.Command, args []string) {
	// Set flags form env's (if not set explicitly)
	setFromEnvs := func(prefix string, flagSet *flag.FlagSet) {
		flagSet.VisitAll(func(f *flag.Flag) {
			if !f.Changed {
				key := strings.ToUpper(strings.Join(
					[]string{
						prefix,
						strings.Replace(f.Name, "-", "_", -1),
					},
					"_",
				))
				val := os.Getenv(key)
				if val != "" {
					flagSet.Set(f.Name, val)
				}
			}
		})
	}

	setFromEnvs(cliName, cmd.Parent().PersistentFlags())
	setFromEnvs(strings.Join([]string{cliName, cmd.Name()}, "_"), cmd.Flags())

	// and then, run!
	renderizr.Run(globalCfg, backendCfgs[store.Backend(cmd.Name())])
}
