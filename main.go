package main

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"plugin"
	"runtime"
	"strconv"
	"strings"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/logger"
	"github.com/appbaseio/reactivesearch-api/model/tracktime"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/denisbrodbeck/machineid"
	"github.com/gorilla/mux"
	"github.com/mackerelio/go-osstat/memory"
	"github.com/pkg/profile"
	"github.com/robfig/cron"
	"github.com/rs/cors"

	log "github.com/sirupsen/logrus"
)

const logTag = "[cmd]"

var (
	envFile         string
	logMode         string
	listPlugins     bool
	address         string
	port            int
	pluginDir       string
	https           bool
	cpuprofile      bool
	enableTelemetry string
	// Version Reactivesearch version set during build
	Version string
	// PlanRefreshInterval can be used to define the custom interval to refresh the plan
	PlanRefreshInterval string
	// Billing is a build time flag
	Billing string
	// HostedBilling is a build time flag
	HostedBilling string
	// ClusterBilling is a build time flag
	ClusterBilling string
	// Opensource is a build time flag
	Opensource string
	// IgnoreBillingMiddleware ignores the billing middleware
	IgnoreBillingMiddleware string
	// Tier for testing
	Tier string
	// FeatureCustomEvents for testing
	FeatureCustomEvents string
	// FeatureSuggestions for testing
	FeatureSuggestions string
	// FeatureRules for testing
	FeatureRules string
	// FeatureFunctions for testing
	FeatureFunctions string
	// FeatureSearchRelevancy for testing
	FeatureSearchRelevancy string
	// FeatureSearchGrader for testing
	FeatureSearchGrader string
	// FeatureEcommerce for testing
	FeatureEcommerce string
	// FeatureCache for testing
	FeatureCache string
)

func init() {
	flag.StringVar(&envFile, "env", ".env", "Path to file with environment variables to load in KEY=VALUE format")
	flag.StringVar(&logMode, "log", "", "Define to change the default log mode(error), other options are: debug(most verbose) and info")
	flag.BoolVar(&listPlugins, "plugins", false, "List currently registered plugins")
	flag.StringVar(&address, "addr", "0.0.0.0", "Address to serve on")
	// env port for deployments like heroku where port is dynamically assigned
	envPort := os.Getenv("PORT")
	defaultPort := 8000
	if envPort != "" {
		portValue, _ := strconv.Atoi(envPort)
		defaultPort = portValue
	}
	fmt.Println("=> port used", defaultPort)
	flag.IntVar(&port, "port", defaultPort, "Port number")
	flag.StringVar(&pluginDir, "pluginDir", "build/plugins", "Directory containing the compiled plugins")
	flag.BoolVar(&https, "https", false, "Starts a https server instead of a http server if true")
	flag.BoolVar(&cpuprofile, "cpuprofile", false, "write cpu profile to `file`")
	flag.StringVar(&enableTelemetry, "enable-telemetry", "", "Set as `false` to disable telemetry")
}

