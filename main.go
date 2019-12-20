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
	"path/filepath"
	"plugin"
	"strconv"
	"strings"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/middleware/logger"
	"github.com/appbaseio/arc/plugins"
	"github.com/appbaseio/arc/util"
	"github.com/gorilla/mux"
	"github.com/robfig/cron"
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
	https       bool
	// PlanRefreshInterval can be used to define the custom interval to refresh the plan
	PlanRefreshInterval string
	// Billing is a build time flag
	Billing string
	// HostedBilling is a build time flag
	HostedBilling string
	// ClusterBilling is a build time flag
	ClusterBilling string
	// IgnoreBillingMiddleware ignores the billing middleware
	IgnoreBillingMiddleware string
)

func init() {
	flag.StringVar(&envFile, "env", ".env", "Path to file with environment variables to load in KEY=VALUE format")
	flag.StringVar(&logFile, "log", "", "Process log file")
	flag.BoolVar(&listPlugins, "plugins", false, "List currently registered plugins")
	flag.StringVar(&address, "addr", "", "Address to serve on")
	flag.IntVar(&port, "port", 8000, "Port number")
	flag.StringVar(&pluginDir, "pluginDir", "build/plugins", "Directory containing the compiled plugins")
	flag.BoolVar(&https, "https", false, "Starts a https server instead of a http server if true")
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

	if PlanRefreshInterval == "" {
		PlanRefreshInterval = "1"
	} else {
		_, err := strconv.Atoi(PlanRefreshInterval)
		if err != nil {
			log.Fatal("PLAN_REFRESH_INTERVAL must be an integer")
		}
	}

	interval := "@every " + PlanRefreshInterval + "h"

	util.Billing = Billing
	util.HostedBilling = HostedBilling
	util.ClusterBilling = ClusterBilling

	if Billing == "true" {
		log.Println("You're running Arc with billing module enabled.")
		util.ReportUsage()
		cronjob := cron.New()
		cronjob.AddFunc(interval, util.ReportUsage)
		cronjob.Start()
		if IgnoreBillingMiddleware != "true" {
			router.Use(util.BillingMiddleware)
		}
	} else if HostedBilling == "true" {
		log.Println("You're running Arc with hosted billing module enabled.")
		util.ReportHostedArcUsage()
		cronjob := cron.New()
		cronjob.AddFunc(interval, util.ReportHostedArcUsage)
		cronjob.Start()
		if IgnoreBillingMiddleware != "true" {
			router.Use(util.BillingMiddleware)
		}
	} else if ClusterBilling == "true" {
		log.Println("You're running Arc with cluster billing module enabled.")
		util.SetClusterPlan()
		// refresh plan
		cronjob := cron.New()
		cronjob.AddFunc(interval, util.SetClusterPlan)
		cronjob.Start()
		if IgnoreBillingMiddleware != "true" {
			router.Use(util.BillingMiddleware)
		}
	} else {
		var plan = util.ArcEnterprise
		util.Tier = &plan
		log.Println("You're running Arc with billing module disabled.")
	}

	// ES client instantiation
	// ES v7 and v6 clients
	util.NewClient()

	var elasticSearchPath string
	elasticSearchMiddleware := make([]middleware.Middleware, 0)
	err := filepath.Walk(pluginDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(info.Name()) == ".so" && info.Name() != "elasticsearch.so" {
			mw, err1 := LoadPluginFromFile(router, path)
			if err1 != nil {
				return err1
			}
			elasticSearchMiddleware = append(elasticSearchMiddleware, mw...)
		} else if info.Name() == "elasticsearch.so" {
			elasticSearchPath = path
		}
		return nil
	})
	LoadESPluginFromFile(router, elasticSearchPath, elasticSearchMiddleware)
	if err != nil {
		log.Fatal("error loading plugins: ", err)
	}

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
	if https {
		httpsCert := os.Getenv("HTTPS_CERT")
		httpsKey := os.Getenv("HTTPS_KEY")
		log.Fatal(http.ListenAndServeTLS(addr, httpsCert, httpsKey, handler))
	} else {
		log.Fatal(http.ListenAndServe(addr, handler))
	}
}

func LoadPIFromFile(path string) (plugin.Symbol, error) {
	pf, err1 := plugin.Open(path)
	if err1 != nil {
		return nil, err1
	}
	return pf.Lookup("PluginInstance")
}

// LoadPluginFromFile loads a plugin at the given location
func LoadPluginFromFile(router *mux.Router, path string) ([]middleware.Middleware, error) {
	pi, err2 := LoadPIFromFile(path)
	if err2 != nil {
		return nil, err2
	}
	var p plugins.Plugin
	p = *pi.(*plugins.Plugin)
	err3 := plugins.LoadPlugin(router, p)
	if err3 != nil {
		return nil, err3
	}
	return p.ESMiddleware(), nil
}

func LoadESPluginFromFile(router *mux.Router, path string, mw []middleware.Middleware) error {
	pi, err2 := LoadPIFromFile(path)
	if err2 != nil {
		return err2
	}
	var p plugins.ESPlugin
	p = *pi.(*plugins.ESPlugin)
	return plugins.LoadESPlugin(router, p, mw)
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
