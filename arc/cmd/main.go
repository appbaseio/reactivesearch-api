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
	"plugin"
	"path/filepath"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/middleware/logger"
	"github.com/gorilla/mux"
	"github.com/rs/cors"

	"gopkg.in/natefinch/lumberjack.v2"

)

const logTag = "[cmd]"

var (
	envFile     string
	logFile     string
	listPlugins bool
	address     string
	port        int
	pluginDir   string
)

func init() {
	flag.StringVar(&envFile, "env", ".env", "Path to file with environment variables to load in KEY=VALUE format")
	flag.StringVar(&logFile, "log", "", "Process log file")
	flag.BoolVar(&listPlugins, "plugins", false, "List currently registered plugins")
	flag.StringVar(&address, "addr", "", "Address to serve on")
	flag.IntVar(&port, "port", 8000, "Port number")
	flag.StringVar(&pluginDir, "pluginDir", "build/plugins", "Directory containing the compiled plugins")
}

func main() {
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)
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
		log.Printf("%s: reading env file %q: %v", logTag, envFile, err)
	}

	router := mux.NewRouter().StrictSlash(true)

	err := filepath.Walk(pluginDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() && filepath.Ext(info.Name()) == ".so" {
			pf, err1 := plugin.Open(path)
			if err1 != nil { return err1 }
			pi, err2 := pf.Lookup("PluginInstance")
			if err2 != nil { return err2 }
			err3 := arc.LoadPlugin(router, *pi.(*arc.Plugin))
			if err3 != nil { return err3 }
		}
		return nil
	})
	if err != nil { log.Fatal("error loading plugins: %v", err) }

	// CORS policy
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"HEAD", "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	})
	handler := c.Handler(router)
	handler = logger.Log(handler)

	// Listen and serve ...
	addr := fmt.Sprintf("%s:%d", address, port)
	log.Printf("%s: listening on %s", logTag, addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

// LoadEnvFromFile loads env vars from envFile. Envs in the file
// should be in KEY=VALUE format.
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

// ParseEnvFile parses the envFile for env variables in present in
// KEY=VALUE format. It ignores the comment lines starting with "#".
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
