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
	VER  = "0.6.2"
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
	ARG_TEMPLATE    = "L:template"
	ARG_DEBUG       = "D:debug"
	ARG_MONITOR     = "m:monitor"
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
	EV_OUTPUT    = "TERRAFARM_OUTPUT"
	EV_TEMPLATE  = "TERRAFARM_TEMPLATE"
	EV_TOKEN     = "TERRAFARM_TOKEN"
	EV_KEY       = "TERRAFARM_KEY"
	EV_REGION    = "TERRAFARM_REGION"
	EV_NODE_SIZE = "TERRAFARM_NODE_SIZE"
	EV_USER      = "TERRAFARM_USER"
	EV_PASSWORD  = "TERRAFARM_PASSWORD"
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

// MonitorState contains monitor specific info
type MonitorState struct {
	Pid          int   `json:"pid"`
	DestroyAfter int64 `json:"destroy_after"`
}

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
	ARG_TEMPLATE:    &arg.V{},
	ARG_DEBUG:       &arg.V{Type: arg.BOOL},
	ARG_MONITOR:     &arg.V{Type: arg.INT},
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
		cmd := args[0]
		processCommand(cmd)
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

// startMonitor starts monitoring process
func startMonitor() {
	destroyAfter := time.Unix(int64(arg.GetI(ARG_MONITOR)), 0)
	monitorPid := os.Getpid()

	state := &MonitorState{
		Pid:          monitorPid,
		DestroyAfter: int64(arg.GetI(ARG_MONITOR)),
	}

	stateFile := getMonitorStateFilePath()

	err := saveMonitorState(stateFile, state)

	if err != nil {
		exit(1)
	}

	log.Set(getMonitorLogFilePath(), 0644)
	log.Aux(SEPARATOR)
	log.Aux("Terrafarm %s monitor started", VER)
	log.Info("Farm will be destroyed after %s", timeutil.Format(destroyAfter, "%Y/%m/%d %H:%M:%S"))

	for {
		if !isTerrafarmActive() {
			log.Info("Farm destroyed manually")
			os.Remove(stateFile)
			exit(0)
		}

		time.Sleep(time.Minute)

		if time.Now().Unix() <= destroyAfter.Unix() {
			continue
		}

		log.Info("Starting farm destroying...")

		prefs := findAndReadPreferences()
		vars, err := prefsToArgs(prefs, "-no-color", "-force")

		if err != nil {
			continue
		}

		fsutil.Push(path.Join(getDataDir(), prefs.Template))

		err = execTerraform(true, "destroy", vars)

		if err != nil {
			log.Error("Can't destroy farm - terrafarm return error: %v", err)
			continue
		}

		fsutil.Pop()

		os.Remove(getFarmStateFilePath())

		break
	}

	log.Info("Farm successfully destroyed!")

	os.Remove(stateFile)

	exit(0)
}

// processCommand execute some command
func processCommand(cmd string) {
	prefs := findAndReadPreferences()

	scm := getSpellcheckModel()
	cmd = scm.Correct(cmd)

	switch cmd {
	case CMD_CREATE, CMD_APPLY, CMD_START:
		createCommand(prefs)
	case CMD_DESTROY, CMD_DELETE, CMD_STOP:
		destroyCommand(prefs)
	case CMD_STATUS, CMD_INFO, CMD_STATE:
		statusCommand(prefs)
	case CMD_TEMPLATES:
		templatesCommand()
	default:
		fmtc.Printf("{r}Unknown command %s\n", cmd)
		exit(1)
	}

	exit(0)
}

