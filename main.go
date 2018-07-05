package main

import (
	"fmt"
	_ "net/http/pprof"
	"os"

	"github.com/ghetzel/cli"
	"github.com/ghetzel/go-stockutil/log"
)

type OnQuitFunc func() // {}
var OnQuit []OnQuitFunc

func main() {
	app := cli.NewApp()
	app.Name = `moped`
	app.Usage = `A pluggable protocol-compatible implementation of Media Player Daemon (MPD).`
	app.Version = `0.0.1`
	app.EnableBashCompletion = false

	var application *Moped

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

		if config, err := LoadConfigFromFile(c.String(`config`)); err == nil {
			application = NewMoped()

			if libraries, err := GetLibrariesFromConfig(config); err == nil {
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
			Name:      `play`,
			Usage:     `Play a given audio file.`,
			ArgsUsage: `PATH`,
			Action: func(c *cli.Context) {
				if uri := c.Args().First(); uri != `` {
					if entry, err := application.Get(uri); err == nil {
						if err := application.PlayAndWait(entry); err != nil {
							log.Fatal(err)
						}
					} else {
						log.Fatal(err)
					}
				} else {
					log.Fatalf("Must specify a PATH to play")
				}
			},
		},
	}

	app.Run(os.Args)
}
