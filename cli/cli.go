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

const (
	APP  = "Terrafarm"
	VER  = "0.5.1"
	DESC = "Utility for working with terraform based rpmbuilder farm"
)

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
	ARG_FORCE       = "f:force"
	ARG_NO_VALIDATE = "nv:no-validate"
	ARG_NO_COLOR    = "nc:no-color"
	ARG_HELP        = "h:help"
	ARG_VER         = "v:version"
)

const (
	CMD_CREATE  = "create"
	CMD_APPLY   = "apply"
	CMD_START   = "start"
	CMD_DESTROY = "destroy"
	CMD_DELETE  = "delete"
	CMD_STOP    = "stop"
	CMD_STATUS  = "status"
	CMD_INFO    = "info"
	CMD_STATE   = "state"
)

// TERRAFORM_DATA_DIR is name of dir with terraform data
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

// DATA_ENV_VAR is name of environment variable with path to data directory
const DATA_ENV_VAR = "TERRADATA"

// ////////////////////////////////////////////////////////////////////////////////// //

// MonitorState cotains monitor specific info
type MonitorState struct {
	Pid          int   `json:"pid"`
	DestroyAfter int64 `json:"destroy_after"`
}

// FarmState contains farm specific info
type FarmState struct {
	Prefs       *Prefs `json:"prefs"`
	Fingerprint string `json:"fingerprint"`
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

// ////////////////////////////////////////////////////////////////////////////////// //

func Init() {
	runtime.GOMAXPROCS(2)

	args, errs := arg.Parse(argMap)

	if len(errs) != 0 {
		fmtc.NewLine()

		for _, err := range errs {
			fmtc.Printf("{r}%v{!}\n", err)
		}

		os.Exit(1)
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
		fmtc.Println("{r}GOPATH must be set to valid path{!}")
		os.Exit(1)
	}

	srcDir := getSrcDir()

	if !fsutil.CheckPerms("DRW", srcDir) {
		fmtc.Printf("{r}Source directory %s is not accessible{!}\n", srcDir)
		os.Exit(1)
	}

	dataDir := getDataDir()

	if !fsutil.CheckPerms("DRW", dataDir) {
		fmtc.Printf("{r}Data directory %s is not accessible{!}\n", dataDir)
		os.Exit(1)
	}
}

// checkDeps check required dependecies
func checkDeps() {
	hasErrors := false

	for _, dep := range depList {
		if env.Which(dep) == "" {
			fmtc.Printf("{r}Can't find %s. Please install it first.{!}\n", dep)
			hasErrors = true
		}
	}

	if hasErrors {
		os.Exit(1)
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
		os.Exit(1)
	}

	log.Set(getMonitorLogFilePath(), 0644)
	log.Aux("Terrafarm %s monitor started", VER)
	log.Info("Farm will be destroyed after %s", timeutil.Format(destroyAfter, "%Y/%m/%d %H:%M:%S"))

	for {
		if !isTerrafarmActive() {
			log.Info("Farm destroyed manually")
			os.Exit(0)
		}

		time.Sleep(time.Minute)

		if time.Now().Unix() <= destroyAfter.Unix() {
			continue
		}

		log.Info("Starting farm destroying...")

		prefs := findAndReadPrefs()
		vars, err := prefsToArgs(prefs)

		if err != nil {
			continue
		}

		vars = append(vars, "-force")

		err = execTerraform(true, "destroy", vars)

		if err != nil {
			log.Error("Can't destroy farm - terrafarm return error: %v", err)
			continue
		}

		break
	}

	log.Info("Farm successfully destroyed!")

	os.Remove(stateFile)
}

// processCommand execute some command
func processCommand(cmd string) {
	prefs := findAndReadPrefs()

	scm := getSpellcheckModel()
	cmd = scm.Correct(cmd)

	switch cmd {
	case CMD_CREATE, CMD_APPLY, CMD_START:
		createCommand(prefs)
	case CMD_DESTROY, CMD_DELETE, CMD_STOP:
		destroyCommand(prefs)
	case CMD_STATUS, CMD_INFO, CMD_STATE:
		statusCommand(prefs)
	default:
		fmtc.Printf("{r}Unknown command %s\n", cmd)
		os.Exit(1)
	}
}

// createCommand is create command handler
func createCommand(prefs *Prefs) {
	if isTerrafarmActive() {
		fmtc.Println("{y}Terrafarm already works{!}")
		os.Exit(1)
	}

	statusCommand(prefs)

	if !arg.GetB(ARG_FORCE) {
		if !terminal.ReadAnswer("Create farm with this preferencies? (y/N)", "n") {
			fmtc.NewLine()
			return
		}

		fmtutil.Separator(false)
	}

	vars, err := prefsToArgs(prefs)

	if err != nil {
		fmtc.Printf("{r}Can't parse prefs: %v{!}\n", err)
		os.Exit(1)
	}

	addSignalInterception()

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("{s}EXEC → terraform apply %s{!}\n\n", strings.Join(vars, " "))
	}

	err = execTerraform(false, "apply", vars)

	if err != nil {
		fmtc.Printf("{r}Error while executing terraform: %v\n{!}", err)
		os.Exit(1)
	}

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
			os.Exit(1)
		}

		fmtc.Println("{g}Monitoring process succefully started!{!}")

		fmtutil.Separator(false)
	}

	saveState(prefs)
}