// createCommand is create command handler
func createCommand(prefs *Preferences) {
	if isTerrafarmActive() {
		printWarn("Terrafarm already works")
		exit(1)
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

		err = runMonitor(prefs.TTL)

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
	)

	disableValidate := arg.GetB(ARG_NO_VALIDATE)
	fingerprint, _ := getFingerprint(prefs.Key + ".pub")
	buildersCount := getBuildNodesCount(prefs.Template)

	if isTerrafarmActive() {
		farmState, err := readFarmState(getFarmStateFilePath())

		if err == nil {
			disableValidate = true
			prefs = farmState.Preferences
			fingerprint = farmState.Fingerprint
		}
	}

	if !disableValidate {
		tokenValid = do.IsValidToken(prefs.Token)
		fingerprintValid = do.IsFingerprintValid(prefs.Token, fingerprint)
		regionValid = do.IsRegionValid(prefs.Token, prefs.Region)
		sizeValid = do.IsSizeValid(prefs.Token, prefs.NodeSize)
	}

	fmtutil.Separator(false, "TERRAFARM")

	fmtc.Printf(
		"  {*}%-16s{!} %s {s}(%s){!}\n", "Template:", prefs.Template,
		fmtutil.Pluralize(buildersCount, "build node", "build nodes"),
	)

	fmtc.Printf("  {*}%-16s{!} %s", "Token:", getMaskedToken(prefs.Token))

	printValidationMarker(tokenValid, disableValidate)

	fmtc.Printf("  {*}%-16s{!} %s\n", "Private Key:", prefs.Key)
	fmtc.Printf("  {*}%-16s{!} %s\n", "Public Key:", prefs.Key+".pub")

	fmtc.Printf("  {*}%-16s{!} %s", "Fingerprint:", fingerprint)

	printValidationMarker(fingerprintValid, disableValidate)

	switch {
	case prefs.TTL <= 0:
		fmtc.Printf("  {*}%-16s{!} {r}disabled{!}\n", "TTL:")
	case prefs.TTL > 360:
		fmtc.Printf("  {*}%-16s{!} {r}%s{!}\n", "TTL:", timeutil.PrettyDuration(prefs.TTL*60))
	case prefs.TTL > 120:
		fmtc.Printf("  {*}%-16s{!} {y}%s{!}\n", "TTL:", timeutil.PrettyDuration(prefs.TTL*60))
	default:
		fmtc.Printf("  {*}%-16s{!} {g}%s{!}\n", "TTL:", timeutil.PrettyDuration(prefs.TTL*60))
	}

	fmtc.Printf("  {*}%-16s{!} %s", "Region:", prefs.Region)

	printValidationMarker(regionValid, disableValidate)

	fmtc.Printf("  {*}%-16s{!} %s", "Node size:", prefs.NodeSize)

	printValidationMarker(sizeValid, disableValidate)

	fmtc.Printf("  {*}%-16s{!} %s\n", "User:", prefs.User)

	if prefs.Output != "" {
		fmtc.Printf("  {*}%-16s{!} %s\n", "Output:", prefs.Output)
	}

	if !isTerrafarmActive() {
		fmtc.Printf("  {*}%-16s{!} {s}stopped{!}\n", "State:")
	} else {
		fmtc.Printf("  {*}%-16s{!} {g}works{!}\n", "State:")

		if isMonitorActive() {
			state, err := readMonitorState(getMonitorStateFilePath())

			if err != nil {
				fmtc.Printf("  {*}%-16s{!} {r}unknown{!}\n", "Monitor:")
			} else {
				ttlEst := state.DestroyAfter - time.Now().Unix()

				if ttlEst < 0 {
					fmtc.Printf("  {*}%-16s{!} {g}works{!} {y}(destroying){!}\n", "Monitor:")
				} else {
					fmtc.Printf(
						"  {*}%-16s{!} {g}works{!} {s}(%s to destroy){!}\n",
						"Monitor:", timeutil.PrettyDuration(ttlEst),
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

	os.Remove(getFarmStateFilePath())
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

	err := saveFarmState(getFarmStateFilePath(), farmState)

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

// exec execute command
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
			if logOutput {
				log.Info(s.Text())
			} else {
				fmtc.Printf("  %s\n", getColoredCommandOutput(s.Text()))
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

// isMonitorActive return true is monitor process is active
func isMonitorActive() bool {
	stateFile := getMonitorStateFilePath()

	if !fsutil.IsExist(stateFile) {
		return false
	}

	state, err := readMonitorState(stateFile)

	if err != nil {
		return false
	}

	return fsutil.IsExist(path.Join("/proc", fmtc.Sprintf("%d", state.Pid)))
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

// getMonitorLogFilePath return path to monitor log file
func getMonitorLogFilePath() string {
	return path.Join(getDataDir(), MONITOR_LOG_FILE)
}

// getMonitorStateFilePath return path to monitor state file
func getMonitorStateFilePath() string {
	return path.Join(getDataDir(), MONITOR_STATE_FILE)
}

// saveMonitorState save monitor state to file
func saveMonitorState(file string, state *MonitorState) error {
	return jsonutil.EncodeToFile(file, state)
}

// readMonitorDestroyDate read monitor state from file
func readMonitorState(file string) (*MonitorState, error) {
	state := &MonitorState{}

	err := jsonutil.DecodeFile(file, state)

	if err != nil {
		return nil, err
	}

	return state, nil
}

// saveFarmState save farm state to file
func saveFarmState(file string, state *FarmState) error {
	return jsonutil.EncodeToFile(file, state)
}

// readMonitorDestroyDate read farm state from file
func readFarmState(file string) (*FarmState, error) {
	state := &FarmState{}

	err := jsonutil.DecodeFile(file, state)

	if err != nil {
		return nil, err
	}

	return state, nil
}

// runMonitor run monitoring process
func runMonitor(ttl int64) error {
	destroyTime := time.Now().Unix() + (ttl * 60)

	cmd := exec.Command("terrafarm", "--monitor", fmtc.Sprintf("%d", destroyTime))

	return cmd.Start()
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

	state, err := readTFState(getTerraformStateFilePath())

	if err != nil {
		return fmtc.Errorf("Can't read state file: %v", err)
	}

	nodes := make([]string, 3)

	for _, node := range state.Modules[0].Resources {
		nodeRec := fmtc.Sprintf(
			"%s:%s@%s",
			prefs.User,
			prefs.Password,
			node.Info.Attributes.IP,
		)

		switch {
		case strings.HasSuffix(node.Info.Attributes.Name, "-x32"):
			nodeRec += "~i386"
			nodes[0] = nodeRec

		case strings.HasSuffix(node.Info.Attributes.Name, "-x48"):
			nodeRec += "~i686"
			nodes[1] = nodeRec

		default:
			nodes[2] = nodeRec
		}
	}

	var result []string

	for _, node := range nodes {
		result = append(result, node)
	}

	fmtc.Fprintln(fd, strings.Join(result, "\n"))

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

	info.AddCommand(CMD_CREATE, "Create and run farm droplets on DigitalOcean")
	info.AddCommand(CMD_DESTROY, "Destroy farm droplets on DigitalOcean")
	info.AddCommand(CMD_STATUS, "Show current Terrafarm preferences and status")
	info.AddCommand(CMD_TEMPLATES, "List all available farm templates")

	info.AddOption(ARG_TTL, "Max farm TTL (Time To Live)", "ttl")
	info.AddOption(ARG_OUTPUT, "Path to output file with access credentials", "file")
	info.AddOption(ARG_TEMPLATE, "Farm template name", "name")
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
