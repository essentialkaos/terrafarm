package cli

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2016 Essential Kaos                         //
//      Essential Kaos Open Source License <http://essentialkaos.com/ekol?en>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"bufio"
	"crypto"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"pkg.re/essentialkaos/ek.v5/arg"
	"pkg.re/essentialkaos/ek.v5/env"
	"pkg.re/essentialkaos/ek.v5/fmtc"
	"pkg.re/essentialkaos/ek.v5/fmtutil"
	"pkg.re/essentialkaos/ek.v5/fsutil"
	"pkg.re/essentialkaos/ek.v5/jsonutil"
	"pkg.re/essentialkaos/ek.v5/log"
	"pkg.re/essentialkaos/ek.v5/path"
	"pkg.re/essentialkaos/ek.v5/pluralize"
	"pkg.re/essentialkaos/ek.v5/req"
	"pkg.re/essentialkaos/ek.v5/signal"
	"pkg.re/essentialkaos/ek.v5/spellcheck"
	"pkg.re/essentialkaos/ek.v5/terminal"
	"pkg.re/essentialkaos/ek.v5/timeutil"
	"pkg.re/essentialkaos/ek.v5/tmp"
	"pkg.re/essentialkaos/ek.v5/usage"

	sshkey "github.com/yosida95/golang-sshkey"

	"github.com/essentialkaos/terrafarm/do"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// App info
const (
	APP  = "Terrafarm"
	VER  = "0.10.4"
	DESC = "Utility for working with terraform based rpmbuilder farm"
)

// List of supported command-line arguments
const (
	ARG_TTL         = "t:ttl"
	ARG_OUTPUT      = "o:output"
	ARG_TOKEN       = "T:token"
	ARG_KEY         = "K:key"
	ARG_REGION      = "R:region"
	ARG_NODE_SIZE   = "N:node-size"
	ARG_USER        = "U:user"
	ARG_PASSWORD    = "P:password"
	ARG_DEBUG       = "D:debug"
	ARG_MONITOR     = "m:monitor"
	ARG_MAX_WAIT    = "w:max-wait"
	ARG_FORCE       = "f:force"
	ARG_NO_VALIDATE = "nv:no-validate"
	ARG_NOTIFY      = "n:notify"
	ARG_NO_COLOR    = "nc:no-color"
	ARG_HELP        = "h:help"
	ARG_VER         = "v:version"
)

// List of supported commands
const (
	CMD_CREATE    = "create"
	CMD_APPLY     = "apply"
	CMD_START     = "start"
	CMD_DESTROY   = "destroy"
	CMD_DELETE    = "delete"
	CMD_STOP      = "stop"
	CMD_STATUS    = "status"
	CMD_INFO      = "info"
	CMD_STATE     = "state"
	CMD_TEMPLATES = "templates"
	CMD_PROLONG   = "prolong"
	CMD_DOCTOR    = "doctor"

	CMD_CREATE_SHORTCUT    = "c"
	CMD_DESTROY_SHORTCUT   = "d"
	CMD_STATUS_SHORTCUT    = "s"
	CMD_TEMPLATES_SHORTCUT = "t"
	CMD_PROLONG_SHORTCUT   = "p"
)

// List of supported environment variables
const (
	EV_DATA      = "TERRAFARM_DATA"
	EV_TTL       = "TERRAFARM_TTL"
	EV_MAX_WAIT  = "TERRAFARM_MAX_WAIT"
	EV_OUTPUT    = "TERRAFARM_OUTPUT"
	EV_TEMPLATE  = "TERRAFARM_TEMPLATE"
	EV_TOKEN     = "TERRAFARM_TOKEN"
	EV_KEY       = "TERRAFARM_KEY"
	EV_REGION    = "TERRAFARM_REGION"
	EV_NODE_SIZE = "TERRAFARM_NODE_SIZE"
	EV_USER      = "TERRAFARM_USER"
	EV_PASSWORD  = "TERRAFARM_PASSWORD"
)

// List of supported preferences
const (
	PREFS_TTL       = "ttl"
	PREFS_MAX_WAIT  = "max-wait"
	PREFS_OUTPUT    = "output"
	PREFS_TEMPLATE  = "template"
	PREFS_TOKEN     = "token"
	PREFS_KEY       = "key"
	PREFS_REGION    = "region"
	PREFS_NODE_SIZE = "node-size"
	PREFS_USER      = "user"
	PREFS_PASSWORD  = "password"
)

// List of build node states
const (
	STATE_UNKNOWN uint8 = iota
	STATE_INACTIVE
	STATE_ACTIVE
	STATE_DOWN
)

// TERRAFORM_DATA_DIR is name of directory with terraform data
const TERRAFORM_DATA_DIR = "terradata"

// TERRAFORM_STATE_FILE is name terraform state file name
const TERRAFORM_STATE_FILE = "terraform.tfstate"

// SRC_DIR is path to directory with terrafarm sources
const SRC_DIR = "github.com/essentialkaos/terrafarm"

// MONITOR_STATE_FILE is name of monitor state file
const MONITOR_STATE_FILE = ".monitor-state"

// FARM_STATE_FILE is name of terrafarm state file
const FARM_STATE_FILE = ".farm-state"

// MONITOR_LOG_FILE is name of monitor log file
const MONITOR_LOG_FILE = "monitor.log"

// SEPARATOR is separator used for log output
const SEPARATOR = "----------------------------------------------------------------------------------------"

// ////////////////////////////////////////////////////////////////////////////////// //