func main() {
	isRunTimeDocker := false

	// Summarizing how we're detecting a container runtime:
	// For Docker runtime, we check for the presence of `lxc` or `docker` or `kubepods` string in the output of /proc/1/cgroup, https://stackoverflow.com/a/23558932/1221677
	// For Podman (OCI) runtime, we check for the presence of /run/.containerenv, http://docs.podman.io/en/latest/markdown/podman-run.1.html
	cmdToDetectRunTime := exec.Command("/bin/sh", "-c", "if [[ -f /.dockerenv ]] || [[ -f /run/.containerenv ]] || grep -Eq '(lxc|docker|kubepods)' /proc/1/cgroup; then echo True; else echo False; fi")
	var output bytes.Buffer
	cmdToDetectRunTime.Stdout = &output
	runtimeDetectErr := cmdToDetectRunTime.Run()
	if runtimeDetectErr != nil {
		log.Fatal(logTag, ": Error encountered while detecting runtime :", runtimeDetectErr)
	}
	// True or False
	parsedOutput := strings.TrimSpace(output.String())
	if parsedOutput == "True" {
		isRunTimeDocker = true
	}

	if isRunTimeDocker {
		log.Println(logTag, "Runtime detected as docker or OCI container ...")
		cmd := exec.Command("/bin/sh", "-c", "head -1 /proc/self/cgroup|cut -d/ -f3")
		var out bytes.Buffer
		cmd.Stdout = &out
		err := cmd.Run()
		id := out.String()
		if err != nil {
			log.Fatal(logTag, ": runtime detected as docker or OCI container: ", err)
		}
		if id == "" {
			log.Fatal(logTag, ": runtime detected as docker or OCI container: machineid can not be empty")
		}
		h := hmac.New(sha256.New, []byte(strings.TrimSuffix(id, "\n")))
		h.Write([]byte("reactivesearch"))
		util.MachineID = hex.EncodeToString(h.Sum(nil))
		util.RunTime = "Docker"
	} else {
		log.Println(logTag, "Runtime detected as a host machine ...")
		id, err1 := machineid.ProtectedID("reactivesearch")
		if err1 != nil {
			log.Fatal(logTag, ": runtime detected as a host machine: ", err1)
		}
		util.MachineID = id
		util.RunTime = "Linux"
	}

	memory, memErr := memory.Get()
	if memErr != nil {
		log.Warnln(logTag, ":", memErr)
	} else {
		util.MemoryAllocated = memory.Total
	}

	flag.Parse()
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:          true,
		TimestampFormat:        "2006/01/02 15:04:05",
		DisableLevelTruncation: true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) {
			filename := path.Base(f.File)
			return "", fmt.Sprintf(" %s:%d", filename, f.Line)
		},
	})

	// add cpu profilling
	if cpuprofile {
		defer profile.Start().Stop()
	}

	switch logMode {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	default:
		log.SetLevel(log.ErrorLevel)
	}

	// Load all env vars from envFile
	if err := LoadEnvFromFile(envFile); err != nil {
		log.Infoln(logTag, ": reading env file", envFile, ". This may happen if the environments are declared directly : ", err)
	}

	router := mux.NewRouter().StrictSlash(true)

	if PlanRefreshInterval == "" {
		PlanRefreshInterval = "1"
	} else {
		_, err := strconv.Atoi(PlanRefreshInterval)
		if err != nil {
			log.Fatal("PLAN_REFRESH_INTERVAL must be an integer: ", err)
		}
	}

	interval := "@every " + PlanRefreshInterval + "h"

	util.Billing = Billing
	util.HostedBilling = HostedBilling
	util.ClusterBilling = ClusterBilling
	util.Opensource = Opensource
	util.Version = Version

	if Billing == "true" {
		log.Println("You're running ReactiveSearch with billing module enabled.")
		util.ReportUsage()
		cronjob := cron.New()
		cronjob.AddFunc(interval, util.ReportUsage)
		cronjob.Start()
		if IgnoreBillingMiddleware != "true" {
			router.Use(util.BillingMiddleware)
		}
	} else if HostedBilling == "true" {
		log.Println("You're running ReactiveSearch with hosted billing module enabled.")
		util.ReportHostedArcUsage()
		cronjob := cron.New()
		cronjob.AddFunc(interval, util.ReportHostedArcUsage)
		cronjob.Start()
		if IgnoreBillingMiddleware != "true" {
			router.Use(util.BillingMiddleware)
		}
	} else if ClusterBilling == "true" {
		log.Println("You're running ReactiveSearch with cluster billing module enabled.")
		util.SetClusterPlan()
		// refresh plan
		cronjob := cron.New()
		cronjob.AddFunc(interval, util.SetClusterPlan)
		cronjob.Start()
		if IgnoreBillingMiddleware != "true" {
			router.Use(util.BillingMiddleware)
		}
	} else {
		util.SetDefaultTier()
		log.Println("You're running ReactiveSearch with billing module disabled.")
	}

	// Testing Env: Set variables based on the build blags
	if Tier != "" {
		var temp1 = map[string]interface{}{
			"tier": Tier,
		}
		type Temp struct {
			Tier *util.Plan `json:"tier"`
		}
		temp2 := Temp{}
		mashalled, err := json.Marshal(temp1)
		if err != nil {
			log.Fatal(err)
		}
		err = json.Unmarshal(mashalled, &temp2)
		if err != nil {
			log.Fatal(err)
		}
		util.SetTier(temp2.Tier)
	}
	if FeatureCustomEvents == "true" {
		util.SetFeatureCustomEvents(true)
	}
	if FeatureSuggestions == "true" {
		util.SetFeatureSuggestions(true)
	}
	if FeatureRules == "true" {
		util.SetFeatureRules(true)
	}
	if FeatureFunctions == "true" {
		util.SetFeatureFunctions(true)
	}
	if FeatureSearchRelevancy == "true" {
		util.SetFeatureSearchRelevancy(true)
	}
	if FeatureSearchGrader == "true" {
		util.SetFeatureSearchGrader(true)
	}
	if FeatureEcommerce == "true" {
		util.SetFeatureEcommerce(true)
	}
	if FeatureCache == "true" {
		util.SetFeatureCache(true)
	}
	// Set port variable
	util.Port = port

	// ES client instantiation
	// ES v7 and v6 clients
	util.NewClient()
	util.SetDefaultIndexTemplate()
	util.SetSystemIndexTemplate()
	// map of specific plugins
	sequencedPlugins := []string{"cache.so", "searchrelevancy.so", "rules.so", "functions.so", "storedquery.so", "analytics.so", "suggestions.so", "applycache.so"}
	sequencedPluginsByPath := make(map[string]string)

	var elasticSearchPath, reactiveSearchPath string
	elasticSearchMiddleware := make([]middleware.Middleware, 0)
	reactiveSearchMiddleware := make([]middleware.Middleware, 0)
	err := filepath.Walk(pluginDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(info.Name()) == ".so" && info.Name() != "elasticsearch.so" {
			if info.Name() != "querytranslate.so" {
				if util.IsExists(info.Name(), sequencedPlugins) {
					sequencedPluginsByPath[info.Name()] = path
				} else {
					plugin, err1 := LoadPluginFromFile(router, path)
					if err1 != nil {
						return err1
					}
					reactiveSearchMiddleware = append(reactiveSearchMiddleware, plugin.RSMiddleware()...)
					elasticSearchMiddleware = append(elasticSearchMiddleware, plugin.ESMiddleware()...)
				}
			} else {
				reactiveSearchPath = path
			}
		} else if info.Name() == "elasticsearch.so" {
			elasticSearchPath = path
		}
		return nil
	})
	// load plugins in a sequence
	for _, pluginName := range sequencedPlugins {
		path := sequencedPluginsByPath[pluginName]
		if path != "" {
			plugin, err := LoadPluginFromFile(router, path)
			if err != nil {
				log.Fatal("error loading plugins: ", err)
			}
			elasticSearchMiddleware = append(elasticSearchMiddleware, plugin.ESMiddleware()...)
			reactiveSearchMiddleware = append(reactiveSearchMiddleware, plugin.RSMiddleware()...)
		}
	}
	// Load ReactiveSearch plugin
	if reactiveSearchPath != "" {
		LoadRSPluginFromFile(router, reactiveSearchPath, reactiveSearchMiddleware)
	}
	LoadESPluginFromFile(router, elasticSearchPath, elasticSearchMiddleware)
	if err != nil {
		log.Fatal("error loading plugins: ", err)
	}

	// Execute the migration scripts
	for _, migration := range util.GetMigrationScripts() {
		shouldExecute, err := migration.ConditionCheck()
		if err != nil {
			log.Fatal(err.Message+": ", err.Err)
		}
		if shouldExecute {
			// Run the script
			if migration.IsAsync() {
				// execute the script in go routine(background) without affecting the init process
				go func() {
					err := migration.Script()
					if err != nil {
						log.Errorln(err.Message+": ", err.Err)
					}
				}()
			} else {
				// Sync scripts will cause the fatal error on failure
				err := migration.Script()
				if err != nil {
					log.Fatal(err.Message+": ", err.Err)
				}
			}
		}
	}
	// Set telemetry based on the user input
	// Runtime flag gets the highest priority
	telemetryEnvVar := os.Getenv("ENABLE_TELEMETRY")
	if enableTelemetry != "" {
		b, err := strconv.ParseBool(enableTelemetry)
		if err != nil {
			log.Fatal(logTag, ": runtime flag `enable-telemetry` must be boolean: ", err)
		}
		util.IsTelemetryEnabled = b
	} else if telemetryEnvVar != "" {
		b, err := strconv.ParseBool(telemetryEnvVar)
		if err != nil {
			log.Fatal(logTag, ": environment value `ENABLE_TELEMETRY` must be boolean: ", err)
		}
		util.IsTelemetryEnabled = b
	}
	if util.IsTelemetryEnabled {
		log.Println("Appbase Telemetry is enabled. You can disable it by setting the `enable-telemetry` runtime flag as `false`")
	}
	// CORS policy
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"HEAD", "GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
		ExposedHeaders: []string{"*"},
	})
	handler := c.Handler(router)
	// Add time tracker middleware
	handler = tracktime.Track(handler)
	// Add logger middleware
	handler = logger.Log(handler)

	// Listen and serve ...
	addr := fmt.Sprintf("%s:%d", address, port)
	log.Println(logTag, ":listening on", addr)
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
func LoadPluginFromFile(router *mux.Router, path string) (plugins.Plugin, error) {
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
	return p, nil
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

func LoadRSPluginFromFile(router *mux.Router, path string, mw []middleware.Middleware) error {
	pi, err2 := LoadPIFromFile(path)
	if err2 != nil {
		return err2
	}
	var p plugins.RSPlugin
	p = *pi.(*plugins.RSPlugin)
	return plugins.LoadRSPlugin(router, p, mw)
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
