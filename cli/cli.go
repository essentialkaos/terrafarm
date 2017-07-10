package cli

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2017 ESSENTIAL KAOS                         //
//        Essential Kaos Open Source License <https://essentialkaos.com/ekol>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"pkg.re/essentialkaos/ek.v9/env"
	"pkg.re/essentialkaos/ek.v9/fmtc"
	"pkg.re/essentialkaos/ek.v9/fmtutil"
	"pkg.re/essentialkaos/ek.v9/fsutil"
	"pkg.re/essentialkaos/ek.v9/jsonutil"
	"pkg.re/essentialkaos/ek.v9/log"
	"pkg.re/essentialkaos/ek.v9/mathutil"
	"pkg.re/essentialkaos/ek.v9/options"
	"pkg.re/essentialkaos/ek.v9/path"
	"pkg.re/essentialkaos/ek.v9/pluralize"
	"pkg.re/essentialkaos/ek.v9/req"
	"pkg.re/essentialkaos/ek.v9/spellcheck"
	"pkg.re/essentialkaos/ek.v9/terminal"
	"pkg.re/essentialkaos/ek.v9/timeutil"
	"pkg.re/essentialkaos/ek.v9/tmp"
	"pkg.re/essentialkaos/ek.v9/usage"
	"pkg.re/essentialkaos/ek.v9/usage/update"

	"github.com/essentialkaos/terrafarm/do"
	"github.com/essentialkaos/terrafarm/prefs"
	"github.com/essentialkaos/terrafarm/terraform"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// App info
const (
	APP  = "Terrafarm"
	VER  = "1.3.0"
	DESC = "Utility for working with terraform based RPMBuilder farm"
)

// List of supported command-line arguments
const (
	OPT_TTL         = "t:ttl"
	OPT_OUTPUT      = "o:output"
	OPT_TOKEN       = "T:token"
	OPT_KEY         = "K:key"
	OPT_REGION      = "R:region"
	OPT_NODE_SIZE   = "N:node-size"
	OPT_USER        = "U:user"
	OPT_PASSWORD    = "P:password"
	OPT_DEBUG       = "D:debug"
	OPT_MONITOR     = "m:monitor"
	OPT_MAX_WAIT    = "w:max-wait"
	OPT_FORCE       = "f:force"
	OPT_NO_VALIDATE = "nv:no-validate"
	OPT_NOTIFY      = "n:notify"
	OPT_NO_COLOR    = "nc:no-color"
	OPT_HELP        = "h:help"
	OPT_VER         = "v:version"
)

