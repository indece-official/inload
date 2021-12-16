package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/indece-official/loadtest/src/model"
	"github.com/indece-official/loadtest/src/report"
	"github.com/indece-official/loadtest/src/stats"
	"github.com/robertkrimen/otto"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Variables set during build
var (
	ProjectName  string
	BuildVersion string
	BuildDate    string
)

var flagVerbose = flag.Bool("v", false, "Verbose")
var flagFile = flag.String("f", "", "Filename of test yaml")

func loadConfig() (*model.Config, error) {
	if *flagFile == "" {
		return nil, fmt.Errorf("missing filename of test yaml (-f <filename>)")
	}

	data, err := ioutil.ReadFile(*flagFile)
	if err != nil {
		return nil, fmt.Errorf("can't read file %s: %s", *flagFile, err)
	}

	config := &model.Config{}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("can't parse yaml file %s: %s", *flagFile, err)
	}

	return config, nil
}

func main() {
	flag.Parse()

	if *flagVerbose {
		log.SetLevel(log.DebugLevel)
	}

	// Load config
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %s", err)

		os.Exit(1)

		return
	}

	err = config.Validate()
	if err != nil {
		log.Fatalf("Invalid config file: %s", err)

		os.Exit(1)

		return
	}

	report := report.NewReport()
	runStats := stats.NewRunStats()
	vm := otto.New()

	log.Infof("Starting tests")

	err = config.Execute([]string{}, vm, runStats, report)
	if err != nil {
		log.Fatalf("Error running tests: %s", err)

		os.Exit(1)

		return
	}

	log.Infof("Successfully finished tests")

	log.Infof("")
	runStats.Print()
}
