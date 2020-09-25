// Copyright 2020 Lennart Espe
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var (
	Version     = "0.1-dev"
	ShowVersion = flag.Bool("version", false, "Display version information")
	DebugMode   = flag.Bool("debug", false, "Set log level to DEBUG")
	ConfigPath  = flag.String("config", "config.yaml", "Path to look for configuration file")
)

var (
	logger = logrus.New()
)

func run(ctx context.Context) error {
	// Parse the YAML configuration file
	configData, err := ioutil.ReadFile(*ConfigPath)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	// Start up a goroutine for each application
	var wg sync.WaitGroup
	for name, app := range config.Application {
		wg.Add(1)
		pp := &Proxy{
			Name:       name,
			Addr:       app.Addr,
			MaxRetries: app.MaxRetries,
			LB:         &RoundRobin{Origins: app.Origins},
		}
		go func() { pp.Listen(ctx); wg.Done() }()
	}
	wg.Wait()
	return nil
}

func main() {
	flag.Parse()
	if *ShowVersion {
		fmt.Println(Version)
		os.Exit(0)
	}
	// Setup logging
	logger.Formatter = &logrus.TextFormatter{
		TimestampFormat: time.RFC3339Nano,
	}
	if *DebugMode {
		logger.Level = logrus.DebugLevel
	}
	// Setup shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		logrus.WithField("signal", sig.String()).Debug("Received shutdown signal")
		cancel()
	}()
	// Start up proxy
	if err := run(ctx); err != nil {
		logrus.WithError(err).Fatal("Proxy shutdown unexpectedly")
	}
}
