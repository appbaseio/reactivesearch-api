package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"plugin"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/requestlogs"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/nodes"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/denisbrodbeck/machineid"
	"github.com/getsentry/sentry-go"
	"github.com/gorilla/mux"
	"github.com/keygen-sh/keygen-go"
	"github.com/mackerelio/go-osstat/memory"
	"github.com/pkg/profile"
	"github.com/robfig/cron"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

const logTag = "[cmd]"

var (
	envFile               string
	logMode               string
	licenseKeyPath        string
	listPlugins           bool
	address               string
	port                  int
	pluginDir             string
	https                 bool
	cpuprofile            bool
	memprofile            bool
	enableTelemetry       string
	disableHealthCheck    bool
	showVersion           bool
	createSchema          bool
	enableDevelopmentMode bool
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
	// MultiTenant is a build time flag
	MultiTenant string
	// Opensource is a build time flag
	Opensource string
	// ExternalElasticsearch is a build time flag that
	// indicates whether external Elasticsearch instance
	// is being used.
	ExternalElasticsearch string
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
	// FeatureSearchRelevancy for testing
	FeatureSearchRelevancy string
	// FeatureSearchGrader for testing
	FeatureSearchGrader string
	// FeatureEcommerce for testing
	FeatureEcommerce string
	// FeatureCache for testing
	FeatureCache string
	// FeaturePipelines for testing
	FeaturePipelines string
	// FeatureUIBuilderPremium for testing
	FeatureUIBuilderPremium string
)

// SentryErrorHook implements the logrus.Hooks interface to report errors to sentry
type SentryErrorHook struct {
}

// Report error logs for logs above or equal to error level
func (h *SentryErrorHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.ErrorLevel}
}

func (h *SentryErrorHook) Fire(e *logrus.Entry) error {
	// send event to sentry
	sentry.CaptureMessage(e.Message)
	return nil
}

type LicenseDetails struct {
	Created string `json:"created"`
	Expiry  string `json:"expiry"`
}
type LicenseData struct {
	License LicenseDetails `json:"license"`
}

func init() {
	flag.StringVar(&enableTelemetry, "enable-telemetry", "", "Set as `false` to disable telemetry")
	flag.StringVar(&envFile, "env", ".env", "Path to file with environment variables to load in KEY=VALUE format")
	flag.StringVar(&logMode, "log", "", "Define to change the default log mode(error), other options are: debug(most verbose) and info")
	flag.StringVar(&licenseKeyPath, "license-key-file", "", "Path to file with license key")
	flag.BoolVar(&listPlugins, "plugins", false, "List currently registered plugins")
	flag.StringVar(&address, "addr", "0.0.0.0", "Address to serve on")
	flag.BoolVar(&disableHealthCheck, "disable-health-check", false, "Set as `true` to disable health check")
	flag.BoolVar(&showVersion, "version", false, "show the version of ReactiveSearch")
	flag.BoolVar(&createSchema, "create-schema", false, "create the schema for the current version of API and exit")
	flag.BoolVar(&enableDevelopmentMode, "development", false, "Set as `true` to use development mode")

	// env port for deployments like heroku where port is dynamically assigned
	envPort := os.Getenv("PORT")
	defaultPort := 8000
	if envPort != "" {
		portValue, _ := strconv.Atoi(envPort)
		defaultPort = portValue
	}

	flag.IntVar(&port, "port", defaultPort, "Port number")
	flag.StringVar(&pluginDir, "pluginDir", "build/plugins", "Directory containing the compiled plugins")
	flag.BoolVar(&https, "https", false, "Starts a https server instead of a http server if true")
	flag.BoolVar(&cpuprofile, "cpuprofile", false, "write cpu profile to `file`")
	flag.BoolVar(&memprofile, "memprofile", false, "write mem profile to `file`")
	flag.Parse()

	if enableDevelopmentMode {
		util.IsDevelopmentEnv = true
	}

	// If showVersion is passed, show the version and do
	// nothing.
	util.Version = Version

	if showVersion {
		fmt.Println(fmt.Sprintf("ReactiveSearch v%s", util.Version))
		os.Exit(0)
	}

	// if createSchema is passed, create the schema
	if createSchema {
		createErr := CreateSchema(pluginDir)

		if createErr != nil {
			fmt.Println("error while creating schema: ", createErr)
			os.Exit(-1)
		}

		os.Exit(0)
	}

	fmt.Println("=> port used", defaultPort)

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
		// configure sentry
		dsn := "https://3b9b4fcedbf4460c90844f51e8634229@o27644.ingest.sentry.io/6063525"
		// Use prod dsn for customers
		if Billing == "true" || ClusterBilling == "true" || HostedBilling == "true" {
			dsn = "https://ecb33128f4514511b2ee7ecaf2e4e689@o27644.ingest.sentry.io/6125897"
		}
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              dsn,
			Release:          util.Version,
			AttachStacktrace: true,
			Debug:            true,
		})
		if err != nil {
			log.Fatalf("sentry.Init: %s", err)
		}

		defer func() {
			err := recover()
			if err != nil {
				sentry.CurrentHub().Recover(err)
				sentry.Flush(time.Second * 10)
			}
		}()
	}
}