// FarmState contains farm specific info
type FarmState struct {
	Preferences *Preferences `json:"preferences"`
	Fingerprint string       `json:"fingerprint"`
}

// NodeInfo contains info about build node
type NodeInfo struct {
	Name     string
	IP       string
	Arch     string
	User     string
	Password string
	State    uint8
}

// ////////////////////////////////////////////////////////////////////////////////// //

// NodeInfoSlice is slice with node info structs
type NodeInfoSlice []*NodeInfo

func (p NodeInfoSlice) Len() int           { return len(p) }
func (p NodeInfoSlice) Less(i, j int) bool { return p[i].Name < p[j].Name }
func (p NodeInfoSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// ////////////////////////////////////////////////////////////////////////////////// //

// argMap is map with supported command-line arguments
var argMap = arg.Map{
	ARG_TTL:         &arg.V{},
	ARG_OUTPUT:      &arg.V{},
	ARG_TOKEN:       &arg.V{},
	ARG_KEY:         &arg.V{},
	ARG_REGION:      &arg.V{},
	ARG_NODE_SIZE:   &arg.V{},
	ARG_USER:        &arg.V{},
	ARG_MAX_WAIT:    &arg.V{},
	ARG_DEBUG:       &arg.V{Type: arg.BOOL},
	ARG_MONITOR:     &arg.V{},
	ARG_FORCE:       &arg.V{Type: arg.BOOL},
	ARG_NO_VALIDATE: &arg.V{Type: arg.BOOL},
	ARG_NOTIFY:      &arg.V{Type: arg.BOOL},
	ARG_NO_COLOR:    &arg.V{Type: arg.BOOL},
	ARG_HELP:        &arg.V{Type: arg.BOOL, Alias: "u:usage"},
	ARG_VER:         &arg.V{Type: arg.BOOL, Alias: "ver"},
}

// depList is slice with dependencies required by terrafarm
var depList = []string{
	"terraform",
}

// envMap is map with environment variables
var envMap = env.Get()

// startTime is time when app is started
var startTime = time.Now().Unix()

// dropletPrices contains per-hour droplet prices
var dropletPrices = map[string]float64{
	"512mb": 0.007,
	"1gb":   0.015,
	"2gb":   0.030,
	"4gb":   0.060,
	"8gb":   0.119,
	"16gb":  0.238,
	"32gb":  0.426,
	"48gb":  0.714,
	"64gb":  0.952,
}

// temp is temp struct
var temp *tmp.Temp

// ////////////////////////////////////////////////////////////////////////////////// //

func Init() {
	runtime.GOMAXPROCS(2)

	args, errs := arg.Parse(argMap)

	if len(errs) != 0 {
		for _, err := range errs {
			terminal.PrintErrorMessage(err.Error())
		}

		exit(1)
	}

	if arg.GetB(ARG_NO_COLOR) {
		fmtc.DisableColors = true
	}

	if arg.GetB(ARG_VER) {
		showAbout()
		return
	}

	if arg.GetB(ARG_HELP) {
		showUsage()
		return
	}

	if !arg.Has(ARG_MONITOR) && len(args) == 0 {
		showUsage()
		return
	}

	prepare()
	checkEnv()
	checkDeps()

	if arg.Has(ARG_MONITOR) {
		startFarmMonitor()
	} else {
		processCommand(args[0], args[1:])
	}
}

// prepare configure resources
func prepare() {
	var err error

	req.SetUserAgent(APP, VER)
	req.SetRequestTimeout(2.0)

	temp, err = tmp.NewTemp()

	if err != nil {
		terminal.PrintErrorMessage(err.Error())
		exit(1)
	}
}

// checkEnv check system environment
func checkEnv() {
	if envMap["GOPATH"] == "" {
		terminal.PrintErrorMessage("GOPATH must be set to valid path")
		exit(1)
	}

	srcDir := getSrcDir()

	if !fsutil.CheckPerms("DRW", srcDir) {
		terminal.PrintErrorMessage("Source directory %s is not accessible", srcDir)
		exit(1)
	}

	dataDir := getDataDir()

	if !fsutil.CheckPerms("DRW", dataDir) {
		terminal.PrintErrorMessage("Data directory %s is not accessible", dataDir)
		exit(1)
	}
}

// checkDeps check required dependencies
func checkDeps() {
	hasErrors := false

	for _, dep := range depList {
		if env.Which(dep) == "" {
			terminal.PrintErrorMessage("Can't find %s. Please install it first.", dep)
			hasErrors = true
		}
	}

	if hasErrors {
		exit(1)
	}
}

// processCommand execute some command
func processCommand(cmd string, args []string) {
	scm := getSpellcheckModel()
	cmd = scm.Correct(cmd)

	switch cmd {
	case CMD_CREATE, CMD_APPLY, CMD_START, CMD_CREATE_SHORTCUT:
		createCommand(findAndReadPreferences(), args)
	case CMD_DESTROY, CMD_DELETE, CMD_STOP, CMD_DESTROY_SHORTCUT:
		destroyCommand(findAndReadPreferences())
	case CMD_STATUS, CMD_INFO, CMD_STATE, CMD_STATUS_SHORTCUT:
		statusCommand(findAndReadPreferences())
	case CMD_TEMPLATES, CMD_TEMPLATES_SHORTCUT:
		templatesCommand()
	case CMD_PROLONG, CMD_PROLONG_SHORTCUT:
		prolongCommand(args)
	case CMD_DOCTOR:
		doctorCommand(findAndReadPreferences())
	default:
		terminal.PrintErrorMessage("Unknown command %s", cmd)
		exit(1)
	}

	exit(0)
}

// createCommand is create command handler
func createCommand(prefs *Preferences, args []string) {
	if isTerrafarmActive() {
		terminal.PrintWarnMessage("Terrafarm already works")
		exit(1)
	}

	if len(args) != 0 {
		prefs.Template = args[0]
		validatePreferences(prefs)
	}

	statusCommand(prefs)

	if !arg.GetB(ARG_FORCE) {
		yes, err := terminal.ReadAnswer("Create farm with this preferences?", "n")

		if !yes || err != nil {
			fmtc.NewLine()
			return
		}

		fmtutil.Separator(false)
	}

	vars, err := prefsToArgs(prefs)

	if err != nil {
		terminal.PrintErrorMessage("Can't parse preferences: %v", err)
		exit(1)
	}

	addSignalInterception()

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("{s-}EXEC → terraform apply %s{!}\n\n", strings.Join(vars, " "))
	}

	fsutil.Push(path.Join(getDataDir(), prefs.Template))

	err = execTerraform(false, "apply", vars)

	if err != nil {
		terminal.PrintErrorMessage("\nError while executing terraform: %v", err)
		exit(1)
	}

	fsutil.Pop()

	fmtutil.Separator(false)

	if prefs.Output != "" {
		fmtc.Println("Exporting info about build nodes...")

		err = exportNodeList(prefs)

		if err != nil {
			terminal.PrintErrorMessage("Error while exporting info: %v", err)
		} else {
			fmtc.Printf("{g}Info about build nodes saved as %s{!}\n", prefs.Output)
		}

		fmtutil.Separator(false)
	} else {
		fmtc.Println("Access credentials for created build nodes:\n")

		printNodesInfo(prefs)

		fmtutil.Separator(false)
	}

	if prefs.TTL > 0 {
		fmtc.Printf("Starting monitoring process... ")

		err = startMonitorProcess(prefs, false)

		if err != nil {
			fmtc.NewLine()
			terminal.PrintErrorMessage("Error while starting monitoring process: %v", err)
			exit(1)
		}

		fmtc.Println("{g}DONE{!}")

		fmtutil.Separator(false)
	}

	saveState(prefs)

	if arg.GetB(ARG_NOTIFY) {
		fmtc.Bell()
	}
}

