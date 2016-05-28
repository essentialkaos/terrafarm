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

	"pkg.re/essentialkaos/ek.v1/arg"
	"pkg.re/essentialkaos/ek.v1/env"
	"pkg.re/essentialkaos/ek.v1/fmtc"
	"pkg.re/essentialkaos/ek.v1/fmtutil"
	"pkg.re/essentialkaos/ek.v1/fsutil"
	"pkg.re/essentialkaos/ek.v1/jsonutil"
	"pkg.re/essentialkaos/ek.v1/log"
	"pkg.re/essentialkaos/ek.v1/path"
	"pkg.re/essentialkaos/ek.v1/signal"
	"pkg.re/essentialkaos/ek.v1/spellcheck"
	"pkg.re/essentialkaos/ek.v1/terminal"
	"pkg.re/essentialkaos/ek.v1/timeutil"
	"pkg.re/essentialkaos/ek.v1/usage"

	"gopkg.in/hlandau/passlib.v1/hash/sha2crypt"

	sshkey "github.com/yosida95/golang-sshkey"

	"github.com/essentialkaos/terrafarm/do"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// App info
const (
	APP  = "Terrafarm"
	VER  = "0.8.0"
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
	ARG_NO_COLOR:    &arg.V{Type: arg.BOOL},
	ARG_HELP:        &arg.V{Type: arg.BOOL, Alias: "u:usage"},
	ARG_VER:         &arg.V{Type: arg.BOOL, Alias: "ver"},
}

