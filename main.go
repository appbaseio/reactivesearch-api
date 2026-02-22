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
	_ "net/http/pprof"
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
	"github.com/appbaseio/reactivesearch-api/model/requestlogs"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/plugins/analytics"
	"github.com/appbaseio/reactivesearch-api/plugins/analyticsrequest"
	"github.com/appbaseio/reactivesearch-api/plugins/applycache"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/cache"
	"github.com/appbaseio/reactivesearch-api/plugins/elasticsearch"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/nodes"
	"github.com/appbaseio/reactivesearch-api/plugins/openai"
	"github.com/appbaseio/reactivesearch-api/plugins/permissions"
	"github.com/appbaseio/reactivesearch-api/plugins/pipelines"
	"github.com/appbaseio/reactivesearch-api/plugins/preferences"
	"github.com/appbaseio/reactivesearch-api/plugins/proxy"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/reindexer"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	"github.com/appbaseio/reactivesearch-api/plugins/searchgrader"
	"github.com/appbaseio/reactivesearch-api/plugins/searchrelevancy"
	"github.com/appbaseio/reactivesearch-api/plugins/storedquery"
	"github.com/appbaseio/reactivesearch-api/plugins/suggestions"
	"github.com/appbaseio/reactivesearch-api/plugins/sync"
	"github.com/appbaseio/reactivesearch-api/plugins/synonyms"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/plugins/uibuilder"
	"github.com/appbaseio/reactivesearch-api/plugins/users"
	"github.com/appbaseio/reactivesearch-api/plugins/zinc"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/denisbrodbeck/machineid"
	"github.com/gorilla/mux"
	"github.com/keygen-sh/keygen-go"
	"github.com/mackerelio/go-osstat/memory"
	"github.com/pkg/profile"
	"github.com/robfig/cron"

	log "github.com/sirupsen/logrus"
	muxtrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/gorilla/mux"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

const logTag = "[cmd]"

