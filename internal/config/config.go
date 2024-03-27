package config

import (
	"log"

	"github.com/spf13/viper"
)

// InitConfig inits configs based on the provided env
func InitConfig(env string) {
	viper.SetConfigName(env)        // name of config file (without extension)
	viper.SetConfigType("yaml")     // or viper.SetConfigType("YAML")
	viper.AddConfigPath("configs/") // path to look for the config file in

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
}

// GetConfigValue reads a value from the loaded configuration using a key.
// The key can represent any level of nesting by separating levels with a dot.
// For example, "server.port" would retrieve the port value from the server configuration.
func GetConfigValue(key string) interface{} {
	return viper.Get(key)
}