// depList is slice with dependencies required by terrafarm
var depList = []string{
	"terraform",
	"terraform-provider-digitalocean",
	"terraform-provisioner-file",
	"terraform-provisioner-remote-exec",
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

// ////////////////////////////////////////////////////////////////////////////////// //

func Init() {
	runtime.GOMAXPROCS(2)

	args, errs := arg.Parse(argMap)

	if len(errs) != 0 {
		fmtc.NewLine()

		for _, err := range errs {
			printError(err.Error())
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

	checkEnv()
	checkDeps()

	if arg.Has(ARG_MONITOR) {
		startMonitor()
	} else {
		processCommand(args[0], args[1:])
	}
}

// checkEnv check system environment
func checkEnv() {
	if envMap["GOPATH"] == "" {
		printError("GOPATH must be set to valid path")
		exit(1)
	}

	srcDir := getSrcDir()

	if !fsutil.CheckPerms("DRW", srcDir) {
		printError("Source directory %s is not accessible", srcDir)
		exit(1)
	}

	dataDir := getDataDir()

	if !fsutil.CheckPerms("DRW", dataDir) {
		printError("Data directory %s is not accessible", dataDir)
		exit(1)
	}
}

// checkDeps check required dependencies
func checkDeps() {
	hasErrors := false

	for _, dep := range depList {
		if env.Which(dep) == "" {
			printError("Can't find %s. Please install it first.", dep)
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
	case CMD_CREATE, CMD_APPLY, CMD_START:
		createCommand(findAndReadPreferences(), args)
	case CMD_DESTROY, CMD_DELETE, CMD_STOP:
		destroyCommand(findAndReadPreferences())
	case CMD_STATUS, CMD_INFO, CMD_STATE:
		statusCommand(findAndReadPreferences())
	case CMD_TEMPLATES:
		templatesCommand()
	default:
		fmtc.Printf("{r}Unknown command %s\n", cmd)
		exit(1)
	}

	exit(0)
}

// createCommand is create command handler
func createCommand(prefs *Preferences, args []string) {
	if isTerrafarmActive() {
		printWarn("Terrafarm already works")
		exit(1)
	}

	if len(args) != 0 {
		prefs.Template = args[0]
		validatePreferences(prefs)
	}

	statusCommand(prefs)

	if !arg.GetB(ARG_FORCE) {
		if !terminal.ReadAnswer("Create farm with this preferences? (y/N)", "n") {
			fmtc.NewLine()
			return
		}

		fmtutil.Separator(false)
	}

	vars, err := prefsToArgs(prefs)

	if err != nil {
		printError("Can't parse preferences: %v", err)
		exit(1)
	}

	addSignalInterception()

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("{s}EXEC → terraform apply %s{!}\n\n", strings.Join(vars, " "))
	}

	fsutil.Push(path.Join(getDataDir(), prefs.Template))

	err = execTerraform(false, "apply", vars)

	if err != nil {
		fmtc.Printf("{r}Error while executing terraform: %v\n{!}", err)
		exit(1)
	}

	fsutil.Pop()

	fmtutil.Separator(false)

	if prefs.Output != "" {
		fmtc.Println("Exporting info about build nodes...")

		err = exportNodeList(prefs)

		if err != nil {
			fmtc.Printf("{r}Error while exporting info: %v\n", err)
		} else {
			fmtc.Printf("{g}Info about build nodes saved as %s{!}\n", prefs.Output)
		}

		fmtutil.Separator(false)
	}

	if prefs.TTL > 0 {
		fmtc.Println("Starting monitoring process...")

		err = runMonitor(prefs)

		if err != nil {
			fmtc.Printf("{r}Error while starting monitoring process: %v\n", err)
			exit(1)
		}

		fmtc.Println("{g}Monitoring process successfully started!{!}")

		fmtutil.Separator(false)
	}

	saveState(prefs)
}

// statusCommand is status command handler
func statusCommand(prefs *Preferences) {
	var (
		tokenValid       bool
		fingerprintValid bool
		regionValid      bool
		sizeValid        bool

		ttlHours           float64
		ttlRemain          int64
		totalUsagePriceMin float64
		totalUsagePriceMax float64
		currentUsagePrice  float64

		disableValidation bool

		waitBuildComplete bool

		buildersActive int
		buildersTotal  int

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

		buildersActive = len(GetActiveBuildNodes(prefs))
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
	}

	if !disableValidation {
		tokenValid = do.IsValidToken(prefs.Token)
		fingerprintValid = do.IsFingerprintValid(prefs.Token, fingerprint)
		regionValid = do.IsRegionValid(prefs.Token, prefs.Region)
		sizeValid = do.IsSizeValid(prefs.Token, prefs.NodeSize)
	}

	fmtutil.Separator(false, "TERRAFARM")

	fmtc.Printf(
		"  {*}%-16s{!} %s {s}(%s){!}\n", "Template:", prefs.Template,
		fmtutil.Pluralize(buildersTotal, "build node", "build nodes"),
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
		fmtc.Printf("{s} + %s wait{!}", fmtutil.Pluralize(int(prefs.MaxWait), "minute", "minutes"))
	}

	if prefs.TTL <= 0 || totalUsagePriceMin <= 0 {
		fmtc.NewLine()
	} else if totalUsagePriceMin > 0 && totalUsagePriceMax > 0 {
		fmtc.Printf(" {s}(~ $%.2f - $%.2f){!}\n", totalUsagePriceMin, totalUsagePriceMax)
	} else {
		fmtc.Printf(" {s}(~ $%.2f){!}\n", totalUsagePriceMin)
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
			fmtc.Printf(" {s}($%.2f){!}\n", currentUsagePrice)
		}

		fmtc.Printf("  {*}%-16s{!} "+getActiveBuildBullets(buildersActive, buildersTotal)+"\n", "Active Builds:")

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
						"  {*}%-16s{!} {g}works{!} {s}(%s to destroy){!}\n",
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
		fmtc.Println("{y}Terrafarm does not works, nothing to destroy{!}")
		exit(1)
	}

	if !arg.GetB(ARG_FORCE) {
		fmtc.NewLine()

		if !terminal.ReadAnswer("Destroy farm? (y/N)", "n") {
			return
		}
	}

	fmtutil.Separator(false)

	vars, err := prefsToArgs(prefs, "-force")

	if err != nil {
		printError("Can't parse prefs: %v", err)
		exit(1)
	}

	addSignalInterception()

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("{s}EXEC → terraform destroy %s{!}\n\n", strings.Join(vars, " "))
	}

	fsutil.Push(path.Join(getDataDir(), prefs.Template))

	err = execTerraform(false, "destroy", vars)

	if err != nil {
		fmtc.Printf("{r}Error while executing terraform: %v\n{!}", err)
		exit(1)
	}

	fsutil.Pop()

	fmtutil.Separator(false)

	deleteFarmStateFile()
}

// templatesCommand is templates command handler
func templatesCommand() {
	templates := fsutil.List(
		getDataDir(), true,
		&fsutil.ListingFilter{Perms: "DRX"},
	)

	if len(templates) == 0 {
		printWarn("No templates found")
		return
	}

	sort.Strings(templates)

	fmtutil.Separator(false, "TEMPLATES")

	for _, template := range templates {
		buildersCount := getBuildNodesCount(template)

		fmtc.Printf(
			"  %s {s}(%s){!}\n", template,
			fmtutil.Pluralize(buildersCount, "build node", "build nodes"),
		)
	}

	fmtutil.Separator(false)
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
func printValidationMarker(value, disableValidate bool) {
	switch {
	case disableValidate == true:
		fmtc.Printf("\n")
	case value == true:
		fmtc.Printf(" {g}✔{!}\n")
	case value == false:
		fmtc.Printf(" {r}✘{!}\n")
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

// getPasswordHash return shadow file compatible hash
func getPasswordHash(password string) (string, error) {
	return sha2crypt.NewCrypter512(5000).Hash(password)
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

// getActiveBuildBullets return colored string with bullets
func getActiveBuildBullets(active, total int) string {
	var result string

	result += strings.Repeat("{g}•{!}", active)
	result += strings.Repeat("{s}•{!}", total-active)

	return result
}

// prefsToArgs return preferences as command line arguments for terraform
func prefsToArgs(prefs *Preferences, args ...string) ([]string, error) {
	auth, err := getPasswordHash(prefs.Password)

	if err != nil {
		return nil, err
	}

	fingerpint, err := getFingerprint(prefs.Key + ".pub")

	if err != nil {
		return nil, err
	}

	var vars []string

	vars = append(vars, fmtc.Sprintf("-var token=%s", prefs.Token))
	vars = append(vars, fmtc.Sprintf("-var auth=%s", auth))
	vars = append(vars, fmtc.Sprintf("-var fingerprint=%s", fingerpint))
	vars = append(vars, fmtc.Sprintf("-var key=%s", prefs.Key))
	vars = append(vars, fmtc.Sprintf("-var user=%s", prefs.User))
	vars = append(vars, fmtc.Sprintf("-var password=%s", prefs.Password))

	if prefs.Region != "" {
		vars = append(vars, fmtc.Sprintf("-var region=%s", prefs.Region))
	}

	if prefs.NodeSize != "" {
		vars = append(vars, fmtc.Sprintf("-var node_size=%s", prefs.NodeSize))
	}

	vars = append(vars, fmtc.Sprintf("-state=%s", getTerraformStateFilePath()))

	if len(args) != 0 {
		vars = append(vars, args...)
	}

	return vars, nil
}

// execTerraform execute terraform command
func execTerraform(logOutput bool, command string, args []string) error {
	cmd := exec.Command("terraform", command)

	if len(args) != 0 {
		cmd.Args = append(cmd.Args, strings.Split(strings.Join(args, " "), " ")...)
	}

	r, err := cmd.StdoutPipe()

	if err != nil {
		return fmtc.Errorf("Can't redirect output: %v", err)
	}

	s := bufio.NewScanner(r)

	go func() {
		for s.Scan() {
			text := s.Text()

			if logOutput {
				// Skip empty line logging
				if text != "" {
					log.Info(text)
				}
			} else {
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

// getNodeList return map with active build nodes
func getNodeList(prefs *Preferences) (map[string]string, error) {
	state, err := readTFState(getTerraformStateFilePath())

	if err != nil {
		return nil, fmtc.Errorf("Can't read state file: %v", err)
	}

	if len(state.Modules) == 0 || len(state.Modules[0].Resources) == 0 {
		return nil, nil
	}

	result := make(map[string]string)

	for _, node := range state.Modules[0].Resources {
		if node.Info == nil || node.Info.Attributes == nil {
			continue
		}

		result[node.Info.Attributes.Name] = node.Info.Attributes.IP
	}

	return result, nil
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

	var (
		nodeNames = make([]string, 0)
		nodeList  = make(map[string]string)
	)

	nodes, err := getNodeList(prefs)

	if err != nil {
		return err
	}

	for nodeName, nodeIP := range nodes {
		nodeRec := fmtc.Sprintf(
			"%s:%s@%s",
			prefs.User,
			prefs.Password,
			nodeIP,
		)

		switch {
		case strings.HasSuffix(nodeName, "-x32"):
			nodeRec += "~i386"

		case strings.HasSuffix(nodeName, "-x48"):
			nodeRec += "~i686"
		}

		nodeNames = append(nodeNames, nodeName)
		nodeList[nodeName] = nodeRec
	}

	sort.Strings(nodeNames)

	for _, nodeName := range nodeNames {
		fmtc.Fprintln(fd, nodeList[nodeName])
	}

	return nil
}

// signalInterceptor is TERM and INT signal handler
func signalInterceptor() {
	printWarn("\nYou can't cancel command execution in this time")
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

// printError prints error message to console
func printError(f string, a ...interface{}) {
	fmtc.Printf("{r}"+f+"{!}\n", a...)
}

// printError prints warning message to console
func printWarn(f string, a ...interface{}) {
	fmtc.Printf("{y}"+f+"{!}\n", a...)
}

// cleanTerraformGarbage remove tf-plugin* files from
// temporary directory
func cleanTerraformGarbage() {
	garbage := fsutil.List(
		"/tmp", false,
		&fsutil.ListingFilter{
			MatchPatterns: []string{"tf-plugin*"},
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

	info.AddOption(ARG_TTL, "Max farm TTL (Time To Live)", "time")
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
	info.AddOption(ARG_NO_COLOR, "Disable colors in output")
	info.AddOption(ARG_HELP, "Show this help message")
	info.AddOption(ARG_VER, "Show version")

	info.AddExample(CMD_CREATE+" --node-size 8gb --ttl 3h", "Create farm with redefined node size and TTL")
	info.AddExample(CMD_CREATE+" --force", "Forced farm creation (without prompt)")
	info.AddExample(CMD_CREATE+" c6-multiarch-fast", "Create farm from template c6-multiarch-fast")
	info.AddExample(CMD_DESTROY, "Destroy all farm nodes")
	info.AddExample(CMD_STATUS, "Show info about terrafarm")

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
