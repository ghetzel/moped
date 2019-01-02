package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ghetzel/cli"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/moped"
)

type OnQuitFunc func() // {}
var OnQuit []OnQuitFunc

func main() {
	app := cli.NewApp()
	app.Name = `moped`
	app.Usage = `A pluggable protocol-compatible implementation of Media Player Daemon (MPD).`
	app.Version = moped.Version

	var application *moped.Moped

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   `log-level, L`,
			Usage:  `Level of log output verbosity`,
			Value:  `debug`,
			EnvVar: `LOGLEVEL`,
		},
		cli.StringFlag{
			Name:  `address, a`,
			Usage: `The address to listen on.`,
			Value: `127.0.0.1:6601`,
		},
		cli.StringFlag{
			Name:  `config, c`,
			Usage: `The path to the configuration file.`,
			Value: `~/.config/moped/moped.yml`,
		},
	}

	app.Before = func(c *cli.Context) error {
		log.SetLevelString(c.String(`log-level`))

		if config, err := moped.LoadConfigFromFile(c.String(`config`)); err == nil {
			application = moped.NewMoped()

			if libraries, err := moped.GetLibrariesFromConfig(config); err == nil {
				for name, lib := range libraries {
					if err := application.AddLibrary(name, lib); err != nil {
						return err
					}
				}
			} else {
				return err
			}
		} else {
			return err
		}

		return nil
	}

	app.Action = func(c *cli.Context) {
		if err := application.Listen(`tcp`, c.String(`address`)); err != nil {
			log.Fatal(err)
		}
	}

	app.Commands = []cli.Command{
		{
			Name:      `ls`,
			Usage:     `List the contents of a given path.`,
			ArgsUsage: `[PATH]`,
			Action: func(c *cli.Context) {
				uri := c.Args().First()

				if entries, err := application.Browse(uri); err == nil {
					for _, entry := range entries {
						fmt.Printf("%v\n", entry)
					}
				} else {
					log.Fatal(err)
				}
			},
		}, {
			Name:      `probe`,
			Usage:     `Probe the metadata for the given file`,
			ArgsUsage: `PATH`,
			Action: func(c *cli.Context) {
				if uri := c.Args().First(); uri != `` {
					if entry, err := application.Get(uri); err == nil {
						if metadata, err := application.GetMetadata(entry); err == nil {
							if output, err := json.MarshalIndent(metadata, ``, `  `); err == nil {
								fmt.Println(string(output))
							} else {
								log.Fatal(err)
							}
						} else {
							log.Fatal(err)
						}
					} else {
						log.Fatal(err)
					}
				} else {
					log.Fatalf("Must specify a PATH to probe")
				}
			},
		},
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		<-sigc

		if application != nil {
			application.Stop()
		}

		log.Debugf("exit")
		os.Exit(-1)
	}()

	app.Run(os.Args)
}