var (
	envFile            string
	logMode            string
	licenseKeyPath     string
	listPlugins        bool
	address            string
	port               int
	pluginDir          string
	https              bool
	cpuprofile         bool
	memprofile         bool
	enableTelemetry    string
	disableHealthCheck bool
	showVersion        bool
	createSchema       bool
	enableProfiling    bool
	enableDiffing      bool
	enableDdTracing    bool
	enableLogging      bool
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
	// FeatureOpenAI for testing
	FeatureOpenAI string
)

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
	flag.BoolVar(&enableProfiling, "profiling", false, "enable route for pprof through the API")
	flag.BoolVar(&enableDiffing, "diff-logs", false, "Store logs by calculating the diff instead of storing the raw body")
	flag.BoolVar(&enableDdTracing, "enable-dd-tracing", false, "Enable tracing for Data Dog. Defaults to `false`.")
	flag.BoolVar(&enableLogging, "enable-logs", true, "Enables HTTP logging of network requests. Set this to false to take advantage of significant memory optimization")
	flag.Parse()

	// If showVersion is passed, show the version and do
	// nothing.
	util.Version = Version

	if showVersion {
		fmt.Printf("ReactiveSearch v%s\n", util.Version)
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

	// Load all env vars from envFile
	if err := LoadEnvFromFile(envFile); err != nil {
		fmt.Println(logTag, ": reading env file", envFile, ". This may happen if the environments are declared directly : ", err)
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

	// Set tracing enabled flag for global access
	util.SetTracingEnabled(enableDdTracing)

	// Read the env values related to tracing and set them
	// accordingly.
	tracingServiceName := os.Getenv("DD_SERVICE")
	if tracingServiceName != "" {
		util.SetServiceName(tracingServiceName)
	} else {
		util.SetServiceName("reactivesearch-api")
	}

	tracingEnv := os.Getenv("DD_ENV")
	if tracingEnv != "" {
		util.SetTracingEnv(tracingEnv)
	} else {
		util.SetTracingEnv("prod")
	}
}

func main() {
	if enableDdTracing {

		tracer.Start(
			tracer.WithService(util.GetServiceName()),
			tracer.WithEnv(util.GetTracingEnv()),
		)
		defer tracer.Stop()
	}

	// add cpu profiling
	if cpuprofile {
		defer profile.Start(profile.CPUProfile, profile.NoShutdownHook).Stop()
	}
	// add mem profiling
	if memprofile {
		defer profile.Start(profile.MemProfile, profile.NoShutdownHook).Stop()
	}

	// Set the enable-diffing value in environments
	shouldEnableDiffing := "false"
	if enableDiffing {
		shouldEnableDiffing = "true"
	}
	os.Setenv("SHOULD_ENABLE_DIFFING", shouldEnableDiffing)

	// Set the disable-logging value in environments
	shouldDisableLogging := "false"
	if !enableLogging {
		shouldDisableLogging = "true"
	}
	os.Setenv("SHOULD_DISABLE_LOGGING", shouldDisableLogging)

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

	var mainRouter *mux.Router
	var muxTraceRouter *muxtrace.Router
	var muxRouter *mux.Router

	if enableDdTracing {
		log.Infoln(logTag, ": Enabling DataDog tracing. If this is not intentional, remove the `--enable-dd-tracing` flag or set it to `false`.")
		muxTraceRouter = muxtrace.NewRouter().StrictSlash(true)
		mainRouter = muxTraceRouter.PathPrefix("").Subrouter()
	} else {
		muxRouter = mux.NewRouter().StrictSlash(true)
		mainRouter = muxRouter.PathPrefix("").Subrouter()
	}

	if enableProfiling {
		log.Debugln(logTag, ": enabling route for profiling since flag is passed as true")
		mainRouter.PathPrefix("/debug/pprof").Handler(http.DefaultServeMux)
	}

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
	util.Opensource = Opensource

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
			log.Fatalln("License key is not genuine, please contact support@reactivesearch.io")
			return
		case err != nil:
			log.Fatalln("License key validation failed, please contact support@reactivesearch.io", err.Error())
			return
		}
		// Validate expiry date for genuine license
		type LicenseDetails struct {
			Created string `json:"created"`
			Expiry  string `json:"expiry"`
		}
		type LicenseData struct {
			License LicenseDetails `json:"license"`
		}
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
	} else if Billing == "true" || HostedBilling == "true" || ClusterBilling == "true" {
		// Fetch the plan limits (requires ACCAPI)
		fetchErr := util.FetchArcLimitsPerPlan()
		if fetchErr != nil {
			errMsg := fmt.Sprint("error while fetching plan limits: ", fetchErr.Error())
			log.Fatal(logTag, ": ", errMsg)
		}

		// Add a cronjob to fetch plan limits
		cronJob := cron.New()
		cronJob.AddFunc("@every 24h", func() {
			limitFetchErr := util.FetchArcLimitsPerPlan()
			if limitFetchErr != nil {
				log.Errorln(logTag, ": error while fetching plan limits: ", limitFetchErr.Error())
			}
		})
		cronJob.Start()

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
	if FeatureOpenAI == "true" {
		util.SetFeatureOpenAI(true)
	}
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
	http.DefaultClient.Timeout = time.Second * time.Duration(util.GetHTTPTimeout())

	// map of specific plugins
	sequencedPlugins := []string{"auth.so", "logs.so", "nodes.so", "openai.so", "permissions.so", "preferences.so", "proxy.so", "reindexer.so", "searchgrader.so", "sync.so", "synonyms.so", "telemetry.so", "uibuilder.so", "users.so", "zinc.so", "analytics.so", "searchrelevancy.so", "rules.so", "cache.so", "suggestions.so", "storedquery.so", "analyticsrequest.so", "applycache.so"}

	elasticSearchMiddleware := make([]middleware.Middleware, 0)
	reactiveSearchMiddleware := make([]middleware.Middleware, 0)
	pipelinePlugin := pipelines.Instance()
	err3 := plugins.LoadPlugin(mainRouter, pipelinePlugin)
	if err3 != nil {
		log.Fatal("error loading pipeline routes: ", err3)
	}

	// load plugins in a sequence
	for _, pluginName := range sequencedPlugins {
		pluginInstance := getPlugin(pluginName)
		if pluginInstance != nil {
			err3 := plugins.LoadPlugin(mainRouter, pluginInstance)
			if err3 != nil {
				log.Fatal("error loading pipeline routes: ", err3)
			}
			elasticSearchMiddleware = append(elasticSearchMiddleware, pluginInstance.ESMiddleware()...)
			reactiveSearchMiddleware = append(reactiveSearchMiddleware, pluginInstance.RSMiddleware()...)
		}
	}
	// Load ReactiveSearch plugin
	rsPlugin := querytranslate.Instance()
	errRSPlugin := plugins.LoadRSPlugin(mainRouter, rsPlugin, reactiveSearchMiddleware)
	if errRSPlugin != nil {
		log.Fatal("error loading reactivesearch plugin: ", errRSPlugin)
	}
	// Load ES plugin
	esPlugin := elasticsearch.Instance()
	errESPlugin := plugins.LoadESPlugin(mainRouter, esPlugin, elasticSearchMiddleware)
	if errESPlugin != nil {
		log.Fatal("error loading elasticsearch plugin: ", errESPlugin)
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
	routerSwapper.SetDDTracing(enableDdTracing)

	if enableDdTracing {
		routerSwapper.SwapTrace(muxTraceRouter)
	} else {
		routerSwapper.Swap(muxRouter)
	}

	routerSwapper.SetRouterAttrs(address, port, https)

	// Set the router health check
	//
	// NOTE: The folowing code should be run just before the
	// server starts.
	// In other words, the server should start withing 10 seconds
	// of running the below code.
	log.Info(logTag, ": setting up router health check")
	routerHealthCheck := plugins.RouterHealthCheckInstance()
	routerHealthCheck.SetAttrs(port, address, https)
	if !disableHealthCheck {
		log.Info(logTag, ": setting up router health check cronjob")
		routerHealthCronJob := cron.New()
		routerHealthCronJob.AddFunc("@every 10s", routerHealthCheck.Check)
		routerHealthCronJob.Start()
	}

	// Start the job to keep pinging ES to mark as live node
	log.Info(logTag, ": setting up active node ping jobs")
	nodeInstance := nodes.Instance()
	nodeInstance.StartAutomatedJobs()

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
	// TODO: Handle es6
	// Fetch ES response
	response, err := util.GetClient7().
		Search(indexToSearch).
		Size(10000).
		Do(context.Background())
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

func getPlugin(plugin string) plugins.Plugin {
	switch plugin {
	case "auth.so":
		return auth.Instance()
	case "logs.so":
		return logs.Instance()
	case "nodes.so":
		return nodes.Instance()
	case "openai.so":
		return openai.Instance()
	case "permissions.so":
		return permissions.Instance()
	case "preferences.so":
		return preferences.Instance()
	case "proxy.so":
		return proxy.Instance()
	case "reindexer.so":
		return reindexer.Instance()
	case "searchgrader.so":
		return searchgrader.Instance()
	case "sync.so":
		return sync.Instance()
	case "synonyms.so":
		return synonyms.Instance()
	case "telemetry.so":
		return telemetry.Instance()
	case "uibuilder.so":
		return uibuilder.Instance()
	case "users.so":
		return users.Instance()
	case "zinc.so":
		return zinc.Instance()
	case "analytics.so":
		return analytics.Instance()
	case "searchrelevancy.so":
		return searchrelevancy.Instance()
	case "rules.so":
		return rules.Instance()
	case "cache.so":
		return cache.Instance()
	case "suggestions.so":
		return suggestions.Instance()
	case "storedquery.so":
		return storedquery.Instance()
	case "analyticsrequest.so":
		return analyticsrequest.Instance()
	case "applycache.so":
		return applycache.Instance()
	default:
		return nil
	}
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
