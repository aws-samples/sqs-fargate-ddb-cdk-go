package main

import (
	"encoding/json"
	"io"
	"os"
)

// Define the struct to match the JSON structure
type ProjectConfig struct {
	ProjectBaseDirectory string `json:"projectBaseDirectory"`
	NatsCredentialsFile  string `json:"natsCredentialsFile"`
	TableName            string `json:"tableName"`
	ClusterName          string `json:"clusterName"`
	Service              struct {
		Name            string `json:"name"`
		LogGroup        string `json:"logGroup"`
		CPU             int    `json:"cpu"`
		Memory          int    `json:"memory"`
		LogStreamPrefix string `json:"logStreamPrefix"`
	}
}

func loadConfig(filename string) (*ProjectConfig, error) {
	// Open the JSON file
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read the file contents
	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	// Unmarshal the JSON data into the struct
	var config ProjectConfig
	err = json.Unmarshal(bytes, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