// statusCommand is status command handler
func statusCommand(prefs *Preferences) {
	var (
		tokenValid       do.StatusCode
		fingerprintValid do.StatusCode
		regionValid      do.StatusCode
		sizeValid        do.StatusCode

		ttlHours           float64
		ttlRemain          int64
		totalUsagePriceMin float64
		totalUsagePriceMax float64
		currentUsagePrice  float64

		disableValidation bool

		waitBuildComplete bool

		buildersTotal   int
		buildersBullets string

		fingerprint string
	)

	var (
		terrafarmActive = isTerrafarmActive()
		monitorActive   = isMonitorActive()
	)

	disableValidation = arg.GetB(ARG_NO_VALIDATE)
	fingerprint, _ = getFingerprint(prefs.Key + ".pub")

	if terrafarmActive {
		farmState, err := readFarmState()

		if err == nil {
			disableValidation = true
			prefs = farmState.Preferences
			fingerprint = farmState.Fingerprint
		}
	}

	buildersTotal = getBuildNodesCount(prefs.Template)

	ttlHours = float64(prefs.TTL) / 60.0
	totalUsagePriceMin = (ttlHours * dropletPrices[prefs.NodeSize]) * float64(buildersTotal)

	if prefs.MaxWait > 0 {
		ttlWaitHours := float64(prefs.MaxWait) / 60.0
		totalUsagePriceMax = totalUsagePriceMin
		totalUsagePriceMax += (ttlWaitHours * dropletPrices[prefs.NodeSize]) * float64(buildersTotal)
	}

	if monitorActive {
		state, err := readMonitorState()

		if err == nil {
			waitBuildComplete = state.MaxWait > 0
			ttlRemain = state.DestroyAfter - time.Now().Unix()
			usageHours := time.Since(time.Unix(state.Started, 0)).Hours()
			currentUsagePrice = (usageHours * dropletPrices[prefs.NodeSize]) * float64(buildersTotal)
		}

		buildersBullets = getBuildBullets(prefs)
	}

	if !disableValidation {
		tokenValid = do.IsValidToken(prefs.Token)
		fingerprintValid = do.IsFingerprintValid(prefs.Token, fingerprint)
		regionValid = do.IsRegionValid(prefs.Token, prefs.Region)
		sizeValid = do.IsSizeValid(prefs.Token, prefs.NodeSize)
	}

	fmtutil.Separator(false, "TERRAFARM")

	fmtc.Printf(
		"  {*}%-16s{!} %s {s-}(%s){!}\n", "Template:", prefs.Template,
		pluralize.Pluralize(buildersTotal, "build node", "build nodes"),
	)

	fmtc.Printf("  {*}%-16s{!} %s", "Token:", getMaskedToken(prefs.Token))

	printValidationMarker(tokenValid, disableValidation)

	fmtc.Printf("  {*}%-16s{!} %s\n", "Private Key:", prefs.Key)
	fmtc.Printf("  {*}%-16s{!} %s\n", "Public Key:", prefs.Key+".pub")

	fmtc.Printf("  {*}%-16s{!} %s", "Fingerprint:", fingerprint)

	printValidationMarker(fingerprintValid, disableValidation)

	switch {
	case prefs.TTL <= 0:
		fmtc.Printf("  {*}%-16s{!} {r}disabled{!}", "TTL:")
	case prefs.TTL > 360:
		fmtc.Printf("  {*}%-16s{!} {r}%s{!}", "TTL:", timeutil.PrettyDuration(prefs.TTL*60))
	case prefs.TTL > 120:
		fmtc.Printf("  {*}%-16s{!} {y}%s{!}", "TTL:", timeutil.PrettyDuration(prefs.TTL*60))
	default:
		fmtc.Printf("  {*}%-16s{!} {g}%s{!}", "TTL:", timeutil.PrettyDuration(prefs.TTL*60))
	}

	if prefs.MaxWait > 0 {
		fmtc.Printf("{s-} + %s wait{!}", pluralize.Pluralize(int(prefs.MaxWait), "minute", "minutes"))
	}

	if prefs.TTL <= 0 || totalUsagePriceMin <= 0 {
		fmtc.NewLine()
	} else if totalUsagePriceMin > 0 && totalUsagePriceMax > 0 {
		fmtc.Printf(" {s-}(~ $%.2f - $%.2f){!}\n", totalUsagePriceMin, totalUsagePriceMax)
	} else {
		fmtc.Printf(" {s-}(~ $%.2f){!}\n", totalUsagePriceMin)
	}

	fmtc.Printf("  {*}%-16s{!} %s", "Region:", prefs.Region)

	printValidationMarker(regionValid, disableValidation)

	fmtc.Printf("  {*}%-16s{!} %s", "Node size:", prefs.NodeSize)

	printValidationMarker(sizeValid, disableValidation)

	fmtc.Printf("  {*}%-16s{!} %s\n", "User:", prefs.User)

	if prefs.Output != "" {
		fmtc.Printf("  {*}%-16s{!} %s\n", "Output:", prefs.Output)
	}

	fmtc.NewLine()

	if !isTerrafarmActive() {
		fmtc.Printf("  {*}%-16s{!} {s}stopped{!}\n", "State:")
	} else {
		fmtc.Printf("  {*}%-16s{!} {g}works{!}", "State:")

		if currentUsagePrice == 0 {
			fmtc.NewLine()
		} else {
			fmtc.Printf(" {s-}($%.2f){!}\n", currentUsagePrice)
		}

		fmtc.Printf("  {*}%-16s{!} "+buildersBullets+"\n", "Nodes Statuses:")

		if monitorActive {
			if ttlRemain == 0 {
				fmtc.Printf("  {*}%-16s{!} {r}unknown{!}\n", "Monitor:")
			} else {
				if ttlRemain < 0 {
					if waitBuildComplete {
						fmtc.Printf("  {*}%-16s{!} {g}works{!} {y}(waiting){!}\n", "Monitor:")
					} else {
						fmtc.Printf("  {*}%-16s{!} {g}works{!} {y}(destroying){!}\n", "Monitor:")
					}
				} else {
					fmtc.Printf(
						"  {*}%-16s{!} {g}works{!} {s-}(%s to destroy){!}\n",
						"Monitor:", timeutil.PrettyDuration(ttlRemain),
					)
				}
			}
		} else {
			fmtc.Printf("  {*}%-16s{!} {r}stopped{!}\n", "Monitor:")
		}
	}

	fmtutil.Separator(false)
}