func main() {
	// add cpu profilling
	if cpuprofile {
		defer profile.Start(profile.NoShutdownHook).Stop()
	}
	// add mem profilling
	if memprofile {
		defer profile.Start(profile.MemProfile).Stop()
	}

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
	if util.IsTelemetryEnabled {
		log.AddHook(&SentryErrorHook{})
	}
	switch logMode {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	default:
		log.SetLevel(log.ErrorLevel)
	}

	isRunTimeDocker := false

	// Summarizing how we're detecting a container runtime:
	// For Docker runtime, we check for the presence of `lxc` or `docker` or `kubepods` string in the output of /proc/1/cgroup, https://stackoverflow.com/a/23558932/1221677
	// For Podman (OCI) runtime, we check for the presence of /run/.containerenv, http://docs.podman.io/en/latest/markdown/podman-run.1.html
	// For Docker on Mac and several other non-linux runtimes, we check for INODE count > 2: https://stackoverflow.com/a/51688023/1221677
	cmdToDetectRunTime := exec.Command("/bin/sh", "-c", "if [[ -f /.dockerenv ]] || [[ -f /run/.containerenv ]] || [ `ls -ali / | sed '2!d' | awk {'print $1'}` != '2' ] || grep -Eq '(lxc|docker|kubepods)' /proc/1/cgroup; then echo True; else echo False; fi")
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

	// Load all env vars from envFile
	if err := LoadEnvFromFile(envFile); err != nil {
		log.Infoln(logTag, ": reading env file", envFile, ". This may happen if the environments are declared directly : ", err)
	}

	router := mux.NewRouter().StrictSlash(true)

	mainRouter := router.PathPrefix("").Subrouter()

	// default is hourly
	interval := "0 0 * * * *"

	if PlanRefreshInterval != "" {
		_, err := strconv.Atoi(PlanRefreshInterval)
		if err != nil {
			log.Fatal("PLAN_REFRESH_INTERVAL must be an integer: ", err)
		}
		interval = "0 0 0-59/" + PlanRefreshInterval + " * * *"
	}

	util.Billing = Billing
	util.HostedBilling = HostedBilling
	util.ClusterBilling = ClusterBilling
	if MultiTenant == "true" {
		util.MultiTenant = true
		if os.Getenv("REACTIVESEARCH_AUTH_TOKEN") == "" {
			log.Fatal("REACTIVESEARCH_AUTH_TOKEN is required in multi-tenant mode")
			return
		}
		if os.Getenv("DOMAIN_NAME_ENCRYPTION_KEY") == "" {
			log.Fatal("DOMAIN_NAME_ENCRYPTION_KEY is required in multi-tenant mode")
			return
		}
	}
	util.Opensource = Opensource
	util.Version = Version
	util.ExternalElasticsearch = ExternalElasticsearch

	var licenseKey string
	// check for offline license key
	if licenseKeyPath != "" {
		// read license key from file
		content, err := ioutil.ReadFile(licenseKeyPath)
		if err != nil {
			log.Fatalln(logTag, "Unable to read license file", err.Error())
		}
		licenseKey = string(content)
	} else {
		// read from env file
		licenseKey = os.Getenv("LICENSE_KEY")
	}
	if licenseKey != "" {
		util.OfflineBilling = true
		keygen.PublicKey = util.AppbasePublicKey
		// validate offline license key
		dataset, err := keygen.Genuine(licenseKey, keygen.SchemeCodeEd25519)
		switch {
		case err == keygen.ErrLicenseNotGenuine:
			log.Fatalln("License key is not genuine, please contact support@appbase.io")
			return
		case err != nil:
			log.Fatalln("License key validation failed, please contact support@appbase.io", err.Error())
			return
		}
		// Validate expiry date for genuine license
		var licenseData LicenseData
		err2 := json.Unmarshal(dataset, &licenseData)
		if err2 != nil {
			log.Fatalln(logTag, "Error encountered while reading the license details:", err2)
		}
		expiryTime, err := time.Parse(time.RFC3339, licenseData.License.Expiry)
		if err != nil {
			log.Fatalln(logTag, ":", err)
		}
		util.SetExpiryTime(expiryTime)
		util.SetDefaultTier()
		// use billing middleware
		if IgnoreBillingMiddleware != "true" {
			mainRouter.Use(util.BillingMiddlewareOffline)
		}
	} else {
		if util.MultiTenant {
			// Maintain SLS instance details
			log.Println("You're running ReactiveSearch with SLS multi-tenancy enabled.")
			util.UpdateSLSInstances()
			cronJob := cron.New()
			cronJob.AddFunc("@every 60s", util.UpdateSLSInstances)
			cronJob.Start()
			// Use validate domain middleware, it creates a context with domain
			mainRouter.Use(validate.ValidateDomain)
			if IgnoreBillingMiddleware != "true" {
				mainRouter.Use(util.BillingMiddleware)
			}
			// TODO: Set default backend as system
			util.SetDefaultBackend()

			// Set the system ES URL
			util.SetSystemESURL()
		} else {
			if Billing == "true" {
				log.Println("You're running ReactiveSearch with billing module enabled.")
				util.ReportUsage()
				cronJob := cron.New()
				cronJob.AddFunc(interval, util.ReportUsage)
				cronJob.Start()
				if IgnoreBillingMiddleware != "true" {
					mainRouter.Use(util.BillingMiddleware)
				}
			} else if HostedBilling == "true" {
				log.Println("You're running ReactiveSearch with hosted billing module enabled.")
				util.ReportHostedArcUsage()
				cronJob := cron.New()
				cronJob.AddFunc(interval, util.ReportHostedArcUsage)
				cronJob.Start()
				if IgnoreBillingMiddleware != "true" {
					mainRouter.Use(util.BillingMiddleware)
				}
			} else if ClusterBilling == "true" {
				log.Println("You're running ReactiveSearch with cluster billing module enabled.")
				util.SetClusterPlan()
				// refresh plan
				cronJob := cron.New()
				cronJob.AddFunc(interval, util.SetClusterPlan)
				cronJob.Start()
				if IgnoreBillingMiddleware != "true" {
					mainRouter.Use(util.BillingMiddleware)
				}
			} else {
				util.SetDefaultTier()
				util.SetDefaultBackend()
				log.Println("You're running ReactiveSearch with billing module disabled.")
			}
		}
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
	if FeatureSearchRelevancy == "true" {
		util.SetFeatureSearchRelevancy(true)
	}
	if FeatureSearchGrader == "true" {
		util.SetFeatureSearchGrader(true)
	}
	if FeatureEcommerce == "true" {
		util.SetFeatureEcommerce(true)
	}
	if FeatureUIBuilderPremium == "true" {
		util.SetFeatureUIBuilderPremium(true)
	}
	if FeatureCache == "true" {
		util.SetFeatureCache(true)
	}
	if FeaturePipelines == "true" {
		util.SetFeaturePipelines(true)
	}
	// Set Global Envs from env file
	util.SetGlobalESURL(os.Getenv(util.EsURLKey))
	util.SetGlobalESHeader(os.Getenv(util.EsHeaderKey))
	util.SetGlobalOSURL(os.Getenv(util.OsURLKey))
	util.SetGlobalOSHeader(os.Getenv(util.OsHeaderKey))
	// Set port variable
	util.Port = port

	// ES client instantiation
	// ES v7 and v6 clients
	util.NewClient()
	util.NewZincClient()
	util.SetDefaultIndexTemplate()
	util.SetSystemIndexTemplate()

	/*
	   Safety net for 'too many open files' issue on legacy code.
	   Set a sane timeout duration for the http.DefaultClient, to ensure idle connections are terminated.
	   Reference: https://stackoverflow.com/questions/37454236/net-http-server-too-many-open-files-error
	*/
	http.DefaultClient.Timeout = time.Minute * 2

	// map of specific plugins
	sequencedPlugins := []string{"analytics.so", "searchrelevancy.so", "rules.so", "cache.so", "suggestions.so", "storedquery.so", "analyticsrequest.so", "applycache.so"}
	sequencedPluginsByPath := make(map[string]string)

	var elasticSearchPath, reactiveSearchPath, pipelinesPath string
	elasticSearchMiddleware := make([]middleware.Middleware, 0)
	reactiveSearchMiddleware := make([]middleware.Middleware, 0)
	pluginsByPath := make(map[string]string)
	err := filepath.Walk(pluginDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() &&
			filepath.Ext(info.Name()) == ".so" &&
			info.Name() != "elasticsearch.so" &&
			info.Name() != "pipelines.so" {
			if info.Name() != "querytranslate.so" {
				if util.IsExists(info.Name(), sequencedPlugins) {
					sequencedPluginsByPath[info.Name()] = path
				} else {
					pluginsByPath[info.Name()] = path
				}
			} else {
				reactiveSearchPath = path
			}
		} else if info.Name() == "elasticsearch.so" {
			elasticSearchPath = path
		} else if info.Name() == "pipelines.so" {
			pipelinesPath = path
		}
		return nil
	})
	if err != nil {
		log.Fatal("error loading plugins: ", err)
	}
	// Load pipeline plugin at the beginning to set the priority to stage routes
	if pipelinesPath != "" {
		_, errPipelinesPlugin := LoadPluginFromFile(mainRouter, pipelinesPath)
		if errPipelinesPlugin != nil {
			log.Fatal("error loading plugins: ", errPipelinesPlugin)
		}
	}

	// Initiate the external ES client
	if util.IsSLSEnabled() {
		util.InitExternalESClient7()
	}

	for _, pluginPath := range pluginsByPath {
		plugin, err1 := LoadPluginFromFile(mainRouter, pluginPath)
		if err1 != nil {
			log.Fatal("error loading plugins: ", err1)
		}
		reactiveSearchMiddleware = append(reactiveSearchMiddleware, plugin.RSMiddleware()...)
		elasticSearchMiddleware = append(elasticSearchMiddleware, plugin.ESMiddleware()...)
	}
	// load plugins in a sequence
	for _, pluginName := range sequencedPlugins {
		path := sequencedPluginsByPath[pluginName]
		if path != "" {
			plugin, err := LoadPluginFromFile(mainRouter, path)
			if err != nil {
				log.Fatal("error loading plugins: ", err)
			}
			elasticSearchMiddleware = append(elasticSearchMiddleware, plugin.ESMiddleware()...)
			reactiveSearchMiddleware = append(reactiveSearchMiddleware, plugin.RSMiddleware()...)
		}
	}
	// Load ReactiveSearch plugin
	if reactiveSearchPath != "" {
		errRSPlugin := LoadRSPluginFromFile(mainRouter, reactiveSearchPath, reactiveSearchMiddleware)
		if errRSPlugin != nil {
			log.Fatal("error loading plugins: ", errRSPlugin)
		}
	}

	errESPlugin := LoadESPluginFromFile(mainRouter, elasticSearchPath, elasticSearchMiddleware)
	if errESPlugin != nil {
		log.Fatal("error loading plugins: ", errESPlugin)
	}

	// Execute the migration scripts
	for _, migration := range util.GetMigrationScripts() {
		shouldExecute, err := migration.ConditionCheck()
		if err != nil {
			log.Errorln(err.Message+": ", err.Err)
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
					log.Errorln(err.Message+": ", err.Err)
				}
			}
		}
	}

	// Initialize request logs map
	requestlogs.InitRequestLogs(60000, 1*60)

	cronjob := cron.New()
	syncInterval := "@every " + strconv.Itoa(util.GetSyncInterval()) + "s"
	cronjob.AddFunc(syncInterval, syncPluginCache)
	cronjob.Start()

	// Set the router in the swapper
	routerSwapper := plugins.RouterSwapperInstance()
	routerSwapper.Swap(router)
	routerSwapper.SetRouterAttrs(address, port, https)

	// Set the router health check
	//
	// NOTE: The folowing code should be run just before the
	// server starts.
	// In other words, the server should start withing 10 seconds
	// of running the below code.
	if !disableHealthCheck {
		log.Info(logTag, ": setting up router health check")
		routerHealthCheck := plugins.RouterHealthCheckInstance()
		routerHealthCheck.SetAttrs(port, address, https)
		routerHealthCronJob := cron.New()
		routerHealthCronJob.AddFunc("@every 10s", routerHealthCheck.Check)
		routerHealthCronJob.Start()
	}

	// Start the job to keep pinging ES to mark as live node
	log.Info(logTag, ": setting up active node ping jobs")
	if !util.MultiTenant {
		nodeInstance := nodes.Instance()
		nodeInstance.StartAutomatedJobs()
	}

	// Finally start the server
	routerSwapper.StartServer()
}

