package cli

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2016 Essential Kaos                         //
//      Essential Kaos Open Source License <http://essentialkaos.com/ekol?en>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"bufio"
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
	"pkg.re/essentialkaos/ek.v5/mathutil"
	"pkg.re/essentialkaos/ek.v5/path"
	"pkg.re/essentialkaos/ek.v5/pluralize"
	"pkg.re/essentialkaos/ek.v5/req"
	"pkg.re/essentialkaos/ek.v5/signal"
	"pkg.re/essentialkaos/ek.v5/spellcheck"
	"pkg.re/essentialkaos/ek.v5/terminal"
	"pkg.re/essentialkaos/ek.v5/timeutil"
	"pkg.re/essentialkaos/ek.v5/tmp"
	"pkg.re/essentialkaos/ek.v5/usage"

	"github.com/essentialkaos/terrafarm/do"
	"github.com/essentialkaos/terrafarm/prefs"
	"github.com/essentialkaos/terrafarm/terraform"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// App info
const (
	APP  = "Terrafarm"
	VER  = "0.11.0"
	DESC = "Utility for working with terraform based RPMBuilder farm"
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

// List of build node states
const (
	STATE_UNKNOWN uint8 = iota
	STATE_INACTIVE
	STATE_ACTIVE
	STATE_DOWN
)

// Environment variable with path to data
const EV_DATA = "TERRAFARM_DATA"

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
	Preferences *prefs.Preferences `json:"preferences"`
	Started     int64              `json:"started"`
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
	ARG_TTL:         {},
	ARG_OUTPUT:      {},
	ARG_TOKEN:       {},
	ARG_KEY:         {},
	ARG_REGION:      {},
	ARG_NODE_SIZE:   {},
	ARG_USER:        {},
	ARG_MAX_WAIT:    {},
	ARG_DEBUG:       {Type: arg.BOOL},
	ARG_MONITOR:     {Type: arg.BOOL},
	ARG_FORCE:       {Type: arg.BOOL},
	ARG_NO_VALIDATE: {Type: arg.BOOL},
	ARG_NOTIFY:      {Type: arg.BOOL},
	ARG_NO_COLOR:    {Type: arg.BOOL},
	ARG_HELP:        {Type: arg.BOOL, Alias: "u:usage"},
	ARG_VER:         {Type: arg.BOOL, Alias: "ver"},
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
	"512mb":   0.007,
	"1gb":     0.015,
	"2gb":     0.030,
	"4gb":     0.060,
	"8gb":     0.119,
	"16gb":    0.238,
	"32gb":    0.426,
	"48gb":    0.714,
	"64gb":    0.952,
	"m-16gb":  0.179,
	"m-32gb":  0.357,
	"m-64gb":  0.714,
	"m-128gb": 1.429,
	"m-224gb": 2.500,
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

	if !arg.GetB(ARG_MONITOR) && len(args) == 0 {
		showUsage()
		return
	}

	prepare()
	checkEnv()
	checkDeps()

	if arg.GetB(ARG_MONITOR) {
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
		createCommand(getPreferences(), args)
	case CMD_DESTROY, CMD_DELETE, CMD_STOP, CMD_DESTROY_SHORTCUT:
		destroyCommand(getPreferences())
	case CMD_STATUS, CMD_INFO, CMD_STATE, CMD_STATUS_SHORTCUT:
		statusCommand(getPreferences())
	case CMD_TEMPLATES, CMD_TEMPLATES_SHORTCUT:
		templatesCommand()
	case CMD_PROLONG, CMD_PROLONG_SHORTCUT:
		prolongCommand(args)
	case CMD_DOCTOR:
		doctorCommand(getPreferences())
	default:
		terminal.PrintErrorMessage("Unknown command %s", cmd)
		exit(1)
	}

	exit(0)
}

