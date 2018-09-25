package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/gorilla/mux"
	"gopkg.in/natefinch/lumberjack.v2"

	_ "github.com/appbaseio-confidential/arc/plugins/permissions"
	_ "github.com/appbaseio-confidential/arc/plugins/users"

	// TODO: Currently this plugin needs to load last in order to correctly register path prefix
	_ "github.com/appbaseio-confidential/arc/plugins/es"
)

var (
	envFile string
	logFile string
	plugins bool
)

func init() {
	flag.StringVar(&envFile, "env", ".env", "Path to file with environment variables to load in KEY=VALUE format")
	flag.StringVar(&logFile, "log", "", "Process log file")
	flag.BoolVar(&plugins, "plugins", false, "List currently registered plugins")
}

func main() {
	flag.Parse()

	switch logFile {
	case "stdout":
		log.SetOutput(os.Stdout)
	case "stderr":
		log.SetOutput(os.Stderr)
	case "":
		log.SetOutput(ioutil.Discard)
	default:
		log.SetOutput(&lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    100,
			MaxAge:     14,
			MaxBackups: 10,
		})
	}

	// Load all env vars from envFile
	if err := LoadEnvFromFile(envFile); err != nil {
		log.Fatalf("[ERROR]: reading env file %q: %v", envFile, err)
	}
	router := mux.NewRouter().StrictSlash(true)
	for _, plugin := range arc.ListPlugins() {
		if err := arc.LoadPlugin(router, plugin); err != nil {
			log.Fatalf("%v", err)
		}
	}

	if plugins {
		fmt.Println(arc.ListPluginsStr())
	}

	log.Printf("[INFO]: listening on localhost:8000")
	log.Fatal(http.ListenAndServe(":8000", router))
}

func LoadEnvFromFile(envFile string) error {
	if envFile == "" {
		return nil
	}

	file, err := os.Open(envFile)
	if err != nil {
		return err
	}
	defer file.Close()

	envMap, err := ParseEnvFile(file)
	if err != nil {
		return err
	}

	for k, v := range envMap {
		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}

	return nil
}

func ParseEnvFile(envFile io.Reader) (map[string]string, error) {
	envMap := make(map[string]string)

	scanner := bufio.NewScanner(envFile)
	var line string
	lineNumber := 0

	for scanner.Scan() {
		line = strings.TrimSpace(scanner.Text())
		lineNumber++

		// skip the lines starting with comment
		if strings.HasPrefix(line, "#") {
			continue
		}

		// skip empty line
		if len(line) == 0 {
			continue
		}

		fields := strings.SplitN(line, "=", 2)
		if len(fields) != 2 {
			return nil, fmt.Errorf("can't parse line %d; line should be in KEY=VALUE format", lineNumber)
		}

		// KEY should not contain any whitespaces
		if strings.Contains(fields[0], " ") {
			return nil, fmt.Errorf("can't parse line %d; KEY contains whitespace", lineNumber)
		}

		key := fields[0]
		value := fields[1]

		if key == "" {
			return nil, fmt.Errorf("can't parse line %d; KEY can't be empty string", lineNumber)
		}
		envMap[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return envMap, nil
}