func syncPluginCache() {
	// Only run for self hosted arc using arc-enterprise plan

	indices := []string{}
	for _, syncScript := range util.GetSyncScripts() {
		// append index
		indices = append(indices, syncScript.Index())
	}
	indexToSearch := strings.Join(indices, ",")

	// Fetch ES response
	//
	// We need to make sure that different tenant's data stays
	// separate so the request needs to be through the util
	// methods for the tenant.
	response, err := util.SearchServiceWithAuth(util.GetInternalClient7().
		Search(indexToSearch).
		Size(10000), nil).Do(context.Background())

	if err != nil {
		log.Errorln(logTag, "Error while syncing plugin cache", err.Error())
		return
	}
	if response.Error != nil {
		log.Errorln(logTag, "Error while syncing plugin cache", response.Error)
		return
	}
	for _, syncScript := range util.GetSyncScripts() {
		err := syncScript.SetCache(response)
		if err != nil {
			log.Errorln(logTag, "Error syncing plugin "+syncScript.PluginName()+" ", response.Error)
		}
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

// CreateSchema will create a file in the current directory
// and save it in the format
// schema/latest/schema.json
func CreateSchema(pluginDir string) error {
	// Create the directory in the current directory.
	// Ignore if already created.
	pathToCreate := filepath.Join("schema", "latest")
	dirCreateErr := os.MkdirAll(pathToCreate, os.ModePerm)

	if dirCreateErr != nil {
		return dirCreateErr
	}

	// Since the directory is created, write the contents into the file
	// now.
	schemaContent, schemaErr := querytranslate.GetReactiveSearchSchema()

	if schemaErr != nil {
		return schemaErr
	}

	// Unmarshal the schema into a temporary map and the marshal it
	// again with indentation
	tempMap := make(map[string]interface{})

	unmarshalErr := json.Unmarshal(schemaContent, &tempMap)
	if unmarshalErr != nil {
		return fmt.Errorf("error while unmarshalling the RS API schema to indent it before writing: %v", unmarshalErr)
	}

	// Marshal it back again with indentation
	writableBytes, indentErr := json.MarshalIndent(tempMap, "", "  ")
	if indentErr != nil {
		return fmt.Errorf("error while marshaling the RS API schema with indentation: %v", indentErr)
	}

	// Create the oss schema
	createSchemaErr := ioutil.WriteFile(filepath.Join(pathToCreate, "schema.json"), writableBytes, 0644)
	if createSchemaErr != nil {
		return createSchemaErr
	}

	// Set the util flag so it's used in noss code.
	util.CreateSchema = true

	// Load the plugin and run initFunc for the schema to be created
	// for pipelines.

	pipelinePath := filepath.Join(pluginDir, "pipelines.so")

	// Check if plugin exists, if not, then skip creating that schema
	_, checkErr := os.Stat(pipelinePath)
	if os.IsNotExist(checkErr) {
		return nil
	}

	// Path exists and we need to create the pipeline schema
	pi, err2 := LoadPIFromFile(pipelinePath)
	if err2 != nil {
		return err2
	}
	var p plugins.Plugin
	p = *pi.(*plugins.Plugin)

	return p.InitFunc()
}