// createCommand is create command handler
func createCommand(p *prefs.Preferences, args []string) {
	if isTerrafarmActive() {
		terminal.PrintWarnMessage("Terrafarm already works")
		exit(1)
	}

	if len(args) != 0 {
		p.Template = args[0]
		validatePreferences(p)
	}

	statusCommand(p)

	if !arg.GetB(ARG_FORCE) {
		yes, err := terminal.ReadAnswer("Create farm with this preferences?", "n")

		if !yes || err != nil {
			fmtc.NewLine()
			return
		}

		fmtutil.Separator(false)
	}

	vars, err := prefsToArgs(p)

	if err != nil {
		terminal.PrintErrorMessage("Can't parse preferences: %v", err)
		exit(1)
	}

	addSignalInterception()

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("{s-}EXEC → terraform apply %s{!}\n\n", strings.Join(vars, " "))
	}

	// Current moment + 90 seconds for starting droplets
	farmStartTime := time.Now().Unix() + 90

	fsutil.Push(path.Join(getDataDir(), p.Template))

	err = execTerraform(false, "apply", vars)

	if err != nil {
		terminal.PrintErrorMessage("\nError while executing terraform: %v", err)
		exit(1)
	}

	fsutil.Pop()

	fmtutil.Separator(false)

	if p.Output != "" {
		fmtc.Println("Exporting info about build nodes...")

		err = exportNodeList(p)

		if err != nil {
			terminal.PrintErrorMessage("Error while exporting info: %v", err)
		} else {
			fmtc.Printf("{g}Info about build nodes saved as %s{!}\n", p.Output)
		}

		fmtutil.Separator(false)
	} else {
		fmtc.Println("Access credentials for created build nodes:\n")

		printNodesInfo(p)

		fmtutil.Separator(false)
	}

	if p.TTL > 0 {
		fmtc.Printf("Starting monitoring process... ")

		err = saveMonitorState(&MonitorState{
			DestroyAfter: time.Now().Unix() + p.TTL*60,
			MaxWait:      p.MaxWait * 60,
		})

		if err != nil {
			fmtc.NewLine()
			terminal.PrintErrorMessage("Error while saving monitoring process state: %v", err)
			exit(1)
		}

		err = startMonitorProcess(false)

		if err != nil {
			fmtc.NewLine()
			terminal.PrintErrorMessage("Error while starting monitoring process: %v", err)
			exit(1)
		}

		fmtc.Println("{g}DONE{!}")

		fmtutil.Separator(false)
	}

	saveState(p, farmStartTime)

	if arg.GetB(ARG_NOTIFY) {
		fmtc.Bell()
	}
}

