// Package app provides the cli app shell of an unrily p2p node
package app

import (
	"fmt"
	api "github.com/UnrulyOS/go-unruly/api"
	apiconf "github.com/UnrulyOS/go-unruly/api/config"
	"github.com/UnrulyOS/go-unruly/log"
	"github.com/UnrulyOS/go-unruly/node"
	"gopkg.in/urfave/cli.v1"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/UnrulyOS/go-unruly/app/config"
	nodeparams "github.com/UnrulyOS/go-unruly/node/config"
)

type UnrulyApp struct {
	*cli.App
	node *node.Node
	grpcApiService *api.UnrulyGrpcService
	jsonApiService *api.JsonHttpServer
}

var (
	app      = NewApp()
	appFlags = []cli.Flag{
		config.LoadConfigFileFlag,
		config.DataFolderPathFlag,
		// add all app flags here ...
	}
	nodeFlags = []cli.Flag{
		nodeparams.KSecurityFlag,
		nodeparams.LocalTcpPortFlag,
		// add all node flags here ...
	}
	apiFlags = []cli.Flag{
		apiconf.StartGrpcApiServerFlag,
		apiconf.GrpcServerPortFlag,
		apiconf.StartJsonApiServerFlag,
		apiconf.JsonServerPortFlag,
	}
	// add flags for other new modules here....
	exitApp = make(chan bool, 1)
)

// add toml config file support and sample toml file

func init() {
	// define main app action
	app.Action = startUnrulyNode
}

func NewApp() *UnrulyApp {
	app := cli.NewApp()
	app.Name = filepath.Base(os.Args[0])
	app.Author = "The go-unruly authors"
	app.Email = "app@unrulyos.io"
	app.Version = "0.0.1"
	if len(config.GitCommitHash) > 8 {
		app.Version += " - " + config.GitCommitHash[:8]
	}
	app.Usage = config.AppUsage
	app.HideVersion = true
	app.Copyright = "(c) 2017 The go-unruly Authors"
	app.Commands = []cli.Command{
		NewVersionCommand(config.AppVersion),
		// add all other commands here
	}

	app.Flags = append(app.Flags, appFlags...)
	app.Flags = append(app.Flags, nodeFlags...)
	app.Flags = append(app.Flags, apiFlags...)

	sort.Sort(cli.FlagsByName(app.Flags))
	app.Before = func(ctx *cli.Context) error {
		// max out box for now
		runtime.GOMAXPROCS(runtime.NumCPU())
		// exit gracefully - e.g. with app cleanup on sig abort (ctrl-c)
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		go func() {
			for _ = range signalChan {
				log.Info("Received an interrupt, stopping services...\n")
				exitApp <- true
			}
		}()
		// todo: add misc app setup here (metrics, debug, etc....)
		return nil
	}

	app.After = func(ctx *cli.Context) error {
		log.Info("App cleanup goes here...")
		// post app cleanup goes here
		return nil
	}

	return &UnrulyApp{app, nil, nil, nil}
}

// start the unruly node
func startUnrulyNode(ctx *cli.Context) error {

	port := nodeparams.LocalTcpPortFlag.Destination
	app.node = node.NewLocalNode(*port, exitApp)

	conf := &apiconf.ConfigValues

	// start api servers

	if conf.StartGrpcServer || conf.StartJsonServer {

		app.grpcApiService = api.NewGrpcService()
		app.grpcApiService.StartService()
	}

	if conf.StartJsonServer {
		app.jsonApiService = api.NewJsonHttpServer()
		app.jsonApiService.Start()
	}

	// wait until node signaled app to exit
	<-exitApp
	return nil
}

// The Unruly console application - responsible for parsing and routing cli flags and commands
// this is the root of all evil, called from Main.main()
func Main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
