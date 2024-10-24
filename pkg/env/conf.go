package env

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

func LoadConf(filename, subConf string, s interface{}) {
	var path string
	path = filepath.Join(GetConfDirPath(), subConf, filename)

	if yamlFile, err := os.ReadFile(path); err != nil {
		panic(filename + " get error: " + err.Error())
	} else if err = yaml.Unmarshal(yamlFile, s); err != nil {
		panic(filename + " unmarshal error: " + err.Error())
	}
}