// List of supported commands
const (
	CMD_APPLY     = "apply"
	CMD_CREATE    = "create"
	CMD_DELETE    = "delete"
	CMD_DESTROY   = "destroy"
	CMD_DOCTOR    = "doctor"
	CMD_INFO      = "info"
	CMD_PROLONG   = "prolong"
	CMD_START     = "start"
	CMD_STATE     = "state"
	CMD_STATUS    = "status"
	CMD_STOP      = "stop"
	CMD_TEMPLATES = "templates"
	CMD_RESOURCES = "resources"

	CMD_CREATE_SHORTCUT    = "c"
	CMD_DESTROY_SHORTCUT   = "d"
	CMD_PROLONG_SHORTCUT   = "p"
	CMD_STATUS_SHORTCUT    = "s"
	CMD_TEMPLATES_SHORTCUT = "t"
	CMD_RESOURCES_SHORTCUT = "r"
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

// DropletInfo contains basic node info
type DropletInfo struct {
	Price  float64
	CPU    int
	Memory float64
	Disk   int
}

type RegionInfo struct {
	DCName     string
	RegionName string
}

// ////////////////////////////////////////////////////////////////////////////////// //

// NodeInfoSlice is slice with node info structs
type NodeInfoSlice []*NodeInfo

func (p NodeInfoSlice) Len() int           { return len(p) }
func (p NodeInfoSlice) Less(i, j int) bool { return p[i].Name < p[j].Name }
func (p NodeInfoSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// ////////////////////////////////////////////////////////////////////////////////// //

// optMap is map with supported command-line options
var optMap = options.Map{
	OPT_TTL:         {},
	OPT_OUTPUT:      {},
	OPT_TOKEN:       {},
	OPT_KEY:         {},
	OPT_REGION:      {},
	OPT_NODE_SIZE:   {},
	OPT_USER:        {},
	OPT_MAX_WAIT:    {},
	OPT_DEBUG:       {Type: options.BOOL},
	OPT_MONITOR:     {Type: options.BOOL},
	OPT_FORCE:       {Type: options.BOOL},
	OPT_NO_VALIDATE: {Type: options.BOOL},
	OPT_NOTIFY:      {Type: options.BOOL},
	OPT_NO_COLOR:    {Type: options.BOOL},
	OPT_HELP:        {Type: options.BOOL, Alias: "u:usage"},
	OPT_VER:         {Type: options.BOOL, Alias: "ver"},
}

// depList is slice with dependencies required by terrafarm
var depList = []string{
	"terraform",
}

// envMap is map with environment variables
var envMap = env.Get()

// startTime is time when app is started
var startTime = time.Now().Unix()

// droplets contains droplets codes
var droplets = []string{
	"512mb", "1gb", "2gb", "4gb", "8gb", "16gb", "32gb", "64gb",
	"m-16gb", "m-32gb", "m-64gb", "m-128gb", "m-224gb",
}

// dropletInfoStorage contains info about droplets
var dropletInfoStorage = map[string]DropletInfo{
	"512mb":   {0.007, 1, 0.512, 20},
	"1gb":     {0.015, 1, 1, 30},
	"2gb":     {0.030, 2, 2, 40},
	"4gb":     {0.060, 2, 4, 60},
	"8gb":     {0.119, 4, 8, 80},
	"16gb":    {0.238, 8, 16, 160},
	"32gb":    {0.426, 12, 32, 320},
	"48gb":    {0.714, 16, 48, 480},
	"64gb":    {0.952, 20, 64, 640},
	"m-16gb":  {0.179, 2, 16, 30},
	"m-32gb":  {0.357, 4, 32, 90},
	"m-64gb":  {0.714, 8, 64, 200},
	"m-128gb": {1.429, 16, 128, 340},
	"m-224gb": {2.500, 32, 224, 500},
}

// regions contains regions codes
var regions = []string{
	"nyc1", "nyc2", "nyc3", "sfo1", "sfo2", "tor1",
	"lon1", "ams2", "ams3", "fra1", "blr1", "sgp1",
}

// regionInfoStorage contains info about regions
var regionInfoStorage = map[string]RegionInfo{
	"nyc1": {"New York #1", "US East"},
	"nyc2": {"New York #2", "US East"},
	"nyc3": {"New York #3", "US East"},
	"sfo1": {"San Francisco #1", "US West"},
	"sfo2": {"San Francisco #2", "US West"},
	"tor1": {"Toronto", "Canada"},
	"lon1": {"London", "UK"},
	"ams2": {"Amsterdam #2", "Europe"},
	"ams3": {"Amsterdam #3", "Europe"},
	"fra1": {"Frankfurt", "Europe"},
	"blr1": {"Bangalore", "Asia Pacific"},
	"sgp1": {"Singapore", "Asia Pacific"},
}

// colorTags contains fmtc color codes
var colorTags = []string{
	"{c}", "{m}", "{b}", "{y}", "{g}",
	"{c*}", "{m*}", "{b*}", "{y*}", "{g*}",
}

// temp is temp struct
var temp *tmp.Temp

// curTmuxWindowIndex is index of tmux window
var curTmuxWindowIndex string

// ////////////////////////////////////////////////////////////////////////////////// //

func Init() {
	runtime.GOMAXPROCS(2)

	args, errs := options.Parse(optMap)

	if len(errs) != 0 {
		for _, err := range errs {
			terminal.PrintErrorMessage(err.Error())
		}

		exit(1)
	}

	configureUI()

	if options.GetB(OPT_VER) {
		showAbout()
		return
	}

	if options.GetB(OPT_HELP) {
		showUsage()
		return
	}

	if !options.GetB(OPT_MONITOR) && len(args) == 0 {
		showUsage()
		return
	}

	prepare()
	checkEnv()
	checkDeps()

	if options.GetB(OPT_MONITOR) {
		startFarmMonitor()
	} else {
		processCommand(args[0], args[1:])
	}
}

// configureUI configure UI
func configureUI() {
	ev := env.Get()
	term := ev.GetS("TERM")

	if term != "" {
		switch {
		case strings.Contains(term, "xterm"),
			strings.Contains(term, "color"),
			term == "screen":
			fmtc.DisableColors = false
		}
	}

	if ev.GetS("TMUX") != "" {
		curTmuxWindowIndex = getCurrentTmuxWindowIndex()
	}

	if options.GetB(OPT_NO_COLOR) {
		fmtc.DisableColors = true
	}

	if !fsutil.IsCharacterDevice("/dev/stdout") && ev.GetS("FAKETTY") == "" {
		fmtc.DisableColors = true
	}

	if fmtc.DisableColors {
		terminal.Prompt = "› "
		fmtutil.SeparatorSymbol = "–"
	}

	fmtutil.SeparatorFullscreen = true
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
	case CMD_RESOURCES, CMD_RESOURCES_SHORTCUT:
		resourcesCommand()
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
	}

	validatePreferences(p)
	statusCommand(p)

	if !options.GetB(OPT_FORCE) {
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

	printDebug("EXEC → terraform apply %s", strings.Join(vars, " "))

	// Current moment + 90 seconds for starting droplets
	farmStartTime := time.Now().Unix() + 90

	fsutil.Push(path.Join(getDataDir(), p.Template))

	err = execTerraform(false, "apply", vars)

	fsutil.Pop()

	if err != nil {
		terminal.PrintErrorMessage("\nError while executing terraform: %v", err)
		exit(1)
	}

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

	notify()
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

	disableValidation = options.GetB(OPT_NO_VALIDATE)

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

		if p.Template != "" {
			regionValid = do.IsRegionValid(p.Token, p.Region)
			sizeValid = do.IsSizeValid(p.Token, p.NodeSize)
		}
	}

	fmtutil.Separator(false, "TERRAFARM")

	if p.Template != "" {
		fmtc.Printf(
			"  {*}%-16s{!} %s {s-}(%s){!}\n", "Template:", p.Template,
			pluralize.Pluralize(buildersTotal, "build node", "build nodes"),
		)
	}

	fmtc.Printf("  {*}%-16s{!} %s", "Token:", getPrettyToken(p.Token))

	printValidationMarker(tokenValid, disableValidation, true)

	fmtc.Printf("  {*}%-16s{!} %s\n", "Private Key:", p.Key)
	fmtc.Printf("  {*}%-16s{!} %s\n", "Public Key:", p.Key+".pub")

	fmtc.Printf("  {*}%-16s{!} %s", "Fingerprint:", p.Fingerprint)

	printValidationMarker(fingerprintValid, disableValidation, true)

	if p.Template != "" {
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

		printValidationMarker(regionValid, disableValidation, true)

		fmtc.Printf("  {*}%-16s{!} %s", "Node size:", p.NodeSize)

		printValidationMarker(sizeValid, disableValidation, false)

		if dropletInfoStorage[p.NodeSize].CPU != 0 {
			if disableValidation {
				fmt.Printf(" ")
			}

			fmtc.Printf(
				"{s-}(%s + %d GB Disk){!}\n",
				pluralize.Pluralize(dropletInfoStorage[p.NodeSize].CPU, "CPU", "CPUs"),
				dropletInfoStorage[p.NodeSize].Disk,
			)
		} else {
			fmtc.NewLine()
		}

		fmtc.Printf("  {*}%-16s{!} %s\n", "User:", p.User)
	}

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
func destroyCommand(prefs *prefs.Preferences) {
	if !isTerrafarmActive() {
		terminal.PrintWarnMessage("Terrafarm does not works, nothing to destroy")
		exit(1)
	}

	activeBuildNodesCount := getActiveBuildNodesCount(prefs)

	if !options.GetB(OPT_FORCE) {
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

	farmState, err := readFarmState()

	if err != nil {
		terminal.PrintErrorMessage("Can't read farm state: %v", err)
		exit(1)
	}

	p := farmState.Preferences
	p.Token = prefs.Token
	p.Password = prefs.Password

	vars, err := prefsToArgs(p, "-force")

	if err != nil {
		terminal.PrintErrorMessage("Can't parse prefs: %v", err)
		exit(1)
	}

	priceMessage, priceMessageComment := getUsagePriceMessage()

	fmtutil.Separator(false)

	printDebug("EXEC → terraform destroy %s", strings.Join(vars, " "))

	fsutil.Push(path.Join(getDataDir(), p.Template))

	err = execTerraform(false, "destroy", vars)

	fsutil.Pop()

	if err != nil {
		terminal.PrintErrorMessage("\nError while executing terraform: %v", err)
		exit(1)
	}

	fmtutil.Separator(false)

	if priceMessage != "" {
		fmtc.Printf("  {*}Usage price:{!} %s {s-}(%s){!}\n\n", priceMessage, priceMessageComment)
	}

	deleteFarmStateFile()

	notify()
}

// templatesCommand is templates command handler
func templatesCommand() {
	templates := fsutil.List(
		getDataDir(), true,
		fsutil.ListingFilter{Perms: "DRX"},
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

// templatesCommand is resources command handler
func resourcesCommand() {
	fmtutil.Separator(false, "DROPLETS")

	for _, d := range droplets {
		di := dropletInfoStorage[d]
		fmtc.Printf("  {c}%7s{!} $%g/hr {s-}(%s + %g GB Memory + %d GB Disk){!}\n", d,
			di.Price, pluralize.Pluralize(di.CPU, "CPU", "CPUs"), di.Memory, di.Disk,
		)
	}

	fmtutil.Separator(false, "REGIONS")

	for _, r := range regions {
		ri := regionInfoStorage[r]
		fmtc.Printf("  {y}%s{!} %s {s-}(%s){!}\n", r, ri.DCName, ri.RegionName)
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

	farmState.Preferences.Token = getMaskedToken(p.Token)
	farmState.Preferences.Password = ""

	err := saveFarmState(farmState)

	if err != nil {
		fmtc.Printf("Can't save farm state: %v\n", err)
	}
}

// printValidationMarker print validation mark
func printValidationMarker(value do.StatusCode, disableValidate, newLine bool) {
	if !disableValidate {
		switch {
		case value == do.STATUS_OK:
			fmtc.Printf(" {g}✔ {!}")
		case value == do.STATUS_NOT_OK:
			fmtc.Printf(" {r}✘ {!}")
		case value == do.STATUS_ERROR:
			fmtc.Printf(" {y*}? {!}")
		}
	}

	if newLine {
		fmtc.NewLine()
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

// getPrettyToken return first and last 8 symbols of token
func getPrettyToken(token string) string {
	if len(token) != 64 {
		return ""
	}

	return token[:8] + "..." + token[56:]
}

// getMaskedToken return token with masked part
func getMaskedToken(token string) string {
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

	stdoutReader, err := cmd.StdoutPipe()

	if err != nil {
		return fmtc.Errorf("Can't redirect output: %v", err)
	}

	var stderrBuffer bytes.Buffer

	cmd.Stderr = &stderrBuffer

	statusLines := false
	scanner := bufio.NewScanner(stdoutReader)

	go func() {
		// map nodeName -> color
		colorStore := make(map[string]string)

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

				fmtc.Printf("  %s\n", getColoredCommandOutput(colorStore, text))
			}
		}
	}()

	err = cmd.Start()

	if err != nil {
		return fmtc.Errorf("Can't start terraform: %v", err)
	}

	err = cmd.Wait()

	if err != nil {
		return fmtc.Errorf(stderrBuffer.String())
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
	errs := p.Validate(getDataDir(), false)

	if len(errs) != 0 {
		for _, err := range errs {
			terminal.PrintErrorMessage(err.Error())
		}

		os.Exit(1)
	}
}

// getColoredCommandOutput return command output with colored remote-exec
func getColoredCommandOutput(colorStore map[string]string, line string) string {

	// Remove garbage from line
	line = strings.Replace(line, "\x1b[0m\x1b[0m", "", -1)

	if !strings.Contains(line, "(remote-exec)") {
		return line
	}

	nodeNameEnd := strings.Index(line, " ")

	if nodeNameEnd == -1 {
		return line
	}

	nodeName := line[:nodeNameEnd]
	colorTag := colorStore[nodeName]

	if colorTag == "" {
		colorTag = colorTags[len(colorStore)]
		colorStore[nodeName] = colorTag
	}

	line = strings.Replace(line, nodeName+" (remote-exec):", colorTag+nodeName+" (remote-exec):{!}", -1)

	return fmtc.Sprintf(line)
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
	currentUsagePrice := (usageHours * dropletInfoStorage[farmState.Preferences.NodeSize].Price) * float64(buildersTotal)
	currentUsagePrice = mathutil.BetweenF(currentUsagePrice, 0.01, 1000000.0)

	switch buildersTotal {
	case 1:
		return fmtc.Sprintf("~ $%.2f", currentUsagePrice),
			fmtc.Sprintf("%s × %d min", farmState.Preferences.NodeSize, usageMinutes)
	default:
		return fmtc.Sprintf("~ $%.2f", currentUsagePrice),
			fmtc.Sprintf("%d × %s × %d min", buildersTotal, farmState.Preferences.NodeSize, usageMinutes)
	}
}

// calculateUsagePrice calculate usage price
func calculateUsagePrice(time int64, nodeNum int, nodeSize string) float64 {
	if dropletInfoStorage[nodeSize].Price == 0.0 {
		return 0.0
	}

	hours := float64(time) / 60.0
	price := (hours * dropletInfoStorage[nodeSize].Price) * float64(nodeNum)
	price = mathutil.BetweenF(price, 0.01, 1000000.0)

	return price
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
		fsutil.ListingFilter{
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

// getSpellcheckModel return spellcheck model for correcting
// given command name
func getSpellcheckModel() *spellcheck.Model {
	return spellcheck.Train([]string{
		CMD_APPLY, CMD_CREATE, CMD_DELETE, CMD_DESTROY,
		CMD_DOCTOR, CMD_INFO, CMD_PROLONG, CMD_START,
		CMD_STATE, CMD_STATUS, CMD_STOP, CMD_TEMPLATES,
		CMD_RESOURCES,
	})
}

// cleanTerraformGarbage remove tf-plugin* files from
// temporary directory
func cleanTerraformGarbage() {
	garbage := fsutil.List(
		"/tmp", false,
		fsutil.ListingFilter{
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

// notify print a bell symbol
func notify() {
	if options.GetB(OPT_NOTIFY) {
		fmtc.Bell()
	}

	if curTmuxWindowIndex != "" {
		windowIndex := getCurrentTmuxWindowIndex()

		if windowIndex != curTmuxWindowIndex {
			fmtc.Bell()
		}
	}
}

// getCurrentTmuxWindowIndex return current window index in tmux
func getCurrentTmuxWindowIndex() string {
	output, err := exec.Command("tmux", "display-message", "-p", "#I").Output()

	if err != nil {
		return ""
	}

	return string(output[:])
}

// printDebug print debug message if debug mode enabled
func printDebug(message string, args ...interface{}) {
	if !options.GetB(OPT_DEBUG) {
		return
	}

	if len(args) == 0 {
		fmtc.Printf("{s-}%s{!}\n\n", message)
	} else {
		fmtc.Printf("{s-}%s{!}\n\n", fmt.Sprintf(message, args...))
	}
}

// exit exit from app with given code
func exit(code int) {
	if !options.GetB(OPT_DEBUG) {
		temp.Clean()
	}

	cleanTerraformGarbage()

	os.Exit(code)
}

// ////////////////////////////////////////////////////////////////////////////////// //

// showUsage show help content
func showUsage() {
	info := usage.NewInfo()

	info.AddCommand(CMD_CREATE, "Create and run farm droplets on DigitalOcean", "?template-name")
	info.AddCommand(CMD_DESTROY, "Destroy farm droplets on DigitalOcean")
	info.AddCommand(CMD_STATUS, "Show current Terrafarm preferences and status")
	info.AddCommand(CMD_TEMPLATES, "List all available farm templates")
	info.AddCommand(CMD_RESOURCES, "List available resources {s-}(droplets & regions){!}")
	info.AddCommand(CMD_PROLONG, "Increase TTL or set max wait time", "ttl", "?max-wait")
	info.AddCommand(CMD_DOCTOR, "Fix problems with farm")

	info.AddOption(OPT_TTL, "Max farm TTL {s-}(Time To Live){!}", "time")
	info.AddOption(OPT_MAX_WAIT, "Max time which monitor will wait if farm have active build", "time")
	info.AddOption(OPT_OUTPUT, "Path to output file with access credentials", "file")
	info.AddOption(OPT_TOKEN, "DigitalOcean token", "token")
	info.AddOption(OPT_KEY, "Path to private key", "key-file")
	info.AddOption(OPT_REGION, "DigitalOcean region", "region")
	info.AddOption(OPT_NODE_SIZE, "Droplet size on DigitalOcean", "size")
	info.AddOption(OPT_USER, "Build node user name", "username")
	info.AddOption(OPT_PASSWORD, "Build node user password", "password")
	info.AddOption(OPT_FORCE, "Force command execution")
	info.AddOption(OPT_NO_VALIDATE, "Don't validate preferences")
	info.AddOption(OPT_NOTIFY, "Ring the system bell after finishing command execution")
	info.AddOption(OPT_NO_COLOR, "Disable colors in output")
	info.AddOption(OPT_HELP, "Show this help message")
	info.AddOption(OPT_VER, "Show version")

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
		App:           APP,
		Version:       VER,
		Desc:          DESC,
		Year:          2006,
		Owner:         "ESSENTIAL KAOS",
		License:       "Essential Kaos Open Source License <https://essentialkaos.com/ekol>",
		UpdateChecker: usage.UpdateChecker{"essentialkaos/terrafarm", update.GitHubChecker},
	}

	about.Render()
}