// statusCommand is status command handler
func statusCommand(prefs *Prefs) {
	var (
		tokenValid       bool
		fingerprintValid bool
		regionValid      bool
		sizeValid        bool
	)

	disableValidate := arg.GetB(ARG_NO_VALIDATE)
	fingerprint, _ := getFingerprint(prefs.Key + ".pub")

	if isTerrafarmActive() {
		farmState, err := readFarmState(getFarmStateFilePath())

		if err == nil {
			disableValidate = true
			prefs = farmState.Prefs
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
func destroyCommand(prefs *Prefs) {
	if !isTerrafarmActive() {
		fmtc.Println("{y}Terrafarm does not works, nothing to destroy{!}")
		os.Exit(1)
	}

	if !arg.GetB(ARG_FORCE) {
		fmtc.NewLine()

		if !terminal.ReadAnswer("Destroy farm? (y/N)", "n") {
			return
		}
	}

	fmtutil.Separator(false)

	vars, err := prefsToArgs(prefs)

	if err != nil {
		fmtc.Printf("{r}Can't parse prefs: %v{!}\n", err)
		os.Exit(1)
	}

	addSignalInterception()

	vars = append(vars, "-force")

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("{s}EXEC → terraform destroy %s{!}\n\n", strings.Join(vars, " "))
	}

	err = execTerraform(false, "destroy", vars)

	if err != nil {
		fmtc.Printf("{r}Error while executing terraform: %v\n{!}", err)
		os.Exit(1)
	}

	fmtutil.Separator(false)

	os.Remove(getFarmStateFilePath())
}

// saveFarmState collect and save farm state into file
func saveState(prefs *Prefs) {
	fingerprint, _ := getFingerprint(prefs.Key + ".pub")

	farmState := &FarmState{
		Prefs:       prefs,
		Fingerprint: fingerprint,
	}

	farmState.Prefs.Token = getCryptedToken(prefs.Token)

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

// prefsToArgs return prefs as command line arguments for terraform
func prefsToArgs(prefs *Prefs) ([]string, error) {
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

	return vars, nil
}

// exec execute command
func execTerraform(logOutput bool, command string, args []string) error {
	fsutil.Push(getDataDir())

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

	fsutil.Pop()

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

// getDataDir return path to directory with terraform data
func getDataDir() string {
	if envMap[DATA_ENV_VAR] != "" {
		return envMap[DATA_ENV_VAR]
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
	return path.Join(getSrcDir(), MONITOR_LOG_FILE)
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
func exportNodeList(prefs *Prefs) error {
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

	fmtc.Fprintln(fd, strings.Join(nodes, "\n"))

	return nil
}

// signalInterceptor is TERM and INT signal handler
func signalInterceptor() {
	fmtc.Println("\n{y}You can't cancel command execution in this time{!}")
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

// ////////////////////////////////////////////////////////////////////////////////// //

// showUsage show help content
func showUsage() {
	info := usage.NewInfo("")

	info.AddCommand(CMD_CREATE, "Create and run farm droplets on DigitalOcean")
	info.AddCommand(CMD_DESTROY, "Destroy farm droplets on DigitalOcean")
	info.AddCommand(CMD_STATUS, "Show current Terrafarm preferencies and status")

	info.AddOption(ARG_TTL, "Max farm TTL (Time To Live)", "ttl")
	info.AddOption(ARG_OUTPUT, "Path to output file with access credentials", "file")
	info.AddOption(ARG_TOKEN, "DigitalOcean token", "token")
	info.AddOption(ARG_KEY, "Path to private key", "key-file")
	info.AddOption(ARG_REGION, "DigitalOcean region", "region")
	info.AddOption(ARG_NODE_SIZE, "Droplet size on DigitalOcean", "size")
	info.AddOption(ARG_USER, "Build node user name", "username")
	info.AddOption(ARG_FORCE, "Force command execution")
	info.AddOption(ARG_NO_VALIDATE, "Don't validate preferencies")
	info.AddOption(ARG_NO_COLOR, "Disable colors in output")
	info.AddOption(ARG_HELP, "Show this help message")
	info.AddOption(ARG_VER, "Show version")

	info.AddExample(CMD_CREATE+" --node-size 8gb --ttl 3h", "Create farm with redefined node size and TTL")
	info.AddExample(CMD_CREATE+" --force", "Forced farm creation (without prompt)")
	info.AddExample(CMD_DESTROY, "Destory all farm nodes")
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