// destroyCommand is destroy command handler
func destroyCommand(prefs *Preferences) {
	if !isTerrafarmActive() {
		terminal.PrintWarnMessage("Terrafarm does not works, nothing to destroy")
		exit(1)
	}

	activeBuildNodesCount := getActiveBuildNodesCount(prefs)

	if !arg.GetB(ARG_FORCE) {
		fmtc.NewLine()

		if activeBuildNodesCount != 0 {
			yes, err := terminal.ReadAnswer(
				fmtc.Sprintf(
					"Currently farm have %s. Do you REALLY want destroy farm?",
					pluralize.Pluralize(activeBuildNodesCount, "active build process", "active build processes"),
				), "n",
			)

			if !yes || err != nil {
				fmtc.NewLine()
				return
			}
		} else {
			yes, err := terminal.ReadAnswer("Destroy farm?", "n")

			if !yes || err != nil {
				fmtc.NewLine()
				return
			}
		}
	}

	fmtutil.Separator(false)

	priceMessage, priceMessageComment := getUsagePriceMessage()

	vars, err := prefsToArgs(prefs, "-force")

	if err != nil {
		terminal.PrintErrorMessage("Can't parse prefs: %v", err)
		exit(1)
	}

	addSignalInterception()

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("{s-}EXEC → terraform destroy %s{!}\n\n", strings.Join(vars, " "))
	}

	fsutil.Push(path.Join(getDataDir(), prefs.Template))

	err = execTerraform(false, "destroy", vars)

	if err != nil {
		terminal.PrintErrorMessage("\nError while executing terraform: %v", err)
		exit(1)
	}

	fsutil.Pop()

	fmtutil.Separator(false)

	if priceMessage != "" {
		fmtc.Printf("  {*}Usage price:{!} %s {s-}(%s){!}\n\n", priceMessage, priceMessageComment)
	}

	deleteFarmStateFile()

	if arg.GetB(ARG_NOTIFY) {
		fmtc.Bell()
	}
}