// statusCommand is status command handler
func statusCommand(p *prefs.Preferences) {
	var (
		err error

		farmState    *FarmState
		monitorState *MonitorState

		tokenValid       do.StatusCode
		fingerprintValid do.StatusCode
		regionValid      do.StatusCode
		sizeValid        do.StatusCode

		ttlRemain          int64
		totalUsagePriceMin float64
		totalUsagePriceMax float64
		currentUsagePrice  float64

		disableValidation bool

		waitBuildComplete bool

		buildersTotal   int
		buildersBullets string
	)

	var (
		terrafarmActive = isTerrafarmActive()
		monitorActive   = isMonitorActive()
	)

	disableValidation = arg.GetB(ARG_NO_VALIDATE)

	if terrafarmActive {
		farmState, err = readFarmState()

		if err == nil {
			disableValidation = true
			p = farmState.Preferences
		}
	}

	buildersTotal = getBuildNodesCount(p.Template)

	totalUsagePriceMin = calculateUsagePrice(p.TTL, buildersTotal, p.NodeSize)

	if terrafarmActive {
		usageHours := int64(time.Since(time.Unix(farmState.Started, 0)).Hours() * 60)
		currentUsagePrice = calculateUsagePrice(usageHours, buildersTotal, p.NodeSize)
	}

	if p.MaxWait > 0 {
		totalUsagePriceMax = totalUsagePriceMin
		totalUsagePriceMax += calculateUsagePrice(p.MaxWait, buildersTotal, p.NodeSize)
	}

	if monitorActive {
		monitorState, err = readMonitorState()

		if err == nil {
			waitBuildComplete = monitorState.MaxWait > 0
			ttlRemain = monitorState.DestroyAfter - time.Now().Unix()
		}

		buildersBullets = getBuildBullets(p)
	}

	if !disableValidation {
		tokenValid = do.IsValidToken(p.Token)
		fingerprintValid = do.IsFingerprintValid(p.Token, p.Fingerprint)
		regionValid = do.IsRegionValid(p.Token, p.Region)
		sizeValid = do.IsSizeValid(p.Token, p.NodeSize)
	}

	fmtutil.Separator(false, "TERRAFARM")

	fmtc.Printf(
		"  {*}%-16s{!} %s {s-}(%s){!}\n", "Template:", p.Template,
		pluralize.Pluralize(buildersTotal, "build node", "build nodes"),
	)

	fmtc.Printf("  {*}%-16s{!} %s", "Token:", getMaskedToken(p.Token))

	printValidationMarker(tokenValid, disableValidation)

	fmtc.Printf("  {*}%-16s{!} %s\n", "Private Key:", p.Key)
	fmtc.Printf("  {*}%-16s{!} %s\n", "Public Key:", p.Key+".pub")

	fmtc.Printf("  {*}%-16s{!} %s", "Fingerprint:", p.Fingerprint)

	printValidationMarker(fingerprintValid, disableValidation)

	switch {
	case p.TTL <= 0:
		fmtc.Printf("  {*}%-16s{!} {r}disabled{!}", "TTL:")
	case p.TTL > 360:
		fmtc.Printf("  {*}%-16s{!} {r}%s{!}", "TTL:", timeutil.PrettyDuration(p.TTL*60))
	case p.TTL > 120:
		fmtc.Printf("  {*}%-16s{!} {y}%s{!}", "TTL:", timeutil.PrettyDuration(p.TTL*60))
	default:
		fmtc.Printf("  {*}%-16s{!} {g}%s{!}", "TTL:", timeutil.PrettyDuration(p.TTL*60))
	}

	if p.MaxWait > 0 {
		fmtc.Printf("{s-} + %s wait{!}", pluralize.Pluralize(int(p.MaxWait), "minute", "minutes"))
	}

	if p.TTL <= 0 || totalUsagePriceMin <= 0 {
		fmtc.NewLine()
	} else if totalUsagePriceMin > 0 && totalUsagePriceMax > 0 {
		fmtc.Printf(" {s-}(~ $%.2f - $%.2f){!}\n", totalUsagePriceMin, totalUsagePriceMax)
	} else {
		fmtc.Printf(" {s-}(~ $%.2f){!}\n", totalUsagePriceMin)
	}

	fmtc.Printf("  {*}%-16s{!} %s", "Region:", p.Region)

	printValidationMarker(regionValid, disableValidation)

	fmtc.Printf("  {*}%-16s{!} %s", "Node size:", p.NodeSize)

	printValidationMarker(sizeValid, disableValidation)

	fmtc.Printf("  {*}%-16s{!} %s\n", "User:", p.User)

	if p.Output != "" {
		fmtc.Printf("  {*}%-16s{!} %s\n", "Output:", p.Output)
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
func destroyCommand(p *prefs.Preferences) {
	if !isTerrafarmActive() {
		terminal.PrintWarnMessage("Terrafarm does not works, nothing to destroy")
		exit(1)
	}

	activeBuildNodesCount := getActiveBuildNodesCount(p)

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

	vars, err := prefsToArgs(p, "-force")

	if err != nil {
		terminal.PrintErrorMessage("Can't parse prefs: %v", err)
		exit(1)
	}

	addSignalInterception()

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("{s-}EXEC → terraform destroy %s{!}\n\n", strings.Join(vars, " "))
	}

	fsutil.Push(path.Join(getDataDir(), p.Template))

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

	var ttl, maxWait int64

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

	err = startMonitorProcess(true)

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
func doctorCommand(p *prefs.Preferences) {
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

	terrafarmDroplets, err := do.GetTerrafarmDropletsList(p.Token)

	if err != nil {
		terminal.PrintErrorMessage(err.Error())
		exit(1)
	}

	fmtc.Println("  Terrafarm monitor stoppped")
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

	printErrorStatusMarker(do.DestroyTerrafarmDroplets(p.Token))
	fmtc.Println("Terrafarm droplets destroyed")

	fmtc.NewLine()
}

// saveFarmState collect and save farm state into file
func saveState(p *prefs.Preferences, farmStartTime int64) {
	farmState := &FarmState{
		Preferences: p,
		Started:     farmStartTime,
	}

	farmState.Preferences.Token = getCryptedToken(p.Token)
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
		fmtc.Printf("{g}✔ {!}")
	} else {
		fmtc.Printf("{r}✘ {!}")
	}
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
func getBuildBullets(p *prefs.Preferences) string {
	nodes := getBuildNodesInfo(p)

	if len(nodes) == 0 {
		return "{y}unknown{!}"
	}

	var result string

	for _, node := range nodes {
		switch node.State {
		case STATE_ACTIVE:
			result += "{g}•{!}"

		case STATE_INACTIVE:
			result += "{s}•{!}"

		default:
			result += "{r}•{!}"
		}
	}

	return result
}

// prefsToArgs return preferences as command line arguments for terraform
func prefsToArgs(p *prefs.Preferences, args ...string) ([]string, error) {
	varsData, err := p.GetVariablesData()

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

// getPreferencies
func getPreferences() *prefs.Preferences {
	p, errs := prefs.FindAndReadPreferences(getDataDir())

	if len(errs) != 0 {
		for _, err := range errs {
			terminal.PrintErrorMessage(err.Error())
		}

		os.Exit(1)
	}

	return p
}

func validatePreferences(p *prefs.Preferences) {
	errs := p.Validate(getDataDir())

	if len(errs) != 0 {
		for _, err := range errs {
			terminal.PrintErrorMessage(err.Error())
		}

		os.Exit(1)
	}
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

	farmState, err := readFarmState()

	if err != nil {
		return "", ""
	}

	buildersTotal := getBuildNodesCount(farmState.Preferences.Template)
	usageHours := time.Since(time.Unix(farmState.Started, 0)).Hours()
	usageMinutes := int(time.Since(time.Unix(farmState.Started, 0)).Minutes())
	currentUsagePrice := (usageHours * dropletPrices[farmState.Preferences.NodeSize]) * float64(buildersTotal)
	currentUsagePrice = mathutil.BetweenF(currentUsagePrice, 0.01, 1000000.0)

	switch buildersTotal {
	case 1:
		return fmtc.Sprintf("$%.2f", currentUsagePrice),
			fmtc.Sprintf("%s × %d min", farmState.Preferences.NodeSize, usageMinutes)
	default:
		return fmtc.Sprintf("$%.2f", currentUsagePrice),
			fmtc.Sprintf("%d × %s × %d min", buildersTotal, farmState.Preferences.NodeSize, usageMinutes)
	}
}

// calculateUsagePrice calculate usage price
func calculateUsagePrice(time int64, nodeNum int, nodeSize string) float64 {
	if dropletPrices[nodeSize] == 0.0 {
		return 0.0
	}

	hours := float64(time) / 60.0
	price := (hours * dropletPrices[nodeSize]) * float64(nodeNum)
	price = mathutil.BetweenF(price, 0.01, 1000000.0)

	return price
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

	tfState, err := terraform.ReadState(stateFile)

	if err != nil {
		return true
	}

	return len(tfState.Modules[0].Resources) != 0
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
func collectNodesInfo(p *prefs.Preferences) ([]*NodeInfo, error) {
	tfState, err := terraform.ReadState(getTerraformStateFilePath())

	if err != nil {
		return nil, fmtc.Errorf("Can't read state file: %v", err)
	}

	if len(tfState.Modules) == 0 || len(tfState.Modules[0].Resources) == 0 {
		return nil, nil
	}

	var result []*NodeInfo

	for _, node := range tfState.Modules[0].Resources {
		if node.Info == nil || node.Info.Attributes == nil {
			continue
		}

		node := &NodeInfo{
			Name:     node.Info.Attributes.Name,
			IP:       node.Info.Attributes.IP,
			User:     p.User,
			Password: p.Password,
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
func printNodesInfo(p *prefs.Preferences) {
	nodesInfo, err := collectNodesInfo(p)

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
func exportNodeList(p *prefs.Preferences) error {
	if fsutil.IsExist(p.Output) {
		if fsutil.IsDir(p.Output) {
			return fmtc.Errorf("Output path must be path to file")
		}

		if !fsutil.IsWritable(p.Output) {
			return fmtc.Errorf("Output path must be path to writable file")
		}

		os.Remove(p.Output)
	}

	fd, err := os.OpenFile(p.Output, os.O_CREATE|os.O_WRONLY, 0644)

	if err != nil {
		return err
	}

	defer fd.Close()

	nodesInfo, err := collectNodesInfo(p)

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
	usage.Breadcrumbs = true

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
		License: "Essential Kaos Open Source License <https://essentialkaos.com/ekol>",
	}

	about.Render()
}
