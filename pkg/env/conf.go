package env

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func LoadConf(filename, subConf string, s interface{}) {
	var path string
	path = filepath.Join(GetConfDirPath(), subConf, filename)
	yamlFile, err := os.ReadFile(path)
	if err != nil {
		panic(filename + " get error: " + err.Error())
	}
	yamlConf := []byte(os.ExpandEnv(string(yamlFile)))
	if err = yaml.Unmarshal(yamlConf, s); err != nil {
		panic(filename + " unmarshal error: " + err.Error())
	}
}