// templatesCommand is templates command handler
func templatesCommand() {
	templates := fsutil.List(
		getDataDir(), true,
		&fsutil.ListingFilter{Perms: "DRX"},
	)

	if len(templates) == 0 {
		terminal.PrintWarnMessage("No templates found")
		return
	}

	sort.Strings(templates)

	fmtutil.Separator(false, "TEMPLATES")

	for _, template := range templates {
		buildersCount := getBuildNodesCount(template)

		fmtc.Printf(
			"  %s {s-}(%s){!}\n", template,
			pluralize.Pluralize(buildersCount, "build node", "build nodes"),
		)
	}

	fmtutil.Separator(false)
}

// prolongCommand prolong farm TTL
func prolongCommand(args []string) {
	if !isTerrafarmActive() {
		terminal.PrintWarnMessage("Farm does not works")
		exit(1)
	}

	if !isMonitorActive() {
		terminal.PrintWarnMessage("Monitor does not works")
		exit(1)
	}

	if len(args) == 0 {
		terminal.PrintErrorMessage("You must provide prolongation time")
		exit(1)
	}

	var (
		ttl     int64
		maxWait int64
	)

	if len(args) >= 1 {
		ttl = timeutil.ParseDuration(args[0]) / 60

		if ttl == 0 {
			terminal.PrintErrorMessage("Incorrect ttl property")
		}
	}

	if len(args) >= 2 {
		maxWait = timeutil.ParseDuration(args[1]) / 60

		if maxWait == 0 {
			terminal.PrintErrorMessage("Incorrect max-wait property")
		}
	}

	fmtc.NewLine()

	var answer string

	switch maxWait {
	case 0:
		answer = fmtc.Sprintf(
			"Do you want to increase TTL on %s?",
			timeutil.PrettyDuration(time.Duration(ttl)*time.Minute),
		)

	default:
		answer = fmtc.Sprintf(
			"Do you want to increase TTL on %s and set max wait to %s?",
			timeutil.PrettyDuration(time.Duration(ttl)*time.Minute),
			timeutil.PrettyDuration(time.Duration(maxWait)*time.Minute),
		)
	}

	yes, err := terminal.ReadAnswer(answer, "y")

	if !yes || err != nil {
		fmtc.NewLine()
		return
	}

	fmtc.NewLine()

	farmState, err := readFarmState()

	if err != nil {
		terminal.PrintErrorMessage("ERROR\n")
		terminal.PrintErrorMessage("Can't read farm state file: %v\n", err)
		exit(1)
	}

	monitorState, err := readMonitorState()

	if err != nil {
		terminal.PrintErrorMessage("ERROR\n")
		terminal.PrintErrorMessage("Can't read monitor state file: %v\n", err)
		exit(1)
	}

	fmtc.Printf("Stopping monitor process... ")

	err = killMonitorProcess()

	if err != nil {
		terminal.PrintErrorMessage("ERROR\n")
		terminal.PrintErrorMessage("Can't stop monitor process: %v\n", err)
		exit(1)
	} else {
		fmtc.Println("{g}DONE{!}")
	}

	fmtc.Printf("Updating farm state... ")

	farmState.Preferences.TTL += ttl

	if maxWait != 0 {
		farmState.Preferences.MaxWait = maxWait
	}

	err = updateFarmState(farmState)

	if err != nil {
		terminal.PrintErrorMessage("ERROR\n")
		terminal.PrintErrorMessage("Can't save farm state: %v\n", err)
		exit(1)
	} else {
		fmtc.Println("{g}DONE{!}")
	}

	fmtc.Printf("Updating monitor state... ")

	monitorState.DestroyAfter += (ttl * 60)

	if maxWait != 0 {
		monitorState.MaxWait = maxWait * 60
	}

	err = saveMonitorState(monitorState)

	if err != nil {
		terminal.PrintErrorMessage("ERROR\n")
		terminal.PrintErrorMessage("Can't save monitoring state: %v\n", err)
		exit(1)
	} else {
		fmtc.Println("{g}DONE{!}")
	}

	fmtc.Printf("Starting monitoring process... ")

	err = startMonitorProcess(farmState.Preferences, true)

	if err != nil {
		terminal.PrintErrorMessage("ERROR\n")
		terminal.PrintErrorMessage("Can't start monitoring process: %v\n", err)
		exit(1)
	} else {
		fmtc.Println("{g}DONE{!}")
	}

	fmtc.NewLine()
}

// doctorCommand fix problems with farm
func doctorCommand(prefs *Preferences) {
	fmtc.Println("\nThis command can solve almost all problems that can occur with farm.")
	fmtc.Println("This is list of actions which will be performed:\n")

	fmtc.Println(" - Terrafarm monitor will be stopped")
	fmtc.Println(" - Terraform state file will be removed")
	fmtc.Println(" - Terrafarm state file will be removed")
	fmtc.Println(" - All droplets with prefix \"terrafarm\" will be destroyed\n")

	yes, err := terminal.ReadAnswer("Perform dry run of this actions?", "n")

	if !yes || err != nil {
		fmtc.NewLine()
		return
	}

	fmtc.NewLine()

	terraformStateFile := getTerraformStateFilePath()
	terrafarmStateFile := getFarmStateFilePath()

	terrafarmDroplets, err := do.GetTerrafarmDropletsList(prefs.Token)

	if err != nil {
		terminal.PrintErrorMessage(err.Error())
		exit(1)
	}

	fmtc.Printf("  File %s removed\n", terraformStateFile)
	fmtc.Printf("  File %s removed\n", terrafarmStateFile)

	for dropletName, dropletID := range terrafarmDroplets {
		fmtc.Printf("  Droplet %s {s-}(ID: %d){!} destroyed\n", dropletName, dropletID)
	}

	fmtc.NewLine()

	yes, err = terminal.ReadAnswer("Perform this actions?", "n")

	if !yes || err != nil {
		fmtc.NewLine()
		return
	}

	fmtc.NewLine()

	printErrorStatusMarker(killMonitorProcess())

	fmtc.Println("Terrafarm monitor stoppped")

	printErrorStatusMarker(os.Remove(terraformStateFile))

	fmtc.Printf("File %s removed\n", terraformStateFile)

	printErrorStatusMarker(os.Remove(terrafarmStateFile))

	fmtc.Printf("File %s removed\n", terrafarmStateFile)

	printErrorStatusMarker(do.DestroyTerrafarmDroplets(prefs.Token))

	fmtc.Println("Terrafarm droplets destroyed")

	fmtc.NewLine()
}

