package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/model/console"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	consolePolyfill "go.kuoruan.net/v8go-polyfills/console"
)

type CronScriptResponse struct {
	RuleId          *string   `json:"ruleId"`
	InvokeTimestamp time.Time `json:"invokeTimestamp"`
	Took            int       `json:"took"`
	Reason          *string   `json:"reason,omitempty"`
	Code            int       `json:"code"`
}

type CronScriptRequest struct {
	Context *RuleContext `json:"context"`
}

type CronScriptOutput struct {
	Category   string              `json:"category"`
	Request    *CronScriptRequest  `json:"request"`
	Response   *CronScriptResponse `json:"response"`
	ScriptTook *int                `json:"scriptTook,omitempty"`
	Console    []string            `json:"console_logs,omitempty"`
}

// Run the cron rule based on the passed
// script details.
// Thie method is called either:
// - when cron rule is created/updated
// - when server restarts
func RunCronRule(r *Rules, ruleDetails ESRuleDoc) {
	for _, action := range *ruleDetails.Actions {
		// Create a rule context and pass the envs in there
		ruleContext := &RuleContext{
			Envs: action.Environments,
		}

		output, _ := r.runCronScriptWithTimeout(*action.DecodeScript, ruleContext, ruleDetails.Trigger.Type.Timeout())

		// Add rule ID before logging the output
		output.Response.RuleId = ruleDetails.ID

		// Log the output manually since log plugin won't pick it up.
		logCronOutput(*output)
	}
}

// Run the cron script with timeout
func (r *Rules) runCronScriptWithTimeout(script string, context *RuleContext, timeout time.Duration) (*CronScriptOutput, error) {
	start := time.Now()
	r.lock.Lock()
	defer r.lock.Unlock()

	// Create the response to return
	var output = new(CronScriptOutput)
	var responseOutput = new(CronScriptResponse)
	var requestOutput = new(CronScriptRequest)
	var consoleWriter = new(bytes.Buffer)

	// Inject console support to context
	consoleInjectErr := consolePolyfill.InjectTo(r.context, consolePolyfill.WithOutput(consoleWriter))
	if consoleInjectErr != nil {
		log.Warnln(logTag, "error while injecting console polyfill, ", consoleInjectErr)
	}

	// Set some default values
	output.Request = requestOutput
	output.Response = responseOutput
	output.Category = "cron"

	var errMsg string

	// Set invoke timestamp
	output.Response.InvokeTimestamp = time.Now()

	// If context is nil, make it an empty context
	if context == nil {
		context = &RuleContext{
			Envs: map[string]interface{}{},
		}
	}

	// Populate the request context field
	output.Request.Context = context

	// Parse context to string
	contextBytes, err := json.Marshal(context)
	if err != nil {
		errMsg = fmt.Sprintf("%s, %s", "error while marshalling context", err)
		log.Errorln(logTag, errMsg)

		output.Response.Reason = &errMsg
		output.Response.Code = http.StatusBadRequest
		return output, err
	}

	// Unique ID for script handler
	functionId := "script" + strconv.Itoa(int(time.Now().Unix()))

	_, scriptError := r.runScriptWithTimeout(fmt.Sprintf(`
	function %s(context) {
		%s;
	}`, functionId, script), functionId+".js", timeout)

	if scriptError != nil {
		errMsg = fmt.Sprintf("%s, %s", "error while creating script with passed data", scriptError)
		log.Errorln(logTag, errMsg)
		output.Response.Reason = &errMsg
		output.Response.Code = http.StatusInternalServerError
		return output, scriptError
	}

	// Invoke the script with the context
	scriptInvoke, scriptInvokeError := r.runScriptWithTimeout(fmt.Sprintf(`%s(%s)`, functionId, string(contextBytes)), "main.js", timeout)
	if scriptInvokeError != nil {
		errMsg = fmt.Sprintf("%s, %s", "error while running script, ", scriptInvokeError)
		log.Errorln(logTag, errMsg)

		output.Response.Reason = &errMsg
		output.Response.Code = http.StatusBadRequest
		return output, scriptInvokeError
	}

	// Try to parse the console data
	if scriptInvoke != nil {
		// Get the string from the writer, split it on \n
		consoleOutputStr := console.LimitConsoleString(consoleWriter.String())
		output.Console = strings.Split(consoleOutputStr, "\n")
	}

	// Parse rest of the data.
	output.Response.Took = int(time.Since(start).Milliseconds())
	output.Response.Code = http.StatusOK
	output.ScriptTook = &output.Response.Took

	return output, nil
}

// Log the cron output
func logCronOutput(cronOutput CronScriptOutput) {
	l := logs.Instance()
	lumberjackLogger := l.Lumberjack()

	// Remove the scriptTook field since logs don't need to contain
	// it.
	cronOutput.ScriptTook = nil

	// Marshal the cron output
	outputMarshalled, err := json.Marshal(cronOutput)
	if err != nil {
		log.Errorln(logTag, "error while marshalling cron output,", err)
		return
	}

	n, err := lumberjackLogger.Write(outputMarshalled)
	if err != nil {
		log.Errorln(logTag, "error encountered while writing logs :", err)
		return
	}
	// Add new line character so filebeat can sync it with ES
	lumberjackLogger.Write([]byte("\n"))
	log.Println(logTag, "logged request successfully", n)
}

// Init the cron job for the passed rule
func (r *Rules) initCronRule(ruleDetails ESRuleDoc) {
	log.Infoln(logTag, "initiating cron job for rule: ", *ruleDetails.ID)

	// Init the cron job
	cronjob := cron.New()
	cronjob.AddFunc(ruleDetails.Trigger.Expression, func() {
		RunCronRule(r, ruleDetails)
	})
	cronjob.Start()

	// Add the cronjob to keep track of them
	runningJob := RunningJob{
		RuleId:  ruleDetails.ID,
		CronJob: cronjob,
	}
	r.runningJobs = append(r.runningJobs, runningJob)
}

// Remove the cron rule based on the passed ID
func (r *Rules) removeCronRule(ruleId string) {
	// Delete the cron job if it is running
	for index, runningJob := range r.runningJobs {
		if *runningJob.RuleId == ruleId {
			log.Infoln(logTag, "stopping cron job for rule: ", ruleId)

			// Stop the cron job and delete it from running jobs
			runningJob.CronJob.Stop()

			// Remove the item at the passed index
			runningJobsLen := len(r.runningJobs)
			if runningJobsLen > 0 {
				r.runningJobs[index] = r.runningJobs[runningJobsLen-1]
				r.runningJobs = r.runningJobs[:runningJobsLen-1]
			}
		}
	}
}

// Initiate all the rules that are of type cron
func (r *Rules) initCronRules(rules []ESRuleDoc) {
	for _, rule := range rules {
		if *rule.Trigger.Type == Cron && *rule.Enabled {
			r.initCronRule(rule)
		}
	}
}
