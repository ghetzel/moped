package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/pathutil"
	"github.com/ghetzel/moped/backends"
	"github.com/ghetzel/moped/library"
	"github.com/ghetzel/moped/metadata"
	"github.com/ghodss/yaml"
)

type LibraryConfig struct {
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	Configuration map[string]interface{} `json:"config"`
}

type Configuration struct {
	Libraries []LibraryConfig `json:"libraries"`
	Patterns  []string        `json:"patterns"`
}

func LoadConfigFromFile(f string) (*Configuration, error) {
	if filename, err := pathutil.ExpandUser(f); err == nil {
		var config Configuration

		if file, err := os.Open(filename); err == nil {
			defer file.Close()

			if data, err := ioutil.ReadAll(file); err == nil {
				if err := yaml.Unmarshal(data, &config); err == nil {
					// initialize regexp patterns for the metadata package
					for _, pattern := range config.Patterns {
						if rx, err := regexp.Compile(pattern); err == nil {
							metadata.RegexpPatterns = append(metadata.RegexpPatterns, rx)
						} else {
							return nil, err
						}
					}

					return &config, nil
				} else {
					return nil, err
				}
			} else {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func GetLibrariesFromConfig(config *Configuration) (map[string]library.Library, error) {
	libraries := make(map[string]library.Library)

	if config != nil {
		for i, libconfig := range config.Libraries {
			var lib library.Library
			var err error

			if libconfig.Name == `` {
				return nil, fmt.Errorf("Must specify a name for library %d", i)
			}

			switch libconfig.Type {
			case `local`:
				var cfg backends.FilesystemConfig

				if err := maputil.TaggedStructFromMap(libconfig.Configuration, &cfg, `json`); err != nil {
					return nil, fmt.Errorf("Error configuring library %d: %v", i, err)
				}

				lib, err = backends.NewFilesystemBackend(&cfg)
			}

			if err == nil {
				libraries[libconfig.Name] = lib
			} else {
				return nil, fmt.Errorf("Error configuring library %d: %v", i, err)
			}
		}
	}

	return libraries, nil
}