// saveFarmState collect and save farm state into file
func saveState(prefs *Preferences) {
	fingerprint, _ := getFingerprint(prefs.Key + ".pub")

	farmState := &FarmState{
		Preferences: prefs,
		Fingerprint: fingerprint,
	}

	farmState.Preferences.Token = getCryptedToken(prefs.Token)
	farmState.Preferences.Password = ""

	err := saveFarmState(farmState)

	if err != nil {
		fmtc.Printf("Can't save farm state: %v\n", err)
	}
}

// printValidationMarker print validation mark
func printValidationMarker(value do.StatusCode, disableValidate bool) {
	switch {
	case disableValidate == true:
		fmtc.Printf("\n")
	case value == do.STATUS_OK:
		fmtc.Printf(" {g}✔ {!}\n")
	case value == do.STATUS_NOT_OK:
		fmtc.Printf(" {r}✘ {!}\n")
	case value == do.STATUS_ERROR:
		fmtc.Printf(" {y*}? {!}\n")
	}
}

// printErrorStatusMarker print green check symbol if err is null
// or red cross otherwise
func printErrorStatusMarker(err error) {
	if err == nil {
		fmtc.Printf("{g}✔{!} ")
	} else {
		fmtc.Printf("{r}✘{!} ")
	}
}

// getFingerprint return fingerprint for public key
func getFingerprint(key string) (string, error) {
	data, err := ioutil.ReadFile(key)

	if err != nil {
		return "", err
	}

	pubkey, err := sshkey.UnmarshalPublicKey(string(data[:]))

	if err != nil {
		return "", err
	}

	return sshkey.PrettyFingerprint(pubkey, crypto.MD5)
}

// getMaskedToken return first and last 8 symbols of token
func getMaskedToken(token string) string {
	if len(token) != 64 {
		return ""
	}

	return token[:8] + "..." + token[56:]
}

// getCryptedToken return token with masked part
func getCryptedToken(token string) string {
	if len(token) != 64 {
		return ""
	}

	return token[:8] + "XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX" + token[56:]
}

// getBuildBullets return colored string with bullets
func getBuildBullets(prefs *Preferences) string {
	nodes := getBuildNodesInfo(prefs)

	if len(nodes) == 0 {
		return "{y}unknown{!}"
	}

	var result string

	for _, node := range nodes {
		switch node.State {
		case STATE_ACTIVE:
			result += "{g}•{!}"

		case STATE_INACTIVE:
			result += "{s-}•{!}"

		default:
			result += "{r}•{!}"
		}
	}

	return result
}

// prefsToArgs return preferences as command line arguments for terraform
func prefsToArgs(prefs *Preferences, args ...string) ([]string, error) {
	varsData, err := prefs.GetVariablesData()

	if err != nil {
		return nil, err
	}

	varsFd, varsFile, err := temp.MkFile()

	if err != nil {
		return nil, err
	}

	_, err = fmtc.Fprintln(varsFd, varsData)

	if err != nil {
		return nil, err
	}

	varsSlice := []string{
		fmtc.Sprintf("-var-file=%s", varsFile),
		fmtc.Sprintf("-state=%s", getTerraformStateFilePath()),
	}

	if len(args) != 0 {
		varsSlice = append(varsSlice, args...)
	}

	return varsSlice, nil
}

// execTerraform execute terraform command
func execTerraform(logOutput bool, command string, args []string) error {
	cmd := exec.Command("terraform", command)

	if len(args) != 0 {
		cmd.Args = append(cmd.Args, strings.Split(strings.Join(args, " "), " ")...)
	}

	reader, err := cmd.StdoutPipe()

	if err != nil {
		return fmtc.Errorf("Can't redirect output: %v", err)
	}

	statusLines := false
	scanner := bufio.NewScanner(reader)

	go func() {
		for scanner.Scan() {
			text := scanner.Text()

			if logOutput {
				// Skip empty line logging
				if text != "" {
					log.Info(text)
				}
			} else {
				if strings.Contains(text, "Still ") {
					if !statusLines {
						statusLines = true
						fmtc.Printf("  {s}.{!}")
					} else {
						fmtc.Printf("{s}.{!}")
					}

					continue
				} else {
					if statusLines {
						statusLines = false
						fmtc.NewLine()
					}
				}

				fmtc.Printf("  %s\n", getColoredCommandOutput(text))
			}
		}
	}()

	err = cmd.Start()

	if err != nil {
		return fmtc.Errorf("Can't start terraform: %v", err)
	}

	err = cmd.Wait()

	if err != nil {
		return fmtc.Errorf("Can't process terraform output: %v", err)
	}

	return nil
}

// getColoredCommandOutput return command output with colored remote-exec
func getColoredCommandOutput(line string) string {
	// Remove garbage from line
	line = strings.Replace(line, "\x1b[0m\x1b[0m", "", -1)

	switch {
	case strings.Contains(line, "-x32 (remote-exec)"):
		return fmtc.Sprintf("{c}%s{!}", line)

	case strings.Contains(line, "-x48 (remote-exec)"):
		return fmtc.Sprintf("{b}%s{!}", line)

	case strings.Contains(line, "-x64 (remote-exec)"):
		return fmtc.Sprintf("{m}%s{!}", line)

	default:
		return line
	}
}

// getUsagePriceMessage return message with usage price
func getUsagePriceMessage() (string, string) {
	if !isMonitorActive() {
		return "", ""
	}

	state, err := readMonitorState()

	if err != nil {
		return "", ""
	}

	farmState, err := readFarmState()

	if err != nil {
		return "", ""
	}

	buildersTotal := getBuildNodesCount(farmState.Preferences.Template)
	usageHours := time.Since(time.Unix(state.Started, 0)).Hours()
	usageMinutes := int(time.Since(time.Unix(state.Started, 0)).Minutes())
	currentUsagePrice := (usageHours * dropletPrices[farmState.Preferences.NodeSize]) * float64(buildersTotal)

	switch buildersTotal {
	case 1:
		return fmtc.Sprintf("$%.2f", currentUsagePrice),
			fmtc.Sprintf("%s × %d min", farmState.Preferences.NodeSize, usageMinutes)
	default:
		return fmtc.Sprintf("$%.2f", currentUsagePrice),
			fmtc.Sprintf("%d × %s × %d min", buildersTotal, farmState.Preferences.NodeSize, usageMinutes)
	}
}

// addSignalInterception add interceptors for INT и TERM signals
func addSignalInterception() {
	signal.Handlers{
		signal.INT:  signalInterceptor,
		signal.TERM: signalInterceptor,
	}.TrackAsync()
}

// isTerrafarmActive return true if terrafarm already active
func isTerrafarmActive() bool {
	stateFile := getTerraformStateFilePath()

	if !fsutil.IsExist(stateFile) {
		return false
	}

	state, err := readTFState(stateFile)

	if err != nil {
		return true
	}

	return len(state.Modules[0].Resources) != 0
}

// getBuildNodesCount return number of nodes in given farm template
func getBuildNodesCount(template string) int {
	templateDir := path.Join(getDataDir(), template)

	builders := fsutil.List(
		templateDir, true,
		&fsutil.ListingFilter{
			MatchPatterns: []string{"builder*.tf"},
		},
	)

	return len(builders)
}

// getDataDir return path to directory with terraform data
func getDataDir() string {
	if envMap[EV_DATA] != "" {
		return envMap[EV_DATA]
	}

	return path.Join(getSrcDir(), TERRAFORM_DATA_DIR)
}

// getSrcDir return path to directory with terrafarm sources
func getSrcDir() string {
	return path.Join(envMap["GOPATH"], "src", SRC_DIR)
}

// getTerraformStateFilePath return path to terraform state file
func getTerraformStateFilePath() string {
	return path.Join(getDataDir(), TERRAFORM_STATE_FILE)
}

// getFarmStateFilePath return path to terrafarm state file
func getFarmStateFilePath() string {
	return path.Join(getDataDir(), FARM_STATE_FILE)
}

// deleteFarmStateFile remote farm state file
func deleteFarmStateFile() error {
	return os.Remove(getFarmStateFilePath())
}

// saveFarmState save farm state to file
func saveFarmState(state *FarmState) error {
	return jsonutil.EncodeToFile(getFarmStateFilePath(), state)
}

// updateState update farm state file
func updateFarmState(state *FarmState) error {
	err := deleteFarmStateFile()

	if err != nil {
		return err
	}

	err = saveFarmState(state)

	if err != nil {
		fmtc.Errorf("Can't save farm state: %v", err)
	}

	return nil
}

// readFarmState read farm state from file
func readFarmState() (*FarmState, error) {
	state := &FarmState{}
	stateFile := getFarmStateFilePath()

	if !fsutil.IsExist(stateFile) {
		return nil, fmtc.Errorf("Farm state file is not exist")
	}

	err := jsonutil.DecodeFile(stateFile, state)

	if err != nil {
		return nil, err
	}

	return state, nil
}

// collectNodesInfo collect base info about build nodes
func collectNodesInfo(prefs *Preferences) ([]*NodeInfo, error) {
	state, err := readTFState(getTerraformStateFilePath())

	if err != nil {
		return nil, fmtc.Errorf("Can't read state file: %v", err)
	}

	if len(state.Modules) == 0 || len(state.Modules[0].Resources) == 0 {
		return nil, nil
	}

	var result []*NodeInfo

	for _, node := range state.Modules[0].Resources {
		if node.Info == nil || node.Info.Attributes == nil {
			continue
		}

		node := &NodeInfo{
			Name:     node.Info.Attributes.Name,
			IP:       node.Info.Attributes.IP,
			User:     prefs.User,
			Password: prefs.Password,
			State:    STATE_UNKNOWN,
		}

		switch {
		case strings.HasSuffix(node.Name, "-x32"):
			node.Arch = "i386"

		case strings.HasSuffix(node.Name, "-x48"):
			node.Arch = "i686"
		}

		result = append(result, node)
	}

	sort.Sort(NodeInfoSlice(result))

	return result, nil
}

// printNodesInfo collect and print info about build nodes
func printNodesInfo(prefs *Preferences) {
	nodesInfo, err := collectNodesInfo(prefs)

	if err != nil {
		terminal.PrintErrorMessage("Can't collect nodes info: %v", err)
		return
	}

	for _, node := range nodesInfo {
		fmtc.Printf(
			"  {*}%20s{!}: ssh %s@%s {s-}(Password: %s){!}\n",
			node.Name, node.User, node.IP, node.Password,
		)
	}
}

// exportNodeList exports info about nodes for usage in rpmbuilder
func exportNodeList(prefs *Preferences) error {
	if fsutil.IsExist(prefs.Output) {
		if fsutil.IsDir(prefs.Output) {
			return fmtc.Errorf("Output path must be path to file")
		}

		if !fsutil.IsWritable(prefs.Output) {
			return fmtc.Errorf("Output path must be path to writable file")
		}

		os.Remove(prefs.Output)
	}

	fd, err := os.OpenFile(prefs.Output, os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return err
	}

	defer fd.Close()

	nodesInfo, err := collectNodesInfo(prefs)

	if err != nil {
		return err
	}

	for _, node := range nodesInfo {
		if node.Arch == "" {
			fmtc.Fprintf(fd, "%s:%s@%s\n", node.User, node.Password, node.IP)
		} else {
			fmtc.Fprintf(fd, "%s:%s@%s~%s\n", node.User, node.Password, node.IP, node.Arch)
		}
	}

	return nil
}

// signalInterceptor is TERM and INT signal handler
func signalInterceptor() {
	terminal.PrintWarnMessage("\nYou can't cancel command execution in this time")
}

// getSpellcheckModel return spellcheck model for correcting
// given command name
func getSpellcheckModel() *spellcheck.Model {
	return spellcheck.Train([]string{
		CMD_CREATE, CMD_APPLY, CMD_START,
		CMD_DESTROY, CMD_DELETE, CMD_STOP,
		CMD_STATUS, CMD_INFO, CMD_STATE,
	})
}

// cleanTerraformGarbage remove tf-plugin* files from
// temporary directory
func cleanTerraformGarbage() {
	garbage := fsutil.List(
		"/tmp", false,
		&fsutil.ListingFilter{
			MatchPatterns: []string{"tf-plugin*", "plugin*"},
			CTimeYounger:  startTime,
		},
	)

	if len(garbage) == 0 {
		return
	}

	fsutil.ListToAbsolute("/tmp", garbage)

	for _, file := range garbage {
		os.Remove(file)
	}
}

// exit exit from app with given code
func exit(code int) {
	if !arg.GetB(ARG_DEBUG) {
		temp.Clean()
	}

	cleanTerraformGarbage()
	os.Exit(code)
}

// ////////////////////////////////////////////////////////////////////////////////// //

// showUsage show help content
func showUsage() {
	info := usage.NewInfo("")

	info.AddCommand(CMD_CREATE, "Create and run farm droplets on DigitalOcean", "template-name")
	info.AddCommand(CMD_DESTROY, "Destroy farm droplets on DigitalOcean")
	info.AddCommand(CMD_STATUS, "Show current Terrafarm preferences and status")
	info.AddCommand(CMD_TEMPLATES, "List all available farm templates")
	info.AddCommand(CMD_PROLONG, "Increase TTL or set max wait time", "ttl max-wait")
	info.AddCommand(CMD_DOCTOR, "Fix problems with farm")

	info.AddOption(ARG_TTL, "Max farm TTL {s-}(Time To Live){!}", "time")
	info.AddOption(ARG_MAX_WAIT, "Max time which monitor will wait if farm have active build", "time")
	info.AddOption(ARG_OUTPUT, "Path to output file with access credentials", "file")
	info.AddOption(ARG_TOKEN, "DigitalOcean token", "token")
	info.AddOption(ARG_KEY, "Path to private key", "key-file")
	info.AddOption(ARG_REGION, "DigitalOcean region", "region")
	info.AddOption(ARG_NODE_SIZE, "Droplet size on DigitalOcean", "size")
	info.AddOption(ARG_USER, "Build node user name", "username")
	info.AddOption(ARG_PASSWORD, "Build node user password", "password")
	info.AddOption(ARG_FORCE, "Force command execution")
	info.AddOption(ARG_NO_VALIDATE, "Don't validate preferences")
	info.AddOption(ARG_NOTIFY, "Ring the system bell after finishing command execution")
	info.AddOption(ARG_NO_COLOR, "Disable colors in output")
	info.AddOption(ARG_HELP, "Show this help message")
	info.AddOption(ARG_VER, "Show version")

	info.AddExample(CMD_CREATE+" --node-size 8gb --ttl 3h", "Create farm with redefined node size and TTL")
	info.AddExample(CMD_CREATE+" --force", "Forced farm creation (without prompt)")
	info.AddExample(CMD_CREATE+" c6-multiarch-fast", "Create farm from template c6-multiarch-fast")
	info.AddExample(CMD_DESTROY, "Destroy all farm nodes")
	info.AddExample(CMD_STATUS, "Show info about terrafarm")
	info.AddExample(CMD_PROLONG+" 1h 15m", "Increase TTL on 1 hour and set max wait to 15 minutes")

	info.Render()
}

// showAbout show info about utility
func showAbout() {
	about := &usage.About{
		App:     APP,
		Version: VER,
		Desc:    DESC,
		Year:    2006,
		Owner:   "ESSENTIAL KAOS",
		License: "Essential Kaos Open Source License <https://essentialkaos.com/ekol?en>",
	}

	about.Render()
}
